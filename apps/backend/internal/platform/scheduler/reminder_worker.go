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
	beginner    transaction.Beginner
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
	notifier application.Notifier,
	beginner transaction.Beginner,
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
		notifier:    notifier,
		beginner:    beginner,
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

	// Process each batch inside a single transaction. The ListDue query holds
	// FOR UPDATE SKIP LOCKED row locks for the selected reminders, and those
	// locks are kept until the transaction commits. Calling the external
	// Notifier inside the transaction guarantees that a concurrent worker or
	// instance cannot dispatch the same reminder while the notify call is in
	// flight, which prevents duplicate sends without additional idempotency
	// infrastructure.
	//
	// Operational trade-off: slow or unresponsive providers extend the lock
	// hold time and can delay other dispatch work. If that becomes a problem,
	// replace this single long transaction with per-reminder short
	// transactions (or advisory locks) so the external call happens outside
	// the database transaction.
	tx, err := w.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := w.repo.WithTx(tx)

	reminders, err := txRepo.ListDue(ctx, now, w.batchSize)
	if err != nil {
		return fmt.Errorf("list due reminders: %w", err)
	}

	for _, r := range reminders {
		if err := w.dispatch(ctx, txRepo, r, now); err != nil {
			w.logger.ErrorContext(ctx, "dispatch reminder failed", "reminder_id", r.ID, "error", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (w *ReminderWorker) dispatch(ctx context.Context, repo application.ReminderRepository, r domain.Reminder, now time.Time) error {
	if err := w.notifier.Notify(ctx, application.Notification{
		RecipientID: r.OwnerID,
		ReminderID:  r.ID,
		EventType:   r.EventType,
		Title:       r.MessageTitle,
		Body:        r.MessageBody,
	}); err != nil {
		w.logger.ErrorContext(ctx, "notify reminder failed", "reminder_id", r.ID, "error", err)
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
