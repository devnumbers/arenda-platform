package application

// The mutation conveyor and the shared gate builders of the tasks use
// cases. Every mutation of the context — rules and tasks alike — runs
// through the same ordering invariants structurally (ADR 0051, mirroring
// ADR 0049 §3): the role gate, the owner's ordered property-row set lock,
// the owner's today, the change step, its audit entry and its action
// journal row (ADR 0061) in the same transaction, and the materialization
// tick after the change. Reads never tick and never write.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

// changeStep is the use-case-specific part of the mutation conveyor. It
// receives the loaded rule (a zero rule when there is no rule to load: at
// creation the new draft travels out with the outcome, and in the task
// mutations the change proves existence from its own store), applies its
// change inside the open transaction and produces one typed outcome: the
// caller-facing response payload plus its explicit audit target, the tick
// verdict and the optional post-commit re-read.
type changeStep[T any] func(
	ctx context.Context, stores *txStores, scope uuid.UUID, rule domain.TaskRule, today time.Time,
) (mutationOutcome[T], error)

// mutationOutcome is the change step's single verdict channel: everything the
// caller and the machine need after the transaction — no side channels.
type mutationOutcome[T any] struct {
	// Response travels to the use case's caller verbatim unless RereadRuleID
	// asks for a post-commit re-read; then the freshly read rule replaces it.
	Response T
	// Audit records this mutation inside the same transaction (fail-safe,
	// ADR 0020). Every step states its audit target explicitly: there is no
	// derivation from Response, so reshaping an outcome can never silently
	// NULL the audit trail. Empty entity type defaults to the rule.
	Audit         auditdomain.Action
	AuditCtx      map[string]any
	AuditEntity   auditdomain.EntityType
	AuditEntityID *uuid.UUID
	// History is the action journal row of the mutation (ADR 0061), built by
	// the step from the row-text catalog and recorded inside the same
	// transaction (fail-safe like the audit). Only the property-bound
	// conveyor records it — the property-less book's (ADR 0052) steps leave
	// it nil (no object the row could hang on) and journal through
	// HistoryByProperty instead.
	History *historydomain.Entry
	// HistoryByProperty holds the per-property journal rows of a bulk
	// mutation (ADR 0061 §3: one row per affected object, its removed count
	// in the row text). Only the property-less conveyor records them — each
	// row carries its own property anchor, so a bulk step cannot travel
	// through the single-anchored History.
	HistoryByProperty map[uuid.UUID]historydomain.Entry
	// Tick runs the materialization tick for the owner after the change;
	// every use case states its need explicitly (deletion and the completed
	// journal clear set false).
	Tick bool
	// RereadRuleID asks the conveyor to re-read this stored rule after
	// commit so the response carries persisted timestamps. Only rule-shaped
	// outcomes set it — the name carries that fact.
	RereadRuleID *uuid.UUID
	// ScheduleOverdueOf asks the conveyor to hand this rule's standing
	// uncompleted tasks to the notifications scheduling seam after the tick
	// and the commit (issue #775): their ids are captured inside the
	// transaction once the tick has settled the rule's rows — the freshly
	// materialized and the kept standing tasks alike. Only the rule
	// create/edit flows set it, always with Tick=true.
	ScheduleOverdueOf *uuid.UUID
}

// mutationGates bundles the dependencies the conveyor needs beyond the
// transactional stores: the owner calendar for the day boundary, the
// notifications scheduling seam the rule flows hand their standing tasks to
// after the commit (issue #775; nil keeps the pre-#775 silence) and the
// logger the best-effort handover reports through — the rule service's
// constructor pairs it with the seam, so a set seam always has a logger.
// The role gate arrives per call, already resolved over the caller's policy.
type mutationGates struct {
	factory     txStoreFactory
	calendar    OwnerCalendar
	overdueSeam MaterializedTaskNotifier
	log         *slog.Logger
}

// runMutation is the mutation conveyor shared by every use case of this
// context — rules and tasks alike. It runs, in one transaction and in this
// order: the role gate, the owner-wide ordered property-row lock (the tick's
// serialization point taken at the front — the deadlock-free global order,
// #546), the owner's today, the load of the target rule (skipped for a zero
// ruleID), the change step, its audit entry and its action journal row
// (ADR 0061) in the same transaction, and the materialization tick when the
// step asked for it. After commit it returns the step's response,
// post-commit-re-read applied.
//
// The owner-wide lock must be the transaction's first property lock: two
// mutations that each held only their own property row and then scanned the
// owner's set deadlocked (SQLSTATE 40P01, #546) — the set scan is ordered by
// id, but a pre-held arbitrary member stands outside that order. The owner
// is resolved by an unlocked read (owner_id never moves), the set lock
// covers every active/maintenance row including this one, and the archived
// check below still reads the row under its lock.
func runMutation[T any](
	g mutationGates,
	ctx context.Context, actor, propertyID, ruleID uuid.UUID,
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
	var seamTaskIDs []uuid.UUID
	err = g.factory.runInTx(ctx, func(stores *txStores) error {
		ref, err := stores.properties.Get(ctx, propertyID)
		if err != nil {
			return err
		}
		if err := stores.tick.LockOwnerProperties(ctx, ref.OwnerID); err != nil {
			return err
		}
		prop, today, err := lockActiveProperty(ctx, stores, g.calendar, propertyID)
		if err != nil {
			return err
		}
		scope = prop.OwnerID
		rule := domain.TaskRule{}
		if ruleID != uuid.Nil {
			rule, err = stores.rules.Get(ctx, ruleID, scope, propertyID)
			if err != nil {
				return err
			}
		}
		out, err = change(ctx, stores, scope, rule, today)
		if err != nil {
			return err
		}
		if err := recordTrail(ctx, stores, actor, role, propertyID,
			auditdomain.EntityTaskRule, out); err != nil {
			return err
		}
		if !out.Tick {
			return nil
		}
		if err := stores.tickOwnerProperties(ctx, scope, today); err != nil {
			return err
		}
		seamTaskIDs, err = captureSeamTasks(ctx, stores, out.ScheduleOverdueOf)
		return err
	})
	if err != nil {
		return zero, err
	}
	dispatchSeamTasks(g, ctx, seamTaskIDs)
	if out.RereadRuleID == nil {
		return out.Response, nil
	}
	return rereadRule[T](g, ctx, scope, propertyID, *out.RereadRuleID)
}

// rereadRule re-reads the stored rule after commit so the response carries
// persisted timestamps. A re-read always yields a rule; only rule-shaped
// outcomes ask for one, so the assertion holds by construction.
func rereadRule[T any](
	g mutationGates,
	ctx context.Context, scope, propertyID, ruleID uuid.UUID,
) (T, error) {
	var zero T
	stored, err := g.factory.rules.Get(ctx, ruleID, scope, propertyID)
	if err != nil {
		return zero, err
	}
	resp, ok := any(stored).(T)
	if !ok {
		return zero, errors.New("tasks conveyor: reread requires a rule-shaped response")
	}
	return resp, nil
}

// runOwnerMutation is the property-less twin of runMutation (ADR 0052): the
// same ordering invariants over the owner's own book, where there is no
// property to resolve a role on — the actor is the data owner (scope =
// actor, the ADR 0028 matrix defines no members outside a property) or the
// privacy 404 hides the data. It runs, in one transaction and in this order:
// the owner-row serialization lock (FOR UPDATE users — the anchor ADR 0052
// chose for the property-less slice), the owner's today, the load of the
// target property-less rule (skipped for a zero ruleID), the change step,
// its audit entry and its per-property action journal rows (ADR 0061 §3) in
// the same transaction, and the property-less materialization tick. After
// commit it returns the step's response, post-commit-re-read applied.
func runOwnerMutation[T any](
	g mutationGates,
	ctx context.Context, actor, ruleID uuid.UUID,
	change changeStep[T],
) (T, error) {
	var zero T
	var out mutationOutcome[T]
	var seamTaskIDs []uuid.UUID
	err := g.factory.runInTx(ctx, func(stores *txStores) error {
		if err := stores.tick.LockOwner(ctx, actor); err != nil {
			return err
		}
		today, err := ownerToday(g.calendar, ctx, actor)
		if err != nil {
			return err
		}
		rule := domain.TaskRule{}
		if ruleID != uuid.Nil {
			rule, err = stores.rules.GetWithoutProperty(ctx, ruleID, actor)
			if err != nil {
				return err
			}
		}
		out, err = change(ctx, stores, actor, rule, today)
		if err != nil {
			return err
		}
		if err := recordTrail(ctx, stores, actor, sharedpolicy.RoleOwner, uuid.Nil,
			auditdomain.EntityTaskRule, out); err != nil {
			return err
		}
		// The bulk step's journal: one row per touched object, each with its
		// own anchor (ADR 0061 §3) — recorded with the owner role the
		// property-less conveyor runs under.
		for propertyID, entry := range out.HistoryByProperty {
			if err := historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
				sharedpolicy.HistoryActorRole(sharedpolicy.RoleOwner), entry); err != nil {
				return err
			}
		}
		if !out.Tick {
			return nil
		}
		if err := stores.tickOwnerWithoutProperty(ctx, actor, today); err != nil {
			return err
		}
		seamTaskIDs, err = captureSeamTasks(ctx, stores, out.ScheduleOverdueOf)
		return err
	})
	if err != nil {
		return zero, err
	}
	dispatchSeamTasks(g, ctx, seamTaskIDs)
	if out.RereadRuleID == nil {
		return out.Response, nil
	}
	return rereadOwnerRule[T](g, ctx, actor, *out.RereadRuleID)
}

// rereadOwnerRule is rereadRule's property-less counterpart: the re-read
// goes through the property-less cut key, so a response can never describe a
// bound rule.
func rereadOwnerRule[T any](
	g mutationGates,
	ctx context.Context, owner, ruleID uuid.UUID,
) (T, error) {
	var zero T
	stored, err := g.factory.rules.GetWithoutProperty(ctx, ruleID, owner)
	if err != nil {
		return zero, err
	}
	resp, ok := any(stored).(T)
	if !ok {
		return zero, errors.New("tasks conveyor: reread requires a rule-shaped response")
	}
	return resp, nil
}

// lockActiveProperty loads the property with its row locked — for an
// active/maintenance row the conveyor's owner-wide lock already holds it, so
// this re-lock is a no-op; an archived row stands outside that set, and the
// lock here closes the archive-vs-mutation race before the read-only check.
// It then resolves the owner's today and rejects the archived state.
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

// ownerToday resolves the data owner's calendar date (ADR 0048), failing
// loudly when the calendar was never wired — a construction mistake, not a
// runtime condition.
func ownerToday(calendar OwnerCalendar, ctx context.Context, ownerID uuid.UUID) (time.Time, error) {
	if calendar == nil {
		return time.Time{}, errors.New("tasks: owner calendar must be configured")
	}
	today, err := calendar.Today(ctx, ownerID)
	if err != nil {
		return time.Time{}, fmt.Errorf("resolve owner today: %w", err)
	}
	return today, nil
}

// captureSeamTasks reads the scheduling seam's handover after the tick has
// settled the rule's rows (issue #775): the rule's standing uncompleted
// tasks — the freshly materialized ones and the kept standing ones alike,
// since notifications re-resolves each task's liveness and term itself. A
// change without the seam verdict captures nothing.
func captureSeamTasks(ctx context.Context, stores *txStores, ruleID *uuid.UUID) ([]uuid.UUID, error) {
	if ruleID == nil {
		return nil, nil
	}
	ids, err := stores.tasks.ListUncompletedTaskIDs(ctx, *ruleID)
	if err != nil {
		return nil, fmt.Errorf("capture standing tasks of rule %s: %w", *ruleID, err)
	}
	return ids, nil
}

// dispatchSeamTasks hands the captured task ids to the notifications
// scheduling seam strictly after the commit — the grace-events canon
// (issue #775): a rolled-back transaction dispatches nothing, a nil seam
// keeps the pre-#775 silence, and a broken seam is logged and never fails
// the committed mutation.
func dispatchSeamTasks(g mutationGates, ctx context.Context, taskIDs []uuid.UUID) {
	if len(taskIDs) == 0 || g.overdueSeam == nil {
		return
	}
	if err := g.overdueSeam.NotifyMaterializedTasks(ctx, taskIDs); err != nil {
		g.log.ErrorContext(ctx, "notify materialized tasks failed",
			slog.Int("task_count", len(taskIDs)),
			slog.String("error", sanitize.Error(err)))
	}
}

// recordAudit writes the mutation's audit entry inside the transaction
// (fail-safe: an insert error rolls the mutation back, ADR 0020). The role is
// the one the gate resolved before the transaction opened. Context carries
// whitelisted keys only — never the rule's title.
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

// recordTrail writes the mutation's audit entry and its action journal row
// inside the transaction — the shared tail of both conveyors. The trail
// fields travel in the step's outcome. The empty entity type defaults to
// the rule (the audit contract); the journal is skipped by a nil entry —
// the property-less conveyor's outcomes never carry one (the book has no
// object the row could hang on, its bulk rows travel through
// HistoryByProperty instead), so the guard stays the single point of truth
// for the skip.
func recordTrail[T any](
	ctx context.Context, stores *txStores, actor uuid.UUID, role sharedpolicy.Role,
	propertyID uuid.UUID, defaultEntity auditdomain.EntityType, out mutationOutcome[T],
) error {
	entity := out.AuditEntity
	if entity == "" {
		entity = defaultEntity
	}
	if err := recordAudit(ctx, stores, actor, role, out.Audit, entity, out.AuditEntityID, out.AuditCtx); err != nil {
		return err
	}
	if out.History != nil {
		return historyapp.RecordScoped(ctx, stores.history, propertyID, actor,
			sharedpolicy.HistoryActorRole(role), *out.History)
	}
	return nil
}

// gateFunc is what the gates produce for the audit trail: the resolved role
// of an actor on a property.
type gateFunc = func(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error)

// newCapabilityGate returns the ADR 0028 gate closure over the policy and one
// capability predicate. The decision mapping is the tasks error vocabulary
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
// capability maps to ErrNotFound so the existence of any task data is never
// revealed.
func resolveReadScope(
	ctx context.Context, policy sharedpolicy.Policy, properties PropertyStore,
	actor, propertyID uuid.UUID,
) (uuid.UUID, error) {
	scope, err := resolveReadScopeRef(ctx, policy, properties, actor, propertyID)
	if err != nil {
		return uuid.Nil, err
	}
	return scope.OwnerID, nil
}

// resolveReadScopeRef is resolveReadScope returning the full property
// reference — the data owner plus the display name the global listing
// projects into its rows (ticket #521).
func resolveReadScopeRef(
	ctx context.Context, policy sharedpolicy.Policy, properties PropertyStore,
	actor, propertyID uuid.UUID,
) (PropertyRef, error) {
	if policy == nil {
		prop, err := properties.Get(ctx, propertyID)
		if err != nil {
			return PropertyRef{}, err
		}
		if prop.OwnerID != actor {
			return PropertyRef{}, ErrNotFound
		}
		return prop, nil
	}
	role, err := policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return PropertyRef{}, fmt.Errorf("resolve role: %w", err)
	}
	// Every non-allow outcome hides the data's existence: none and suspended
	// are the privacy 404, forbidden (unreachable for view today) would mean
	// the same for reads.
	if sharedpolicy.GateFor(role, sharedpolicy.CanView) != sharedpolicy.GateAllow {
		return PropertyRef{}, ErrNotFound
	}
	prop, err := properties.Get(ctx, propertyID)
	if err != nil {
		return PropertyRef{}, err
	}
	return prop, nil
}
