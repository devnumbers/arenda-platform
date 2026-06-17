package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// Backoff computes the next retry delay for a failed dispatch attempt.
type Backoff interface {
	Next(attempt int) time.Duration
}

// ReminderWorker polls for due reminders and dispatches them through a Notifier.
type ReminderWorker struct {
	repo        application.ReminderRepository
	resolver    application.ContactResolver
	notifier    application.Notifier
	clock       clock.Clock
	backoff     Backoff
	maxAttempts int
	interval    time.Duration
	batchSize   int
	logger      *slog.Logger
}

// NewReminderWorker creates a new reminder dispatch worker.
func NewReminderWorker(
	repo application.ReminderRepository,
	resolver application.ContactResolver,
	notifier application.Notifier,
	clock clock.Clock,
	backoff Backoff,
	maxAttempts int,
	interval time.Duration,
	batchSize int,
	logger *slog.Logger,
) *ReminderWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReminderWorker{
		repo:        repo,
		resolver:    resolver,
		notifier:    notifier,
		clock:       clock,
		backoff:     backoff,
		maxAttempts: maxAttempts,
		interval:    interval,
		batchSize:   batchSize,
		logger:      logger,
	}
}

// Run starts the worker loop. It stops when the provided context is cancelled.
func (w *ReminderWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.ErrorContext(ctx, "reminder worker tick failed", "error", err)
			}
		}
	}
}

func (w *ReminderWorker) tick(ctx context.Context) error {
	now := w.clock.Now()
	reminders, err := w.repo.ListDue(ctx, now, w.batchSize)
	if err != nil {
		return err
	}

	for _, r := range reminders {
		if err := w.dispatch(ctx, r, now); err != nil {
			w.logger.ErrorContext(ctx, "dispatch reminder failed", "reminder_id", r.ID, "error", err)
		}
	}
	return nil
}

func (w *ReminderWorker) dispatch(ctx context.Context, r domain.Reminder, now time.Time) error {
	if _, err := w.resolver.Resolve(ctx, r.OwnerID); err != nil {
		return w.markFailure(ctx, r, now)
	}

	if err := w.notifier.Notify(ctx, application.Notification{
		RecipientID: r.OwnerID,
		ReminderID:  r.ID,
		EventType:   r.EventType,
		Title:       r.MessageTitle,
		Body:        r.MessageBody,
	}); err != nil {
		return w.markFailure(ctx, r, now)
	}

	if err := w.repo.MarkSent(ctx, r.ID, now); err != nil {
		return err
	}
	return nil
}

func (w *ReminderWorker) markFailure(ctx context.Context, r domain.Reminder, now time.Time) error {
	attempts := r.FailedAttempts + 1
	terminal := attempts >= w.maxAttempts

	var next *time.Time
	if !terminal {
		n := now.Add(w.backoff.Next(attempts))
		next = &n
	}

	if err := w.repo.MarkFailed(ctx, r.ID, next, terminal); err != nil {
		w.logger.ErrorContext(ctx, "mark reminder failed", "reminder_id", r.ID, "error", err)
		return err
	}
	return nil
}
