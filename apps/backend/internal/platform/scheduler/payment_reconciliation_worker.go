package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// PaymentReconciler is the billing port consumed only by this worker shell;
// per the consumer-side interface rule (ADR 0035) it is declared here next to
// its consumer. It reconciles payments the webhooks may have lost: stale
// pending payments and payments stuck in the refunding state.
type PaymentReconciler interface {
	ReconcilePendingPayments(ctx context.Context, now time.Time) (int, error)
	ReconcileStaleRefunds(ctx context.Context, now time.Time) (int, error)
}

// paymentReconciliationWorkerLockKey is a stable application-level key for the
// PostgreSQL advisory lock used to ensure only one payment reconciliation
// worker runs at a time.
const paymentReconciliationWorkerLockKey int64 = 0xB112

// PaymentReconciliationWorker periodically reconciles stuck pending
// subscription payments. It delegates the actual reconciliation to the billing
// payments port so the worker stays a thin scheduling shell.
type PaymentReconciliationWorker struct {
	payments PaymentReconciler
	pool     *pgxpool.Pool
	clock    clock.Clock
	interval time.Duration
	logger   *slog.Logger
}

// NewPaymentReconciliationWorker creates a new payment reconciliation worker.
func NewPaymentReconciliationWorker(
	payments PaymentReconciler,
	pool *pgxpool.Pool,
	clk clock.Clock,
	interval time.Duration,
	logger *slog.Logger,
) *PaymentReconciliationWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &PaymentReconciliationWorker{
		payments: payments,
		pool:     pool,
		clock:    clk,
		interval: interval,
		logger:   logger,
	}
}

// Run starts the worker loop. It stops when the provided context is cancelled.
func (w *PaymentReconciliationWorker) Run(ctx context.Context) {
	runTickerLoop(ctx, "payment reconciliation", w.interval, w.logger, w.tick)
}

func (w *PaymentReconciliationWorker) tick(ctx context.Context) error {
	// Without a pool there is no leader election to run (unit tests and local
	// dev construct the worker without one); reconciliation then runs unlocked.
	if w.pool == nil {
		return w.processTick(ctx)
	}
	return withAdvisoryTickLock(ctx, w.pool, paymentReconciliationWorkerLockKey, "payment reconciliation", w.logger, w.processTick)
}

// processTick is one reconciliation pass; when a pool is configured, the
// advisory lock is held by tick for its whole duration.
func (w *PaymentReconciliationWorker) processTick(ctx context.Context) error {
	now := w.clock.Now().UTC()

	count, err := w.payments.ReconcilePendingPayments(ctx, now)
	if err != nil {
		w.logger.ErrorContext(ctx, "payment reconciliation failed", "error", sanitize.Error(err))
		return fmt.Errorf("reconcile pending payments: %w", err)
	}
	w.logger.InfoContext(ctx, "payment reconciliation processed pending payments", "count", count)

	refundCount, err := w.payments.ReconcileStaleRefunds(ctx, now)
	if err != nil {
		w.logger.ErrorContext(ctx, "stale refund reconciliation failed", "error", sanitize.Error(err))
		return fmt.Errorf("reconcile stale refunds: %w", err)
	}
	w.logger.InfoContext(ctx, "payment reconciliation processed stale refunding payments", "count", refundCount)
	return nil
}
