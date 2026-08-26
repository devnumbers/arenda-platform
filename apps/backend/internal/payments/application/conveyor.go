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
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// changeStep is the use-case-specific part of the mutation conveyor: it
// receives the loaded rule (a zero rule when there is no rule to load: at
// creation the new draft travels back, and in the operation mutations the
// change proves existence from its own store), applies its change inside the
// open transaction and tells the conveyor the audit entry, the tick and the
// re-read verdicts.
type changeStep func(
	ctx context.Context, stores *txStores, scope uuid.UUID, rule domain.Payment, today time.Time,
) (domain.Payment, mutationOutcome, error)

// mutationOutcome is the change step's verdict for the conveyor: the audit
// entry to record in the same transaction (the entity defaults to the rule;
// an operation-level change overrides it), whether the materialization tick
// runs after the change, and whether the stored rule is re-read after commit
// for the response.
type mutationOutcome struct {
	audit    auditdomain.Action
	auditCtx map[string]any
	tick     bool
	reread   bool
	// The auditEntity and auditEntityID fields retarget the entry away from
	// the rule: operation.paid targets the operation row, not its originating
	// rule.
	auditEntity   auditdomain.EntityType
	auditEntityID *uuid.UUID
}

// mutationGates bundles the dependencies the conveyor needs beyond the
// transactional stores: the owner calendar for the day boundary; the role
// gate arrives per call, already resolved over the caller's policy.
type mutationGates struct {
	factory  txStoreFactory
	calendar OwnerCalendar
}

// runMutation is the mutation conveyor shared by every use case of this
// context — rules and operations alike.
// It runs, in one transaction and in this order: the role gate, the property
// serialization lock (FOR UPDATE — ADR 0049 §3), the owner's today, the load
// of the target rule (skipped for a zero paymentID — creation mints its own),
// the change step, its audit entry, and the materialization tick after the
// change. The change step is the only use-case-specific part; after commit
// the conveyor re-reads the stored rule so the response carries the persisted
// timestamps, pauses and category view.
func (g mutationGates) runMutation(
	ctx context.Context, actor, propertyID, paymentID uuid.UUID,
	gate func(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error),
	change changeStep,
) (domain.Payment, error) {
	role, err := gate(ctx, actor, propertyID)
	if err != nil {
		return domain.Payment{}, err
	}
	var scope, ruleID uuid.UUID
	var outcome mutationOutcome
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
		rule, outcome, err = change(ctx, stores, scope, rule, today)
		if err != nil {
			return err
		}
		ruleID = rule.ID
		entityType := outcome.auditEntity
		if entityType == "" {
			entityType = auditdomain.EntityPayment
		}
		entityID := outcome.auditEntityID
		if entityID == nil {
			entityID = &rule.ID
		}
		if err := recordAudit(ctx, stores, actor, role, outcome.audit, entityType, entityID, outcome.auditCtx); err != nil {
			return err
		}
		if outcome.tick {
			return stores.tickOwner(ctx, scope, today)
		}
		return nil
	})
	if err != nil {
		return domain.Payment{}, err
	}
	if !outcome.reread {
		return domain.Payment{}, nil
	}
	return g.factory.payments.Get(ctx, ruleID, scope, propertyID)
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
	if calendar == nil {
		return PropertyRef{}, time.Time{}, errors.New("payments: owner calendar must be configured")
	}
	today, err := calendar.Today(ctx, prop.OwnerID)
	if err != nil {
		return PropertyRef{}, time.Time{}, fmt.Errorf("resolve owner today: %w", err)
	}
	return prop, today, nil
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

// newCapabilityGate returns the ADR 0028 gate closure over the policy and one
// capability predicate. The decision mapping is the payments error vocabulary
// over the shared gate skeleton (policy.GateFor): none/suspended stay
// privacy-preserving (ErrNotFound — the existence of the data is never
// revealed), a role without the capability is a straight ErrForbidden. A nil
// policy keeps the historical owner-only behaviour.
func newCapabilityGate(
	policy sharedpolicy.Policy, can func(sharedpolicy.Role) bool,
) func(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
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
