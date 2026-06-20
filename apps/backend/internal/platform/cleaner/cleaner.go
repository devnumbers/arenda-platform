package cleaner

import (
	"context"
	"log/slog"
	"time"

	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

const defaultDeleteBatchSize = 1000

// Cleaner periodically removes expired identity data.
type Cleaner struct {
	sessions  identityapp.SessionRepository
	codes     identityapp.SMSCodeRepository
	attempts  identityapp.AttemptRepository
	clock     clock.Clock
	interval  time.Duration
	retention time.Duration
	logger    *slog.Logger
}

// New creates a Cleaner with the given repositories and schedule.
func New(sessions identityapp.SessionRepository, codes identityapp.SMSCodeRepository, attempts identityapp.AttemptRepository, clock clock.Clock, interval, retention time.Duration, logger *slog.Logger) *Cleaner {
	if clock == nil {
		clock = realClock{}
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

	c.deleteInBatches(ctx, "sms codes", c.codes.DeleteExpiredBeforeBatch)
	c.deleteInBatches(ctx, "sessions", c.sessions.DeleteExpiredBeforeBatch)
	c.deleteInBatches(ctx, "login attempts", c.attempts.DeleteStaleBeforeBatch)

	c.logger.Info("cleanup completed", slog.Time("before", before))
}

func (c *Cleaner) deleteInBatches(ctx context.Context, name string, deleteBatch func(context.Context, time.Time, int32) (int64, error)) {
	before := c.clock.Now().UTC().Add(-c.retention)
	for {
		n, err := deleteBatch(ctx, before, defaultDeleteBatchSize)
		if err != nil {
			c.logger.Error("failed to clean expired "+name, slog.String("error", err.Error()))
			return
		}
		if n == 0 {
			return
		}
	}
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }
