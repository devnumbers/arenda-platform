package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// Cleaner periodically removes expired identity data.
type Cleaner struct {
	sessions  identityapp.SessionRepository
	codes     identityapp.LoginCodeRepository
	attempts  identityapp.AttemptRepository
	clock     clock.Clock
	interval  time.Duration
	retention time.Duration
	logger    *slog.Logger
}

// NewCleaner creates a Cleaner with the given repositories and schedule.
func NewCleaner(
	sessions identityapp.SessionRepository,
	codes identityapp.LoginCodeRepository,
	attempts identityapp.AttemptRepository,
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

// Run starts the cleanup loop and blocks until ctx is cancelled.
func (c *Cleaner) Run(ctx context.Context) {
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

	c.cleanExpired(ctx, "login codes", before, c.codes.DeleteExpiredBefore)
	c.cleanExpired(ctx, "sessions", before, c.sessions.DeleteExpiredBefore)
	c.cleanExpired(ctx, "login attempts", before, c.attempts.DeleteStaleBefore)

	c.logger.InfoContext(ctx, "cleanup completed", slog.Time("before", before))
}

// cleanExpired runs a single DeleteExpiredBefore/DeleteStaleBefore call and logs
// any non-cancellation error. The adapter loops the SQL batch internally, so
// the scheduler only speaks the "delete old data" intent.
func (c *Cleaner) cleanExpired(
	ctx context.Context,
	name string,
	before time.Time,
	deleteBefore func(context.Context, time.Time) error,
) {
	if err := deleteBefore(ctx, before); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		c.logger.ErrorContext(ctx, "failed to clean expired "+name, slog.String("error", err.Error()))
	}
}
