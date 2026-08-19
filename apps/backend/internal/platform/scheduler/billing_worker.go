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
	// The lockKey field elects the tick leader through a PostgreSQL advisory
	// lock; the production default is billingWorkerLockKey, tests override it
	// so parallel DB-backed tick tests do not elect each other's leader (same
	// pattern as OperationOverdueWorker.nextRun).
	lockKey int64
}

// NewBillingWorker creates a new billing lifecycle worker.
func NewBillingWorker(
	renewals RenewalProcessor,
	scheduled ScheduledChangeProcessor,
	pool *pgxpool.Pool,
	clk clock.Clock,
	interval time.Duration,
	logger *slog.Logger,
) *BillingWorker {
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
		clock:     clk,
		interval:  interval,
		logger:    logger,
		lockKey:   billingWorkerLockKey,
	}
}

// Run starts the worker loop. It stops when the provided context is cancelled.
func (w *BillingWorker) Run(ctx context.Context) {
	runTickerLoop(ctx, "billing", w.interval, w.logger, w.tick)
}

func (w *BillingWorker) tick(ctx context.Context) error {
	if w.pool == nil {
		return errors.New("billing worker requires a database pool")
	}
	return withAdvisoryTickLock(ctx, w.pool, w.lockKey, "billing", w.logger, w.processTick)
}

// processTick is one leader-elected pass of the subscription lifecycle; the
// advisory lock is held by tick for its whole duration.
func (w *BillingWorker) processTick(ctx context.Context) error {
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
