package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// TickService runs the materialization tick (ADR 0048, ADR 0049 §3): rules +
// pauses + today → operations, as a per-owner unit of work. The run is
// idempotent — repeated runs between the owner's midnights converge to a
// no-op — which is what makes both the hourly worker (#458) and the
// in-mutation reruns safe.
type TickService struct {
	txStoreFactory
	tz    OwnerTimezoneResolver
	clock clock.Clock
}

// NewTickService builds the tick service over the transactional store factory.
// A nil clock or resolver is a wiring mistake; the service fails on first use
// rather than silently writing with a zero time.
func NewTickService(factory txStoreFactory, tz OwnerTimezoneResolver, clk clock.Clock) *TickService {
	return &TickService{txStoreFactory: factory, tz: tz, clock: clk}
}

// RunOwnerTick materializes one owner's payment rules as of the owner's
// today: the calendar date in the property owner's timezone (ADR 0048),
// computed in Go and passed down as a parameter — no AT TIME ZONE in SQL.
// The whole run is one transaction serialized on the owner's property rows.
func (s *TickService) RunOwnerTick(ctx context.Context, ownerID uuid.UUID) error {
	if s.clock == nil || s.tz == nil {
		return errors.New("payments tick: clock and timezone resolver must be configured")
	}
	loc, err := s.tz.OwnerTimezone(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("resolve owner timezone: %w", err)
	}
	// The owner's calendar date as a UTC-midnight domain date. Note the
	// normalization order: the year/month/day are read in the owner's
	// location and rebuilt at UTC midnight — taking the UTC date of the
	// instant (or midnight in loc) would compare wrongly against the
	// UTC-midnight dates stored in DATE columns (see timeutil.BeforeDay).
	now := s.clock.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return s.runInTx(ctx, func(stores *txStores) error {
		return runTick(ctx, stores.tick, ownerID, today)
	})
}

// runTick is the tick body inside the unit of work: take the serialization
// lock, load the rules with pauses and the existing operation keys, then
// apply each rule's plan.
func runTick(ctx context.Context, store TickStore, ownerID uuid.UUID, today time.Time) error {
	if err := store.LockOwnerProperties(ctx, ownerID); err != nil {
		return fmt.Errorf("lock owner properties: %w", err)
	}
	payments, err := store.LoadOwnerPayments(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("load owner payments: %w", err)
	}
	if len(payments) == 0 {
		return nil
	}

	paymentIDs := make([]uuid.UUID, len(payments))
	for i, p := range payments {
		paymentIDs[i] = p.ID
	}
	statuses, err := store.ListOperationStatuses(ctx, paymentIDs)
	if err != nil {
		return fmt.Errorf("load operation statuses: %w", err)
	}

	for _, p := range payments {
		if err := runPaymentTick(ctx, store, p, today, statuses[p.ID]); err != nil {
			return err
		}
	}
	return nil
}

// runPaymentTick applies one rule's tick plan: materialize the due
// occurrences, close today's occurrence for auto-pay, rebuild the single
// future planned (ADR 0049 §2).
func runPaymentTick(
	ctx context.Context, store TickStore, p domain.Payment, today time.Time,
	existing map[time.Time]domain.OperationStatus,
) error {
	plan := domain.PlanPaymentTick(p, today, existing)

	for _, date := range plan.Materialize {
		if err := insertOccurrence(ctx, store, p, date); err != nil {
			return err
		}
	}
	if plan.InsertFuture != nil {
		if err := insertOccurrence(ctx, store, p, *plan.InsertFuture); err != nil {
			return err
		}
	}
	if plan.AutoPayToday {
		if err := store.PayDueToday(ctx, p.ID, today); err != nil {
			return fmt.Errorf("auto-pay occurrence of payment %s due today: %w", p.ID, err)
		}
	}
	if plan.KeepFuture != nil {
		if err := store.DeleteFuturePlannedExcept(ctx, p.ID, today, *plan.KeepFuture); err != nil {
			return fmt.Errorf("rebuild future planned of payment %s: %w", p.ID, err)
		}
		return nil
	}
	if err := store.DeleteFuturePlannedAll(ctx, p.ID, today); err != nil {
		return fmt.Errorf("remove future planned of payment %s: %w", p.ID, err)
	}
	return nil
}

// insertOccurrence materializes one planned occurrence of the rule; the
// insert is idempotent through the (payment_id, date) partial unique index.
func insertOccurrence(
	ctx context.Context, store TickStore, p domain.Payment, date time.Time,
) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("mint operation id: %w", err)
	}
	op := domain.NewMaterializedOperation(p, date)
	op.ID = id
	if err := store.InsertOperation(ctx, op); err != nil {
		return fmt.Errorf("materialize occurrence %s of payment %s: %w",
			date.Format(time.DateOnly), p.ID, err)
	}
	return nil
}
