package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// TickService runs the materialization tick (ADR 0048, ADR 0049 §3): rules +
// pauses + today → operations. The run is idempotent — repeated runs between
// the owner's midnights converge to a no-op — which is what makes both the
// hourly worker (#458) and the in-mutation reruns safe.
//
// The tick body itself is txStores.tickOwner, a capability of any open
// transaction of this context. RunZoneTicks is the worker's hourly door
// (ticket #458) — the only production caller outside a mutation; the tests'
// single-owner door is RunOwnerTick; context mutations reach the same body
// exclusively through the runMutation conveyor, inside their own transaction
// after their change (ADR 0048, decision №1). Calling RunOwnerTick from
// inside a mutation transaction would self-deadlock on the second FOR UPDATE
// of the owner's property rows — the conveyor makes that path unreachable.
type TickService struct {
	txStoreFactory
	zones    TickZoneDirectory
	calendar OwnerCalendar
	metrics  *Metrics
}

// NewTickService builds the tick service over the transactional store factory,
// the tick zone directory of the hourly sweep (ADR 0048 p.3), the owner
// calendar and the heartbeat metrics. A nil calendar is a wiring mistake; the
// service fails on first use rather than silently writing with a zero date.
func NewTickService(
	factory txStoreFactory, zones TickZoneDirectory, calendar OwnerCalendar, metrics *Metrics,
) *TickService {
	return &TickService{txStoreFactory: factory, zones: zones, calendar: calendar, metrics: metrics}
}

// RunOwnerTick materializes one owner's payment rules as of the owner's
// today (ADR 0048): one calendar lookup, then one transaction serialized on
// the owner's property rows. The production door is RunZoneTicks (the hourly
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

// RunZoneTicks is the hourly worker sweep of the whole context (ticket #458,
// ADR 0048 p.3): list the zones, compute one today per zone from the passed
// instant, and materialize every owner of the zone in its own unit of work.
// Failures are isolated — a broken zone or owner does not stop the rest; the
// joined error reports everything that failed. A run without failures
// records the heartbeat (an empty zone list included: a no-op run is a
// successful run, a silent worker is the alert's business).
func (s *TickService) RunZoneTicks(ctx context.Context, now time.Time) error {
	if s.zones == nil {
		return errors.New("payments tick: tick zone directory must be configured")
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

// tickOwner is the materialization tick body inside an open transaction of
// this context (ADR 0049 §3): take the serialization lock, load the owner
// snapshot, then compute and apply each rule's plan. The caller supplies
// today — computed once per request by the owner calendar, so a mutation and
// the tick it triggers always agree on the day boundary.
func (s *txStores) tickOwner(ctx context.Context, ownerID uuid.UUID, today time.Time) error {
	return RunOwnerTickInTx(ctx, s.tick, ownerID, today)
}

// RunOwnerTickInTx is the tick body of tickOwner over a bare TickStore: the
// single home of the lock → snapshot → plan → apply sequence, shared with the
// rentals context's in-mutation tick (ADR 0053 §3) — its wiring-bound
// RentPaymentGateway calls this with the payments TickStore bound to the
// rentals transaction, so the sync of the managed payment ticks exactly like
// a payments mutation, with no second conveyor.
func RunOwnerTickInTx(ctx context.Context, tick TickStore, ownerID uuid.UUID, today time.Time) error {
	if err := tick.LockOwnerProperties(ctx, ownerID); err != nil {
		return fmt.Errorf("lock owner properties: %w", err)
	}
	snapshot, err := tick.LoadOwnerSnapshot(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("load owner snapshot: %w", err)
	}
	if len(snapshot.Payments) == 0 {
		return nil
	}
	for _, p := range snapshot.Payments {
		plan := domain.PlanPaymentTick(p, today, snapshot.Statuses[p.ID])
		if err := tick.ApplyTickPlan(ctx, p, today, plan); err != nil {
			return fmt.Errorf("apply tick plan of payment %s: %w", p.ID, err)
		}
	}
	return nil
}
