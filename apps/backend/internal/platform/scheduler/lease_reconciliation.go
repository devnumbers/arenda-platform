package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasedomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// LeaseReconciliationWorker scans open leases with an end date in the past and
// ensures their status moves to requires_action and a requires_action reminder
// exists.
type LeaseReconciliationWorker struct {
	leases    application.LeaseRepository
	scheduler notificationsapp.ReminderScheduler
	beginner  transaction.Beginner
	clock     clock.Clock
	interval  time.Duration
	logger    *slog.Logger
}

// NewLeaseReconciliationWorker creates a new lease reconciliation worker.
func NewLeaseReconciliationWorker(
	leases application.LeaseRepository,
	scheduler notificationsapp.ReminderScheduler,
	beginner transaction.Beginner,
	clock clock.Clock,
	interval time.Duration,
	logger *slog.Logger,
) *LeaseReconciliationWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &LeaseReconciliationWorker{
		leases:    leases,
		scheduler: scheduler,
		beginner:  beginner,
		clock:     clock,
		interval:  interval,
		logger:    logger,
	}
}

// Run starts the reconciliation loop. It stops when the provided context is cancelled.
func (w *LeaseReconciliationWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.ErrorContext(ctx, "lease reconciliation tick failed", "error", err)
			}
		}
	}
}

func (w *LeaseReconciliationWorker) tick(ctx context.Context) error {
	leases, err := w.leases.ListOpenLeasesWithPastEndDate(ctx)
	if err != nil {
		return fmt.Errorf("list open leases with past end date: %w", err)
	}

	for _, lease := range leases {
		if err := w.reconcile(ctx, lease); err != nil {
			w.logger.ErrorContext(ctx, "reconcile lease failed", "lease_id", lease.ID, "error", err)
		}
	}
	return nil
}

func (w *LeaseReconciliationWorker) reconcile(ctx context.Context, lease leasedomain.Lease) error {
	tx, err := w.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txLeases := w.leases.WithTx(tx)
	txScheduler := w.scheduler.WithTx(tx)

	if lease.Status != leasedomain.LeaseStatusRequiresAction {
		lease.Status = leasedomain.LeaseStatusRequiresAction
		lease.UpdatedAt = w.clock.Now()
		if _, err := txLeases.Update(ctx, lease.OwnerID, lease); err != nil {
			return fmt.Errorf("update lease status: %w", err)
		}
	}

	if err := txScheduler.ScheduleForLease(ctx, notificationsapp.LeaseInfo{
		ID:         lease.ID,
		OwnerID:    lease.OwnerID,
		PropertyID: lease.PropertyID,
		EndDate:    lease.EndDate,
	}); err != nil {
		return fmt.Errorf("schedule lease reminder: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

