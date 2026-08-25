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
// transaction of this context: RunOwnerTick is the standalone driver (the
// worker's), while context mutations (#457) run the same body inside their
// own transaction after their change (ADR 0048, decision №1) — with the
// owner's property rows already locked, so a nested standalone run would
// self-deadlock on the same FOR UPDATE.
type TickService struct {
	txStoreFactory
	calendar OwnerCalendar
}

// NewTickService builds the tick service over the transactional store factory
// and the owner calendar. A nil calendar is a wiring mistake; the service
// fails on first use rather than silently writing with a zero date.
func NewTickService(factory txStoreFactory, calendar OwnerCalendar) *TickService {
	return &TickService{txStoreFactory: factory, calendar: calendar}
}

// RunOwnerTick materializes one owner's payment rules as of the owner's
// today (ADR 0048): one calendar lookup, then one transaction serialized on
// the owner's property rows.
func (s *TickService) RunOwnerTick(ctx context.Context, ownerID uuid.UUID) error {
	if s.calendar == nil {
		return errors.New("payments tick: owner calendar must be configured")
	}
	today, err := s.calendar.Today(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("resolve owner today: %w", err)
	}
	return s.runInTx(ctx, func(stores *txStores) error {
		return stores.tickOwner(ctx, ownerID, today)
	})
}

// tickOwner is the materialization tick body inside an open transaction of
// this context (ADR 0049 §3): take the serialization lock, load the owner
// snapshot, then compute and apply each rule's plan. The caller supplies
// today — computed once per request by the owner calendar, so a mutation and
// the tick it triggers always agree on the day boundary.
func (s *txStores) tickOwner(ctx context.Context, ownerID uuid.UUID, today time.Time) error {
	if err := s.tick.LockOwnerProperties(ctx, ownerID); err != nil {
		return fmt.Errorf("lock owner properties: %w", err)
	}
	snapshot, err := s.tick.LoadOwnerSnapshot(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("load owner snapshot: %w", err)
	}
	if len(snapshot.Payments) == 0 {
		return nil
	}
	for _, p := range snapshot.Payments {
		plan := domain.PlanPaymentTick(p, today, snapshot.Statuses[p.ID])
		if err := s.tick.ApplyTickPlan(ctx, p, today, plan); err != nil {
			return fmt.Errorf("apply tick plan of payment %s: %w", p.ID, err)
		}
	}
	return nil
}
