package cleaner

import (
	"context"
	"log/slog"
	"time"

	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
)

// Cleaner periodically removes expired identity data.
type Cleaner struct {
	sessions  identityapp.SessionRepository
	codes     identityapp.SMSCodeRepository
	attempts  identityapp.AttemptRepository
	interval  time.Duration
	retention time.Duration
	logger    *slog.Logger
}

// New creates a Cleaner with the given repositories and schedule.
func New(sessions identityapp.SessionRepository, codes identityapp.SMSCodeRepository, attempts identityapp.AttemptRepository, interval, retention time.Duration, logger *slog.Logger) *Cleaner {
	return &Cleaner{
		sessions:  sessions,
		codes:     codes,
		attempts:  attempts,
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
	before := time.Now().UTC().Add(-c.retention)

	if err := c.codes.DeleteExpiredBefore(ctx, before); err != nil {
		c.logger.Error("failed to clean expired sms codes", slog.String("error", err.Error()))
	}
	if err := c.sessions.DeleteExpiredBefore(ctx, before); err != nil {
		c.logger.Error("failed to clean expired sessions", slog.String("error", err.Error()))
	}
	if err := c.attempts.DeleteStaleBefore(ctx, before); err != nil {
		c.logger.Error("failed to clean stale login attempts", slog.String("error", err.Error()))
	}

	c.logger.Info("cleanup completed", slog.Time("before", before))
}
