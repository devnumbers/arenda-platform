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
// worker: auto-renewal charges, grace dunning retries, pending upgrade
// payments, grace-expiry reminders and expired grace handling (ADR 0008).
type RenewalProcessor interface {
	ProcessRenewals(ctx context.Context, now time.Time) (int, error)
	ProcessGraceRetries(ctx context.Context, now time.Time) (int, error)
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

	var errs []error
	for _, phase := range []func(context.Context, time.Time) error{
		w.processScheduledChanges,
		w.processRenewals,
		// The dunning retries run right after the renewals: a charge that
		// entered grace this tick is not retried today (the schedule anchors at
		// the grace entry, +24 h out), a window whose retry is due is (ticket
		// #431).
		w.processGraceRetries,
		w.processPendingUpgrades,
		// The grace-expiry reminder runs before the expired-grace downgrade:
		// a window closing this tick is reminded first, and once the window
		// has ended the reminder is moot (issue #253).
		w.processGraceExpiryReminders,
		w.processExpiredGrace,
	} {
		if err := phase(ctx, now); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// processScheduledChanges applies due deferred tariff changes and wraps a
// failure for the joined tick error.
func (w *BillingWorker) processScheduledChanges(ctx context.Context, now time.Time) error {
	count, err := w.scheduled.ProcessScheduledChanges(ctx, now)
	if err != nil {
		w.logger.ErrorContext(ctx, "billing worker scheduled changes processing failed", "error", sanitize.Error(err))
		return fmt.Errorf("scheduled changes: %w", err)
	}
	if count > 0 {
		w.logger.InfoContext(ctx, "billing worker applied scheduled changes", "count", count)
	}
	return nil
}

// processRenewals charges the due auto-renewals and wraps a failure for the
// joined tick error.
func (w *BillingWorker) processRenewals(ctx context.Context, now time.Time) error {
	count, err := w.renewals.ProcessRenewals(ctx, now)
	if err != nil {
		w.logger.ErrorContext(ctx, "billing worker renewal processing failed", "error", sanitize.Error(err))
		return fmt.Errorf("renewals: %w", err)
	}
	if count > 0 {
		w.logger.InfoContext(ctx, "billing worker processed renewals", "count", count)
	}
	return nil
}

// processGraceRetries charges the due dunning retries of grace subscriptions
// and wraps a failure for the joined tick error.
func (w *BillingWorker) processGraceRetries(ctx context.Context, now time.Time) error {
	count, err := w.renewals.ProcessGraceRetries(ctx, now)
	if err != nil {
		w.logger.ErrorContext(ctx, "billing worker grace retry processing failed", "error", sanitize.Error(err))
		return fmt.Errorf("grace retries: %w", err)
	}
	if count > 0 {
		w.logger.InfoContext(ctx, "billing worker charged grace retries", "count", count)
	}
	return nil
}

// processPendingUpgrades finalizes pending upgrade payments and wraps a
// failure for the joined tick error.
func (w *BillingWorker) processPendingUpgrades(ctx context.Context, now time.Time) error {
	count, err := w.renewals.ProcessPendingUpgradePayments(ctx, now)
	if err != nil {
		w.logger.ErrorContext(ctx, "billing worker pending upgrade processing failed", "error", sanitize.Error(err))
		return fmt.Errorf("pending upgrades: %w", err)
	}
	if count > 0 {
		w.logger.InfoContext(ctx, "billing worker finalized pending upgrade payments", "count", count)
	}
	return nil
}

// processGraceExpiryReminders dispatches the grace-expiry reminders and wraps
// a failure for the joined tick error.
func (w *BillingWorker) processGraceExpiryReminders(ctx context.Context, now time.Time) error {
	count, err := w.renewals.ProcessGraceExpiryReminders(ctx, now)
	if err != nil {
		w.logger.ErrorContext(ctx, "billing worker grace expiry reminders failed", "error", sanitize.Error(err))
		return fmt.Errorf("grace expiry reminders: %w", err)
	}
	if count > 0 {
		w.logger.InfoContext(ctx, "billing worker dispatched grace expiry reminders", "count", count)
	}
	return nil
}

// processExpiredGrace downgrades the subscriptions whose grace window has
// ended and wraps a failure for the joined tick error.
func (w *BillingWorker) processExpiredGrace(ctx context.Context, now time.Time) error {
	count, err := w.renewals.ProcessExpiredGrace(ctx, now)
	if err != nil {
		w.logger.ErrorContext(ctx, "billing worker expired grace processing failed", "error", sanitize.Error(err))
		return fmt.Errorf("expired grace: %w", err)
	}
	if count > 0 {
		w.logger.InfoContext(ctx, "billing worker downgraded expired grace subscriptions", "count", count)
	}
	return nil
}
