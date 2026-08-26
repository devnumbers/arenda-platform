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

// PaymentsTicker is the payments port consumed only by this worker shell;
// per the consumer-side interface rule (ADR 0035) it is declared here next
// to its consumer. RunZoneTicks is the hourly zone sweep of the
// materialization tick (ADR 0048 p.3): one today per owner timezone, every
// owner of the zone materialized in its own unit of work, failures isolated
// per owner. The heartbeat of a fully successful sweep is recorded by the
// implementation — the "tick has not run in N hours" alert watches it.
type PaymentsTicker interface {
	RunZoneTicks(ctx context.Context, now time.Time) error
}

// paymentsTickWorkerLockKey is a stable application-level key for the
// PostgreSQL advisory lock used to ensure only one payments tick worker runs
// at a time.
const paymentsTickWorkerLockKey int64 = 0xB113

// PaymentsTickWorker periodically sweeps the materialization tick across the
// owner timezones. It delegates the actual materialization to the payments
// tick port so the worker stays a thin scheduling shell.
type PaymentsTickWorker struct {
	ticker   PaymentsTicker
	pool     *pgxpool.Pool
	clock    clock.Clock
	interval time.Duration
	logger   *slog.Logger
}

// NewPaymentsTickWorker creates a new payments tick worker.
func NewPaymentsTickWorker(
	ticker PaymentsTicker,
	pool *pgxpool.Pool,
	clk clock.Clock,
	interval time.Duration,
	logger *slog.Logger,
) *PaymentsTickWorker {
	if interval <= 0 {
		interval = time.Hour
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &PaymentsTickWorker{
		ticker:   ticker,
		pool:     pool,
		clock:    clk,
		interval: interval,
		logger:   logger,
	}
}

// Run starts the worker loop. It stops when the provided context is cancelled.
func (w *PaymentsTickWorker) Run(ctx context.Context) {
	runTickerLoop(ctx, "payments tick", w.interval, w.logger, w.tick)
}

func (w *PaymentsTickWorker) tick(ctx context.Context) error {
	// Without a pool there is no leader election to run (unit tests and local
	// dev construct the worker without one); the sweep then runs unlocked.
	if w.pool == nil {
		return w.processTick(ctx)
	}
	return withAdvisoryTickLock(ctx, w.pool, paymentsTickWorkerLockKey, "payments tick", w.logger, w.processTick)
}

// processTick is one zone sweep pass; when a pool is configured, the
// advisory lock is held by tick for its whole duration.
func (w *PaymentsTickWorker) processTick(ctx context.Context) error {
	now := w.clock.Now().UTC()

	if err := w.ticker.RunZoneTicks(ctx, now); err != nil {
		w.logger.ErrorContext(ctx, "payments tick sweep failed", "error", sanitize.Error(err))
		return fmt.Errorf("run zone ticks: %w", err)
	}
	w.logger.InfoContext(ctx, "payments tick sweep completed")
	return nil
}
