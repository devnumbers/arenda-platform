package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// Consumer-side, single-method ports (ADR 0035). The Cleaner is the only
// caller of these repository methods, so each contract lives here, next to its
// consumer, instead of coupling the adapter to the full application repository
// interfaces. The postgres repositories satisfy them through structural typing;
// conformance is checked at the assignment site in wire/workers.go.

// ExpiredSessionDeleter removes sessions whose expiry falls before the given
// instant and reports the number of rows removed.
type ExpiredSessionDeleter interface {
	DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error)
}

// ExpiredLoginCodeDeleter removes login codes whose expiry falls before the
// given instant and reports the number of rows removed.
type ExpiredLoginCodeDeleter interface {
	DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error)
}

// StaleAttemptDeleter removes login-attempt windows older than the given
// instant and reports the number of rows removed.
type StaleAttemptDeleter interface {
	DeleteStaleBefore(ctx context.Context, before time.Time) (int64, error)
}

// Cleaner periodically removes expired identity data.
type Cleaner struct {
	sessions  ExpiredSessionDeleter
	codes     ExpiredLoginCodeDeleter
	attempts  StaleAttemptDeleter
	clock     clock.Clock
	interval  time.Duration
	retention time.Duration
	logger    *slog.Logger
}

// NewCleaner creates a Cleaner with the given repositories and schedule.
func NewCleaner(
	sessions ExpiredSessionDeleter,
	codes ExpiredLoginCodeDeleter,
	attempts StaleAttemptDeleter,
	clock clock.Clock,
	interval, retention time.Duration,
	logger *slog.Logger,
) *Cleaner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Cleaner{
		sessions:  sessions,
		codes:     codes,
		attempts:  attempts,
		clock:     clock,
		interval:  interval,
		retention: retention,
		logger:    logger,
	}
}

// Run starts the cleanup loop and blocks until ctx is cancelled. The first
// cycle runs immediately so a restart does not postpone cleanup by a full
// interval; subsequent cycles run on the ticker.
func (c *Cleaner) Run(ctx context.Context) {
	c.clean(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.clean(ctx)
		}
	}
}

func (c *Cleaner) clean(ctx context.Context) {
	before := c.clock.Now().UTC().Add(-c.retention)

	codes := c.cleanExpired(ctx, "login codes", before, c.codes.DeleteExpiredBefore)
	sessions := c.cleanExpired(ctx, "sessions", before, c.sessions.DeleteExpiredBefore)
	attempts := c.cleanExpired(ctx, "login attempts", before, c.attempts.DeleteStaleBefore)

	c.logger.InfoContext(ctx, "cleanup completed",
		slog.Time("before", before),
		slog.Int64("login_codes_deleted", codes),
		slog.Int64("sessions_deleted", sessions),
		slog.Int64("login_attempts_deleted", attempts),
	)
}

// cleanExpired runs a single DeleteExpiredBefore/DeleteStaleBefore call, logs
// any non-cancellation error and reports how many rows were removed. The
// adapter loops the SQL batch internally, so the scheduler only speaks the
// "delete old data" intent.
func (c *Cleaner) cleanExpired(
	ctx context.Context,
	name string,
	before time.Time,
	deleteBefore func(context.Context, time.Time) (int64, error),
) int64 {
	n, err := deleteBefore(ctx, before)
	if err != nil && !errors.Is(err, context.Canceled) {
		c.logger.ErrorContext(ctx, "failed to clean expired "+name, slog.String("error", sanitize.Error(err)))
	}
	return n
}
