package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

// TickService runs the materialization tick (ADR 0051, mirroring ADR 0049
// §3): rules + today → tasks. The run is idempotent — repeated runs between
// the owner's midnights converge to a no-op — which is what makes both the
// hourly worker and the in-mutation reruns safe. Pauses do not exist in this
// context: an undated rule materializes its single task at creation and
// every dated rule keeps «all due + exactly one future» standing.
//
// The tick body itself is txStores.tickOwner, a capability of any open
// transaction of this context. RunZoneTicks is the worker's hourly door —
// the only production caller outside a mutation; the tests' single-owner
// door is RunOwnerTick; context mutations reach the same body exclusively
// through the runMutation conveyor, inside their own transaction after their
// change. Calling RunOwnerTick from inside a mutation transaction would
// self-deadlock on the second FOR UPDATE of the owner's property rows — the
// conveyor makes that path unreachable.
type TickService struct {
	txStoreFactory
	zones    TickZoneDirectory
	calendar OwnerCalendar
	metrics  *Metrics
}

// NewTickService builds the tick service over the transactional store
// factory, the tick zone directory of the hourly sweep, the owner calendar
// and the heartbeat metrics. A nil calendar is a wiring mistake; the service
// fails on first use rather than silently writing with a zero date.
func NewTickService(
	factory txStoreFactory, zones TickZoneDirectory, calendar OwnerCalendar, metrics *Metrics,
) *TickService {
	return &TickService{txStoreFactory: factory, zones: zones, calendar: calendar, metrics: metrics}
}

// RunOwnerTick materializes one owner's task rules as of the owner's today
// (ADR 0048): one calendar lookup, then one transaction serialized on the
// owner's property rows. The production door is RunZoneTicks (the hourly
// worker); this single-owner form serves the integration suite and bespoke
// tooling — not any production caller.
func (s *TickService) RunOwnerTick(ctx context.Context, ownerID uuid.UUID) error {
	today, err := ownerToday(s.calendar, ctx, ownerID)
	if err != nil {
		return err
	}
	return s.runInTx(ctx, func(stores *txStores) error {
		return stores.tickOwner(ctx, ownerID, today)
	})
}

// RunZoneTicks is the hourly worker sweep of the whole context: list the
// zones, compute one today per zone from the passed instant, and
// materialize every owner of the zone in its own unit of work. Failures are
// isolated — a broken zone or owner does not stop the rest; the joined error
// reports everything that failed. A run without failures records the
// heartbeat (an empty zone list included: a no-op run is a successful run, a
// silent worker is the alert's business).
func (s *TickService) RunZoneTicks(ctx context.Context, now time.Time) error {
	if s.zones == nil {
		return errors.New("tasks tick: tick zone directory must be configured")
	}
	zones, err := s.zones.ListTickZones(ctx)
	if err != nil {
		return fmt.Errorf("list tick zones: %w", err)
	}
	var errs []error
	for _, zone := range zones {
		today, err := zoneToday(now, zone.Timezone)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, ownerID := range zone.Owners {
			if err := s.runInTx(ctx, func(stores *txStores) error {
				return stores.tickOwner(ctx, ownerID, today)
			}); err != nil {
				errs = append(errs, fmt.Errorf("tick owner %s of zone %s: %w", ownerID, zone.Timezone, err))
			}
		}
	}
	err = errors.Join(errs...)
	if err == nil {
		s.metrics.RecordTickSuccess(ctx, now)
	}
	return err
}

// zoneToday computes the sweep's "today" for one zone (ADR 0048 p.2): the
// calendar date of the instant in the zone's IANA location under the
// module's UTC-midnight date convention. An unknown zone name is a data
// integrity error, not a fallback case: users.timezone is IANA-validated on
// write.
func zoneToday(now time.Time, timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("load tick zone %q as location: %w", timezone, err)
	}
	return DateAtUTCMidnight(now, loc), nil
}

// The three tick bodies below are the same plan-and-apply walk over
// different doors and slices (ADR 0052): the lock set defines the
// serialization, the keep predicate picks the slice. Property-bound
// mutations must never take the owner-row lock, and owner mutations never
// take the property rows — the slices' rules are disjoint row sets, and
// crossed lock orders would invite deadlocks.

// tickOwner is the full materialization tick body inside an open transaction
// of this context — the worker's and RunOwnerTick's door: take both
// serialization locks (the owner's property rows first, then the owner's own
// users row — the property-less anchor of ADR 0052) and plan every rule
// across both slices. The caller supplies today — computed once per request
// by the owner calendar, so a mutation and the tick it triggers always agree
// on the day boundary.
func (s *txStores) tickOwner(ctx context.Context, ownerID uuid.UUID, today time.Time) error {
	return s.runTickSlice(ctx, ownerID, today,
		func(rule domain.TaskRule) bool { return true },
		s.tick.LockOwnerProperties, s.tick.LockOwner)
}

// tickOwnerProperties is the property-bound slice of the tick — the body a
// property mutation runs inside its transaction: its own property row is
// already locked, and the slice's rules hang on properties, so the property
// rows are the only serialization this cut needs. The property-less rules
// are not this transaction's business: a bound-rule mutation cannot change
// them.
func (s *txStores) tickOwnerProperties(ctx context.Context, ownerID uuid.UUID, today time.Time) error {
	return s.runTickSlice(ctx, ownerID, today,
		func(rule domain.TaskRule) bool { return rule.PropertyID != nil },
		s.tick.LockOwnerProperties)
}

// tickOwnerWithoutProperty is the property-less slice of the tick (ADR 0052)
// — the body an owner-scope mutation runs inside its transaction, under the
// owner-row lock it took at the conveyor's front: only property-less rules
// are planned and applied.
func (s *txStores) tickOwnerWithoutProperty(ctx context.Context, ownerID uuid.UUID, today time.Time) error {
	return s.runTickSlice(ctx, ownerID, today,
		func(rule domain.TaskRule) bool { return rule.PropertyID == nil },
		s.tick.LockOwner)
}

// runTickSlice takes the serialization locks in the given order (the order
// is the deadlock contract — see the bodies above), loads the owner
// snapshot, then plans and applies the tick for the rules the keep predicate
// admits — one plan per rule, in snapshot order.
func (s *txStores) runTickSlice(
	ctx context.Context, ownerID uuid.UUID, today time.Time,
	keep func(domain.TaskRule) bool,
	locks ...func(context.Context, uuid.UUID) error,
) error {
	for _, lock := range locks {
		if err := lock(ctx, ownerID); err != nil {
			return err
		}
	}
	snapshot, err := s.tick.LoadOwnerSnapshot(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("load owner snapshot: %w", err)
	}
	for _, rule := range snapshot.Rules {
		if !keep(rule) {
			continue
		}
		plan := domain.PlanTaskTick(rule, today, snapshot.Existing[rule.ID])
		if err := s.tick.ApplyTickPlan(ctx, rule, today, plan); err != nil {
			return fmt.Errorf("apply tick plan of rule %s: %w", rule.ID, err)
		}
	}
	return nil
}
