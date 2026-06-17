package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Backoff computes the next retry delay for a failed dispatch attempt.
type Backoff interface {
	Next(attempt int) time.Duration
}

// ReminderWorker polls for due reminders and dispatches them through a Notifier.
type ReminderWorker struct {
	repo        application.ReminderRepository
	notifier    application.Notifier
	db          transaction.Beginner
	clock       clock.Clock
	backoff     Backoff
	maxAttempts int
	interval    time.Duration
	logger      *slog.Logger
}

// NewReminderWorker creates a new reminder dispatch worker.
func NewReminderWorker(
	repo application.ReminderRepository,
	notifier application.Notifier,
	db transaction.Beginner,
	clock clock.Clock,
	backoff Backoff,
	maxAttempts int,
	interval time.Duration,
	logger *slog.Logger,
) *ReminderWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReminderWorker{
		repo:        repo,
		notifier:    notifier,
		db:          db,
		clock:       clock,
		backoff:     backoff,
		maxAttempts: maxAttempts,
		interval:    interval,
		logger:      logger,
	}
}

// Run starts the worker loop. It stops when the provided context is cancelled.
func (w *ReminderWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	if err := w.tick(ctx); err != nil {
		w.logger.ErrorContext(ctx, "reminder worker tick failed", "error", err)
	}

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

	for {
		tx, err := w.db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}

		txRepo := w.repo.WithTx(tx)
		reminders, err := txRepo.ListDue(ctx, now, 1)
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("list due reminders: %w", err)
		}
		if len(reminders) == 0 {
			_ = tx.Rollback(ctx)
			break
		}

		r := reminders[0]
		if err := w.dispatch(ctx, r, txRepo, now); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit transaction: %w", err)
		}
	}
	return nil
}

func (w *ReminderWorker) dispatch(ctx context.Context, r domain.Reminder, repo application.ReminderRepository, now time.Time) error {
	if err := w.notifier.Notify(ctx, application.Notification{
		RecipientID: r.OwnerID,
		ReminderID:  r.ID,
		EventType:   r.EventType,
		Title:       r.MessageTitle,
		Body:        r.MessageBody,
	}); err != nil {
		w.logger.ErrorContext(ctx, "notify reminder failed", "reminder_id", r.ID, "event_type", r.EventType, "error", err)
		return w.markFailure(ctx, repo, r, now)
	}

	if err := repo.MarkSent(ctx, r.ID, now); err != nil {
		return fmt.Errorf("mark reminder sent: %w", err)
	}
	return nil
}

func (w *ReminderWorker) markFailure(ctx context.Context, repo application.ReminderRepository, r domain.Reminder, now time.Time) error {
	attempts := r.FailedAttempts + 1
	terminal := attempts >= w.maxAttempts

	var next *time.Time
	if !terminal {
		n := now.Add(w.backoff.Next(attempts))
		next = &n
	}

	if err := repo.MarkFailed(ctx, r.ID, next, terminal); err != nil {
		return fmt.Errorf("mark reminder failed: %w", err)
	}
	return nil
}
