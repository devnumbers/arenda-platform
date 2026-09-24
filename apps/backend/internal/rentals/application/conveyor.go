package application

// The mutation conveyor and the shared gate builders of the rentals use
// cases. Every mutation of the context runs through the same ordering
// invariants structurally (ADR 0053 §3, the payments discipline): the role
// gate, the property row lock, the owner's today, the change step, its audit
// entry and its action journal row (ADR 0061) in the same transaction, and
// the payments tick after the change when the step's verdict says the
// payment changed. Reads never tick and never write.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// The audit action constants of the context's mutations (ADR 0053 §3). There
// is no rental.extended — «продление» is a UI scenario of the planned-end
// edit and audits as rental.updated.
const (
	auditActionRentalCreated   = auditdomain.ActionRentalCreated
	auditActionRentalUpdated   = auditdomain.ActionRentalUpdated
	auditActionRentalCompleted = auditdomain.ActionRentalCompleted
	auditActionRentalDeleted   = auditdomain.ActionRentalDeleted
)

// changeStep is the use-case-specific part of the mutation conveyor. It
// receives the loaded rental (a zero rental at creation), applies its change
// inside the open transaction and produces one typed verdict: the changed
// rental's id (the post-commit re-read target), its explicit audit action and
// whether the managed payment changed — the in-transaction tick runs only
// then.
type changeStep func(
	ctx context.Context, stores *txStores, scope uuid.UUID, rental domain.Rental, today time.Time,
) (mutationOutcome, error)

// mutationOutcome is the change step's single verdict channel: everything the
// caller and the machine need after the transaction — no side channels.
type mutationOutcome struct {
	// RentalID is the post-commit re-read target: the response always carries
	// the persisted view. Deletion states it too; its caller ignores it.
	RentalID uuid.UUID
	// Audit records this mutation inside the same transaction (fail-safe,
	// ADR 0020). Every step states its action explicitly — there is no
	// derivation from the response.
	Audit    auditdomain.Action
	AuditCtx map[string]any
	// History is the action journal row of the mutation (ADR 0061), built by
	// the step from the row-text catalog and recorded inside the same
	// transaction (fail-safe like the audit).
	History *historydomain.Entry
	// Tick runs the payments materialization tick for the owner after the
	// change; only a step that changed the managed payment sets it (ADR 0053
	// §3). Completion and deletion resolve the plan themselves and set false.
	Tick bool
}

// mutationGates bundles the dependencies the conveyor needs beyond the
// transactional stores: the owner calendar for the day boundary; the role
// gate arrives per call, already resolved over the caller's policy.
type mutationGates struct {
	factory  txStoreFactory
	calendar paymentsapp.OwnerCalendar
}

// runRentalMutation is the mutation conveyor shared by every use case of this
// context. It runs, in one transaction and in this order: the role gate, the
// property serialization lock (FOR UPDATE — ADR 0053 §3), the owner's today,
// the load of the target rental (skipped for a zero rentalID), the change
// step, its audit entry and its action journal row (ADR 0061) in the same
// transaction, and the payments tick when the step changed the payment.
// After commit it returns the changed rental's id for the re-read.
func runRentalMutation(
	g mutationGates,
	ctx context.Context, actor, propertyID, rentalID uuid.UUID,
	gate gateFunc,
	change changeStep,
) (uuid.UUID, error) {
	role, err := gate(ctx, actor, propertyID)
	if err != nil {
		return uuid.Nil, err
	}
	var scope uuid.UUID
	var out mutationOutcome
	err = g.factory.runInTx(ctx, func(stores *txStores) error {
		prop, today, err := lockActiveProperty(ctx, stores, g.calendar, propertyID)
		if err != nil {
			return err
		}
		scope = prop.OwnerID
		rental := domain.Rental{}
		if rentalID != uuid.Nil {
			rental, err = stores.rentals.Get(ctx, rentalID, scope, propertyID)
			if err != nil {
				return err
			}
		}
		out, err = change(ctx, stores, scope, rental, today)
		if err != nil {
			return err
		}
		if err := recordAudit(ctx, stores, actor, role, out.Audit, out.RentalID, out.AuditCtx); err != nil {
			return err
		}
		if out.History != nil {
			if err := historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
				sharedpolicy.HistoryActorRole(role), *out.History); err != nil {
				return err
			}
		}
		if !out.Tick {
			return nil
		}
		return stores.pay.RunTick(ctx, scope, today)
	})
	if err != nil {
		return uuid.Nil, err
	}
	return out.RentalID, nil
}

// lockActiveProperty loads the property with its row locked — the mutation's
// serialization point shared with the payments context (ADR 0053 §3) —
// resolves the owner's today and rejects the financial read-only archived
// state.
func lockActiveProperty(
	ctx context.Context, stores *txStores, calendar paymentsapp.OwnerCalendar, propertyID uuid.UUID,
) (paymentsapp.PropertyRef, time.Time, error) {
	prop, err := stores.properties.GetForUpdate(ctx, propertyID)
	if err != nil {
		return paymentsapp.PropertyRef{}, time.Time{}, err
	}
	if prop.Archived {
		return paymentsapp.PropertyRef{}, time.Time{}, ErrArchivedProperty
	}
	today, err := ownerToday(calendar, ctx, prop.OwnerID)
	if err != nil {
		return paymentsapp.PropertyRef{}, time.Time{}, err
	}
	return prop, today, nil
}

// ownerToday resolves the data owner's calendar date (ADR 0048), failing
// loudly when the calendar was never wired — a construction mistake, not a
// runtime condition.
func ownerToday(
	calendar paymentsapp.OwnerCalendar, ctx context.Context, ownerID uuid.UUID,
) (time.Time, error) {
	if calendar == nil {
		return time.Time{}, errors.New("rentals: owner calendar must be configured")
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
// whitelisted keys only — never the comment or the amounts.
func recordAudit(
	ctx context.Context, stores *txStores, actor uuid.UUID, role sharedpolicy.Role,
	action auditdomain.Action, rentalID uuid.UUID, auditCtx map[string]any,
) error {
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor,
		ActorRole:  sharedpolicy.AuditActorRole(role),
		Action:     action,
		EntityType: auditdomain.EntityRental,
		EntityID:   &rentalID,
		Context:    auditCtx,
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}

// gateFunc is what the gates produce for the audit trail: the resolved role
// of an actor on a property.
type gateFunc = func(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error)

// newCapabilityGate returns the ADR 0028 gate closure over the policy and one
// capability predicate. The decision mapping is the rentals error vocabulary
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
// SQL reads filter by (ADR 0028). A role without the view capability maps to
// ErrNotFound so the existence of any rental data is never revealed.
func resolveReadScope(
	ctx context.Context, policy sharedpolicy.Policy, properties paymentsapp.PropertyStore,
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
