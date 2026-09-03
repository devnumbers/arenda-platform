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

// ZoneTickWorker is the shared hourly shell of the context materialization
// sweeps (payments — ADR 0048, tasks — ADR 0051): the wall-clock-aligned
// ticker loop, the advisory-lock leader election on its own namespace and
// the logging around one zone sweep pass. The context-specific part is only
// the sweep function (the context's RunZoneTicks) and the advisory-lock key;
// each context keeps its consumer-side port and thin constructor next to its
// own code (ADR 0035).
type ZoneTickWorker struct {
	sweep    func(ctx context.Context, now time.Time) error
	pool     *pgxpool.Pool
	clock    clock.Clock
	interval time.Duration
	name     string
	lockKey  int64
	logger   *slog.Logger
}

// newZoneTickWorker creates the shared shell: name for logs and the lock
// diagnostics, lockKey for the advisory-lock namespace, sweep for one pass.
// An interval ≤ 0 means the domain-decided hourly cadence.
func newZoneTickWorker(
	name string, lockKey int64, sweep func(ctx context.Context, now time.Time) error,
	pool *pgxpool.Pool, clk clock.Clock, interval time.Duration, logger *slog.Logger,
) *ZoneTickWorker {
	if interval <= 0 {
		interval = time.Hour
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ZoneTickWorker{
		sweep:    sweep,
		pool:     pool,
		clock:    clk,
		interval: interval,
		name:     name,
		lockKey:  lockKey,
		logger:   logger,
	}
}

// Run starts the worker loop. It stops when the provided context is cancelled.
func (w *ZoneTickWorker) Run(ctx context.Context) {
	runTickerLoop(ctx, w.name, w.interval, w.logger, w.tick)
}

func (w *ZoneTickWorker) tick(ctx context.Context) error {
	// Without a pool there is no leader election to run (unit tests and local
	// dev construct the worker without one); the sweep then runs unlocked.
	if w.pool == nil {
		return w.processTick(ctx)
	}
	return withAdvisoryTickLock(ctx, w.pool, w.lockKey, w.name, w.logger, w.processTick)
}

// processTick is one zone sweep pass; when a pool is configured, the
// advisory lock is held by tick for its whole duration.
func (w *ZoneTickWorker) processTick(ctx context.Context) error {
	now := w.clock.Now().UTC()

	if err := w.sweep(ctx, now); err != nil {
		w.logger.ErrorContext(ctx, "zone tick sweep failed", "worker", w.name, "error", sanitize.Error(err))
		return fmt.Errorf("run zone ticks: %w", err)
	}
	w.logger.InfoContext(ctx, "zone tick sweep completed", "worker", w.name)
	return nil
}
