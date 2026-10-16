package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
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
// at a time (the tasks sweep has its own namespace, 0xB114).
const paymentsTickWorkerLockKey int64 = 0xB113

// NewPaymentsTickWorker creates the payments tick worker: the shared
// ZoneTickWorker shell over the payments sweep and lock namespace.
func NewPaymentsTickWorker(
	ticker PaymentsTicker,
	pool *pgxpool.Pool,
	clk clock.Clock,
	interval time.Duration,
	logger *slog.Logger,
) *ZoneTickWorker {
	return newZoneTickWorker("payments tick", paymentsTickWorkerLockKey, ticker.RunZoneTicks, pool, clk, interval, logger)
}
