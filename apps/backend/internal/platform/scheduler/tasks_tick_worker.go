package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// tasksTickWorkerLockKey is a stable application-level key for the
// PostgreSQL advisory lock used to ensure only one tasks tick worker runs at
// a time (its own namespace: the payments sweep has 0xB113).
const tasksTickWorkerLockKey int64 = 0xB114

// NewTasksTickWorker creates the tasks tick worker: the shared
// ZoneTickWorker shell over the tasks zone sweep (TickService.RunZoneTicks —
// the hourly sweep of ADR 0051, mirroring ADR 0048 p.3: one today per owner
// timezone, every owner materialized in its own unit of work, failures
// isolated per owner; the implementation records the success heartbeat the
// "tick has not run in N hours" alert watches) and the tasks lock namespace.
func NewTasksTickWorker(
	runZoneTicks func(ctx context.Context, now time.Time) error,
	pool *pgxpool.Pool,
	clk clock.Clock,
	interval time.Duration,
	logger *slog.Logger,
) *ZoneTickWorker {
	return newZoneTickWorker("tasks tick", tasksTickWorkerLockKey, runZoneTicks, pool, clk, interval, logger)
}
