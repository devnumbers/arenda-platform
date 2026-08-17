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
func NewPaymentReconciliationWorker(payments PaymentReconciler, pool *pgxpool.Pool, clock clock.Clock, interval time.Duration, logger *slog.Logger) *PaymentReconciliationWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &PaymentReconciliationWorker{
		payments: payments,
		pool:     pool,
		clock:    clock,
		interval: interval,
		logger:   logger,
	}
}

// Run starts the worker loop. It stops when the provided context is cancelled.
func (w *PaymentReconciliationWorker) Run(ctx context.Context) {
	w.logger.InfoContext(ctx, "payment reconciliation worker started", "interval", w.interval.String())

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	if err := w.tick(ctx); err != nil {
		w.logger.ErrorContext(ctx, "payment reconciliation tick failed", "error", sanitize.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.ErrorContext(ctx, "payment reconciliation tick failed", "error", sanitize.Error(err))
			}
		}
	}
}

func (w *PaymentReconciliationWorker) tick(ctx context.Context) error {
	if w.pool != nil {
		acquired, release, err := w.acquireTickLock(ctx)
		if err != nil {
			return err
		}
		if !acquired {
			w.logger.InfoContext(ctx, "payment reconciliation tick skipped, another instance holds the lock")
			return nil
		}
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		defer release(releaseCtx)
	}

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

// acquireTickLock elects a single leader for the current tick using a PostgreSQL
// advisory lock. The returned release function releases the lock and returns
// the dedicated connection to the pool; it must be called exactly once when the
// caller no longer needs the lock.
func (w *PaymentReconciliationWorker) acquireTickLock(ctx context.Context) (bool, func(context.Context), error) {
	conn, err := w.pool.Acquire(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("acquire db connection for lock: %w", err)
	}

	release := func(releaseCtx context.Context) {
		if _, unlockErr := conn.Exec(releaseCtx, "SELECT pg_advisory_unlock($1)", paymentReconciliationWorkerLockKey); unlockErr != nil {
			w.logger.ErrorContext(releaseCtx, "payment reconciliation failed to release advisory lock", "error", sanitize.Error(unlockErr))
		}
		conn.Release()
	}

	var acquired bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", paymentReconciliationWorkerLockKey).Scan(&acquired); err != nil {
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
