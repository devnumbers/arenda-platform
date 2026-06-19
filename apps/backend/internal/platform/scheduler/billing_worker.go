package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// billingWorkerLockKey is a stable application-level key for the PostgreSQL
// advisory lock used to ensure only one billing worker runs at a time.
const billingWorkerLockKey int64 = 0xB111

// BillingWorker periodically processes subscription renewals and expired grace
// periods. It delegates the actual billing decisions to BillingService so the
// worker stays a thin scheduling shell.
type BillingWorker struct {
	billing  *billingapp.BillingService
	pool     *pgxpool.Pool
	clock    clock.Clock
	interval time.Duration
	logger   *slog.Logger
}

// NewBillingWorker creates a new billing lifecycle worker.
func NewBillingWorker(billing *billingapp.BillingService, pool *pgxpool.Pool, clock clock.Clock, interval time.Duration, logger *slog.Logger) *BillingWorker {
	if interval <= 0 {
		interval = time.Hour
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &BillingWorker{
		billing:  billing,
		pool:     pool,
		clock:    clock,
		interval: interval,
		logger:   logger,
	}
}

// Run starts the worker loop. It stops when the provided context is cancelled.
func (w *BillingWorker) Run(ctx context.Context) {
	w.logger.InfoContext(ctx, "billing worker started", "interval", w.interval.String())

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	if err := w.tick(ctx); err != nil {
		w.logger.ErrorContext(ctx, "billing worker tick failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.ErrorContext(ctx, "billing worker tick failed", "error", err)
			}
		}
	}
}

func (w *BillingWorker) tick(ctx context.Context) error {
	if w.pool != nil {
		conn, err := w.pool.Acquire(ctx)
		if err != nil {
			return fmt.Errorf("acquire db connection for lock: %w", err)
		}
		defer conn.Release()

		var acquired bool
		if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", billingWorkerLockKey).Scan(&acquired); err != nil {
			return fmt.Errorf("acquire advisory lock: %w", err)
		}
		if !acquired {
			w.logger.InfoContext(ctx, "billing worker tick skipped, another instance holds the lock")
			return nil
		}
		defer func() {
			_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", billingWorkerLockKey)
		}()
	}

	now := w.clock.Now().UTC()

	scheduled, scheduledErr := w.billing.ProcessScheduledChanges(ctx, now)
	if scheduledErr != nil {
		w.logger.ErrorContext(ctx, "billing worker scheduled changes processing failed", "error", scheduledErr)
	} else if scheduled > 0 {
		w.logger.InfoContext(ctx, "billing worker applied scheduled changes", "count", scheduled)
	}

	renewed, renewalErr := w.billing.ProcessRenewals(ctx, now)
	if renewalErr != nil {
		w.logger.ErrorContext(ctx, "billing worker renewal processing failed", "error", renewalErr)
	} else if renewed > 0 {
		w.logger.InfoContext(ctx, "billing worker processed renewals", "count", renewed)
	}

	downgraded, graceErr := w.billing.ProcessExpiredGrace(ctx, now)
	if graceErr != nil {
		w.logger.ErrorContext(ctx, "billing worker expired grace processing failed", "error", graceErr)
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
	if graceErr != nil {
		errs = append(errs, fmt.Errorf("expired grace: %w", graceErr))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
