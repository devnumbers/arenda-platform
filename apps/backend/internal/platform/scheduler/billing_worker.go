package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// The two interfaces below are consumed only by this package (the billing
// worker shell), so per the consumer-side interface rule (ADR 0035) they are
// declared here next to their consumer rather than in the billing application.
// The billing module provides the implementation; conformance is checked at
// the wiring site (cmd/api/wire).

// ScheduledChangeProcessor applies due deferred tariff changes.
type ScheduledChangeProcessor interface {
	ProcessScheduledChanges(ctx context.Context, now time.Time) (int, error)
}

// RenewalProcessor drives the subscription lifecycle phases of the billing
// worker: auto-renewal charges, pending upgrade payments, grace-expiry
// reminders and expired grace handling (ADR 0008).
type RenewalProcessor interface {
	ProcessRenewals(ctx context.Context, now time.Time) (int, error)
	ProcessPendingUpgradePayments(ctx context.Context, now time.Time) (int, error)
	ProcessGraceExpiryReminders(ctx context.Context, now time.Time) (int, error)
	ProcessExpiredGrace(ctx context.Context, now time.Time) (int, error)
}

// billingWorkerLockKey is a stable application-level key for the PostgreSQL
// advisory lock used to ensure only one billing worker runs at a time.
const billingWorkerLockKey int64 = 0xB111

// BillingWorker periodically processes subscription renewals and expired grace
// periods. It delegates the actual billing decisions to the billing ports so the
// worker stays a thin scheduling shell.
type BillingWorker struct {
	renewals  RenewalProcessor
	scheduled ScheduledChangeProcessor
	pool      *pgxpool.Pool
	clock     clock.Clock
	interval  time.Duration
	logger    *slog.Logger
}

// NewBillingWorker creates a new billing lifecycle worker.
func NewBillingWorker(renewals RenewalProcessor, scheduled ScheduledChangeProcessor, pool *pgxpool.Pool, clock clock.Clock, interval time.Duration, logger *slog.Logger) *BillingWorker {
	if interval <= 0 {
		interval = time.Hour
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &BillingWorker{
		renewals:  renewals,
		scheduled: scheduled,
		pool:      pool,
		clock:     clock,
		interval:  interval,
		logger:    logger,
	}
}

// Run starts the worker loop. It stops when the provided context is cancelled.
func (w *BillingWorker) Run(ctx context.Context) {
	w.logger.InfoContext(ctx, "billing worker started", "interval", w.interval.String())

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	if err := w.tick(ctx); err != nil {
		w.logger.ErrorContext(ctx, "billing worker tick failed", "error", sanitize.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.ErrorContext(ctx, "billing worker tick failed", "error", sanitize.Error(err))
			}
		}
	}
}

func (w *BillingWorker) tick(ctx context.Context) error {
	if w.pool == nil {
		return errors.New("billing worker requires a database pool")
	}

	acquired, release, err := w.acquireTickLock(ctx)
	if err != nil {
		return err
	}
	if !acquired {
		w.logger.InfoContext(ctx, "billing worker tick skipped, another instance holds the lock")
		return nil
	}
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	defer release(releaseCtx)

	now := w.clock.Now().UTC()

	scheduled, scheduledErr := w.scheduled.ProcessScheduledChanges(ctx, now)
	if scheduledErr != nil {
		w.logger.ErrorContext(ctx, "billing worker scheduled changes processing failed", "error", sanitize.Error(scheduledErr))
	} else if scheduled > 0 {
		w.logger.InfoContext(ctx, "billing worker applied scheduled changes", "count", scheduled)
	}

	renewed, renewalErr := w.renewals.ProcessRenewals(ctx, now)
	if renewalErr != nil {
		w.logger.ErrorContext(ctx, "billing worker renewal processing failed", "error", sanitize.Error(renewalErr))
	} else if renewed > 0 {
		w.logger.InfoContext(ctx, "billing worker processed renewals", "count", renewed)
	}

	upgrades, upgradeErr := w.renewals.ProcessPendingUpgradePayments(ctx, now)
	if upgradeErr != nil {
		w.logger.ErrorContext(ctx, "billing worker pending upgrade processing failed", "error", sanitize.Error(upgradeErr))
	} else if upgrades > 0 {
		w.logger.InfoContext(ctx, "billing worker finalized pending upgrade payments", "count", upgrades)
	}

	// The grace-expiry reminder runs before the expired-grace downgrade: a
	// window closing this tick is reminded first, and once the window has
	// ended the reminder is moot (issue #253).
	reminded, reminderErr := w.renewals.ProcessGraceExpiryReminders(ctx, now)
	if reminderErr != nil {
		w.logger.ErrorContext(ctx, "billing worker grace expiry reminders failed", "error", sanitize.Error(reminderErr))
	} else if reminded > 0 {
		w.logger.InfoContext(ctx, "billing worker dispatched grace expiry reminders", "count", reminded)
	}

	downgraded, graceErr := w.renewals.ProcessExpiredGrace(ctx, now)
	if graceErr != nil {
		w.logger.ErrorContext(ctx, "billing worker expired grace processing failed", "error", sanitize.Error(graceErr))
	} else if downgraded > 0 {
		w.logger.InfoContext(ctx, "billing worker downgraded expired grace subscriptions", "count", downgraded)
	}

	var errs []error
	if scheduledErr != nil {
		errs = append(errs, fmt.Errorf("scheduled changes: %w", scheduledErr))
	}
	if renewalErr != nil {
		errs = append(errs, fmt.Errorf("renewals: %w", renewalErr))
	}
	if upgradeErr != nil {
		errs = append(errs, fmt.Errorf("pending upgrades: %w", upgradeErr))
	}
	if reminderErr != nil {
		errs = append(errs, fmt.Errorf("grace expiry reminders: %w", reminderErr))
	}
	if graceErr != nil {
		errs = append(errs, fmt.Errorf("expired grace: %w", graceErr))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// acquireTickLock elects a single leader for the current tick using a PostgreSQL
// advisory lock. The returned release function releases the lock and returns
// the dedicated connection to the pool; it must be called exactly once when the
// caller no longer needs the lock.
func (w *BillingWorker) acquireTickLock(ctx context.Context) (bool, func(context.Context), error) {
	conn, err := w.pool.Acquire(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("acquire db connection for lock: %w", err)
	}

	release := func(releaseCtx context.Context) {
		if _, unlockErr := conn.Exec(releaseCtx, "SELECT pg_advisory_unlock($1)", billingWorkerLockKey); unlockErr != nil {
			w.logger.ErrorContext(releaseCtx, "billing worker failed to release advisory lock", "error", sanitize.Error(unlockErr))
		}
		conn.Release()
	}

	var acquired bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", billingWorkerLockKey).Scan(&acquired); err != nil {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		release(releaseCtx)
		return false, nil, fmt.Errorf("acquire advisory lock: %w", err)
	}
	if !acquired {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		release(releaseCtx)
		return false, nil, nil
	}
	return true, release, nil
}
