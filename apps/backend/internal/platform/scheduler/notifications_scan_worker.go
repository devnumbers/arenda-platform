package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// NotificationsScanRunner is the notifications port consumed only by this
// worker shell; per the consumer-side interface rule (ADR 0035) it is
// declared here next to its consumer. RunZoneScans is the hourly zone sweep
// of the rental-completed publisher (карта #734, #748): one today per owner
// timezone (ADR 0048 p.3), every needs_attention rental of the zone
// published — idempotently, through the row's dedup key, so the hourly
// cadence lands the notification on the first pass after the owner's
// midnight following the planned end.
type NotificationsScanRunner interface {
	RunZoneScans(ctx context.Context, now time.Time) error
}

// notificationsScanWorkerLockKey is a stable application-level key for the
// PostgreSQL advisory lock used to ensure only one notifications scan worker
// runs at a time (the payments sweep has 0xB113, the tasks sweep 0xB114).
const notificationsScanWorkerLockKey int64 = 0xB115

// NewNotificationsScanWorker creates the notifications scan worker: the
// shared ZoneTickWorker shell over the rental-completed sweep and its own
// lock namespace.
func NewNotificationsScanWorker(
	scan NotificationsScanRunner,
	pool *pgxpool.Pool,
	clk clock.Clock,
	interval time.Duration,
	logger *slog.Logger,
) *ZoneTickWorker {
	return newZoneTickWorker("notifications scan", notificationsScanWorkerLockKey, scan.RunZoneScans, pool, clk, interval, logger)
}
