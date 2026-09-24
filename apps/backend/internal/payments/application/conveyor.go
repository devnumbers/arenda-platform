package application

// The mutation conveyor and the shared gate builders of the payments use
// cases. Every mutation of the context — rules and operations alike — runs
// through the same ordering invariants structurally (ADR 0049 §3): the role
// gate, the property row lock, the owner's today, the change step, its audit
// entry in the same transaction and the materialization tick after the
// change. Reads never tick and never write.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// changeStep is the use-case-specific part of the mutation conveyor. It
// receives the loaded rule (a zero rule when there is no rule to load: at
// creation the new draft travels out with the outcome, and in the operation
// mutations the change proves existence from its own store), applies its
// change inside the open transaction and produces one typed outcome: the
// caller-facing response payload plus its explicit audit target, the tick
// verdict and the optional post-commit re-read.
type changeStep[T any] func(
	ctx context.Context, stores *txStores, scope uuid.UUID, rule domain.Payment, today time.Time,
) (mutationOutcome[T], error)

// mutationOutcome is the change step's single verdict channel: everything the
// caller and the machine need after the transaction — no side channels.
type mutationOutcome[T any] struct {
	// Response travels to the use case's caller verbatim unless RereadPaymentID
	// asks for a post-commit re-read; then the freshly read rule replaces it.
	Response T
	// Audit records this mutation inside the same transaction (fail-safe,
	// ADR 0020). Every step states its audit target explicitly: there is no
	// derivation from Response, so reshaping an outcome can never silently
	// NULL the audit trail. Empty entity type defaults to the payment.
	Audit         auditdomain.Action
	AuditCtx      map[string]any
	AuditEntity   auditdomain.EntityType
	AuditEntityID *uuid.UUID
	// History is the action journal row of the mutation (ADR 0061), built by
	// the step from the row-text catalog and recorded inside the same
	// transaction (fail-safe like the audit). A nil entry writes no row — the
	// noise actions (the favorite star) are excluded by the map charter.
	History *historydomain.Entry
	// Tick runs the materialization tick for the owner after the change;
	// every use case states its need explicitly (deletion sets false).
	Tick bool
	// RereadPaymentID asks the conveyor to re-read this stored payment after
	// commit so the response carries persisted timestamps, pauses and category
	// view. Only payment-shaped outcomes set it — the name carries that fact.
	RereadPaymentID *uuid.UUID
}

// mutationGates bundles the dependencies the conveyor needs beyond the
// transactional stores: the owner calendar for the day boundary; the role
// gate arrives per call, already resolved over the caller's policy.
type mutationGates struct {
	factory  txStoreFactory
	calendar OwnerCalendar
}

// runMutation is the mutation conveyor shared by every use case of this
// context — rules and operations alike. It runs, in one transaction and in
// this order: the role gate, the property serialization lock (FOR UPDATE —
// ADR 0049 §3), the owner's today, the load of the target rule (skipped for a
// zero paymentID), the change step, its audit entry, and the materialization
// tick when the step asked for it. After commit it returns the step's
// response, post-commit-re-read applied.
func runMutation[T any](
	g mutationGates,
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
	gate gateFunc,
	change changeStep[T],
) (T, error) {
	var zero T
	role, err := gate(ctx, actor, propertyID)
	if err != nil {
		return zero, err
	}
	var scope uuid.UUID
	var out mutationOutcome[T]
	err = g.factory.runInTx(ctx, func(stores *txStores) error {
		prop, today, err := lockActiveProperty(ctx, stores, g.calendar, propertyID)
		if err != nil {
			return err
		}
		scope = prop.OwnerID
		rule := domain.Payment{}
		if paymentID != uuid.Nil {
			rule, err = stores.payments.Get(ctx, paymentID, scope, propertyID)
			if err != nil {
				return err
			}
		}
		out, err = change(ctx, stores, scope, rule, today)
		if err != nil {
			return err
		}
		entityType := out.AuditEntity
		if entityType == "" {
			entityType = auditdomain.EntityPayment
		}
		entityID := out.AuditEntityID
		if err := recordAudit(ctx, stores, actor, role, out.Audit, entityType, entityID, out.AuditCtx); err != nil {
			return err
		}
		if err := recordHistory(ctx, stores, actor, role, propertyID, out.History); err != nil {
			return err
		}
		if !out.Tick {
			return nil
		}
		return stores.tickOwner(ctx, scope, today)
	})
	if err != nil {
		return zero, err
	}
	if out.RereadPaymentID == nil {
		return out.Response, nil
	}
	return rereadPayment[T](g, ctx, scope, propertyID, *out.RereadPaymentID)
}

// rereadPayment re-reads the stored rule after commit so the response carries
// persisted timestamps, pauses and category view. A re-read always yields a
// payment; only payment-shaped outcomes ask for one, so the assertion holds
// by construction.
func rereadPayment[T any](
	g mutationGates,
	ctx context.Context, scope, propertyID, ruleID uuid.UUID,
) (T, error) {
	var zero T
	stored, err := g.factory.payments.Get(ctx, ruleID, scope, propertyID)
	if err != nil {
		return zero, err
	}
	resp, ok := any(stored).(T)
	if !ok {
		return zero, errors.New("payments conveyor: reread requires a payment-shaped response")
	}
	return resp, nil
}

// lockActiveProperty loads the property with its row locked — the mutation's
// serialization point (ADR 0049 §3) — resolves the owner's today and rejects
// the financial read-only archived state.
func lockActiveProperty(
	ctx context.Context, stores *txStores, calendar OwnerCalendar, propertyID uuid.UUID,
) (PropertyRef, time.Time, error) {
	prop, err := stores.properties.GetForUpdate(ctx, propertyID)
	if err != nil {
		return PropertyRef{}, time.Time{}, err
	}
	if prop.Archived {
		return PropertyRef{}, time.Time{}, ErrArchivedProperty
	}
	today, err := ownerToday(calendar, ctx, prop.OwnerID)
	if err != nil {
		return PropertyRef{}, time.Time{}, err
	}
	return prop, today, nil
}

// ensureNotRentalManaged rejects the rule mutations of the rent payment
// (ADR 0053, ticket #818): the payment a rental references is created,
// edited and deleted only through the rental, so pause/resume/update/delete
// from the payments side are ErrRentManagedPayment — the honest 409 the
// rentals RESTRICT FK always implied. The check runs inside the conveyor's
// transaction under the property lock — the same serialization point the
// rentals mutations take — and the rental pipeline's own gateway writes run
// past it by construction (they use the stores, not this conveyor). The
// payment facts (pay) and the favorite star never call it: they are not the
// rental's terms.
func ensureNotRentalManaged(ctx context.Context, stores *txStores, scope, paymentID uuid.UUID) error {
	managed, err := stores.rentalManaged.ManagedPaymentIDs(ctx, scope, []uuid.UUID{paymentID})
	if err != nil {
		return fmt.Errorf("rental-managed gate: %w", err)
	}
	if managed[paymentID] {
		return ErrRentManagedPayment
	}
	return nil
}

// ownerToday resolves the data owner's calendar date (ADR 0048), failing
// loudly when the calendar was never wired — a construction mistake, not a
// runtime condition.
func ownerToday(calendar OwnerCalendar, ctx context.Context, ownerID uuid.UUID) (time.Time, error) {
	if calendar == nil {
		return time.Time{}, errors.New("payments: owner calendar must be configured")
	}
	today, err := calendar.Today(ctx, ownerID)
	if err != nil {
		return time.Time{}, fmt.Errorf("resolve owner today: %w", err)
	}
	return today, nil
}

// recordAudit writes the mutation's audit entry inside the transaction
// (fail-safe: an insert error rolls the mutation back, ADR 0020). The role is
// the one the gate resolved before the transaction opened. Context carries
// whitelisted keys only — never the rule's title or amount.
func recordAudit(
	ctx context.Context, stores *txStores, actor uuid.UUID, role sharedpolicy.Role,
	action auditdomain.Action, entityType auditdomain.EntityType,
	entityID *uuid.UUID, auditCtx map[string]any,
) error {
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  sharedpolicy.AuditActorRole(role),
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Context:    auditCtx,
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}

// recordHistory writes the mutation's action journal entry inside the
// transaction (fail-safe like the audit: an insert error rolls the mutation
// back, ADR 0061 §3) — the row and the action share the transaction's fate.
// The role is the one the gate resolved before the transaction opened. A nil
// entry (the excluded noise actions) writes nothing.
func recordHistory(
	ctx context.Context, stores *txStores, actor uuid.UUID, role sharedpolicy.Role,
	propertyID uuid.UUID, entry *historydomain.Entry,
) error {
	if entry == nil {
		return nil
	}
	entry.PropertyID = propertyID
	entry.ActorID = &actor
	entry.ActorRole = sharedpolicy.HistoryActorRole(role)
	if err := stores.history.Record(ctx, *entry); err != nil {
		return fmt.Errorf("record history: %w", err)
	}
	return nil
}

// gateFunc is what the gates produce for the audit trail: the resolved role
// of an actor on a property.
type gateFunc = func(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error)

// newCapabilityGate returns the ADR 0028 gate closure over the policy and one
// capability predicate. The decision mapping is the payments error vocabulary
// over the shared gate skeleton (policy.GateFor): none/suspended stay
// privacy-preserving (ErrNotFound — the existence of the data is never
// revealed), a role without the capability is a straight ErrForbidden. A nil
// policy keeps the historical owner-only behaviour.
func newCapabilityGate(policy sharedpolicy.Policy, can func(sharedpolicy.Role) bool) gateFunc {
	return func(ctx context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
		if policy == nil {
			return sharedpolicy.RoleOwner, nil
		}
		role, err := policy.RoleForProperty(ctx, actor, propertyID)
		if err != nil {
			return "", fmt.Errorf("resolve role: %w", err)
		}
		switch sharedpolicy.GateFor(role, can) {
		case sharedpolicy.GateAllow:
			return role, nil
		case sharedpolicy.GateForbidden:
			return "", ErrForbidden
		default: // GateNone, GateSuspended.
			return "", ErrNotFound
		}
	}
}

// resolveReadScope applies the read gate and returns the data owner whose
// scope the SQL reads filter by (ADR 0028). A role without the view
// capability maps to ErrNotFound so the existence of any payment data is
// never revealed.
func resolveReadScope(
	ctx context.Context, policy sharedpolicy.Policy, properties PropertyStore,
	actor, propertyID uuid.UUID,
) (uuid.UUID, error) {
	if policy == nil {
		prop, err := properties.Get(ctx, propertyID)
		if err != nil {
			return uuid.Nil, err
		}
		if prop.OwnerID != actor {
			return uuid.Nil, ErrNotFound
		}
		return actor, nil
	}
	role, err := policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve role: %w", err)
	}
	// Every non-allow outcome hides the data's existence: none and suspended
	// are the privacy 404, forbidden (unreachable for view today) would mean
	// the same for reads.
	if sharedpolicy.GateFor(role, sharedpolicy.CanView) != sharedpolicy.GateAllow {
		return uuid.Nil, ErrNotFound
	}
	prop, err := properties.Get(ctx, propertyID)
	if err != nil {
		return uuid.Nil, err
	}
	return prop.OwnerID, nil
}
