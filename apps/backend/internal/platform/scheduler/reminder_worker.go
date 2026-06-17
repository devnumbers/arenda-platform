package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// staleSendingTimeout is the age after which a sending reminder is considered stuck
// and reset to pending so another dispatch attempt can be made.
const staleSendingTimeout = 5 * time.Minute

// Backoff computes the next retry delay for a failed dispatch attempt.
type Backoff interface {
	Next(attempt int) time.Duration
}

// ReminderWorker polls for due reminders and dispatches them through a Notifier.
type ReminderWorker struct {
	repo            application.ReminderRepository
	notifier        application.Notifier
	resolver        application.ContactResolver
	db              transaction.Beginner
	clock           clock.Clock
	backoff         Backoff
	maxAttempts     int
	interval        time.Duration
	dispatchTimeout time.Duration
	logger          *slog.Logger
}

// NewReminderWorker creates a new reminder dispatch worker.
func NewReminderWorker(
	repo application.ReminderRepository,
	notifier application.Notifier,
	resolver application.ContactResolver,
	db transaction.Beginner,
	clock clock.Clock,
	backoff Backoff,
	maxAttempts int,
	interval time.Duration,
	dispatchTimeout time.Duration,
	logger *slog.Logger,
) *ReminderWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &ReminderWorker{
		repo:            repo,
		notifier:        notifier,
		resolver:        resolver,
		db:              db,
		clock:           clock,
		backoff:         backoff,
		maxAttempts:     maxAttempts,
		interval:        interval,
		dispatchTimeout: dispatchTimeout,
		logger:          logger,
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

	if err := w.dispatchDue(ctx, now); err != nil {
		return err
	}
	if err := w.recoverStaleSending(ctx, now); err != nil {
		return err
	}
	return nil
}

func (w *ReminderWorker) dispatchDue(ctx context.Context, now time.Time) error {
	for {
		tx1, err := w.db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin claim transaction: %w", err)
		}

		txRepo := w.repo.WithTx(tx1)
		reminders, err := txRepo.ListDue(ctx, now, 1)
		if err != nil {
			_ = tx1.Rollback(ctx)
			return fmt.Errorf("list due reminders: %w", err)
		}
		if len(reminders) == 0 {
			_ = tx1.Rollback(ctx)
			break
		}

		r := reminders[0]
		if _, err := txRepo.MarkReminderSending(ctx, r.ID); err != nil {
			_ = tx1.Rollback(ctx)
			if errors.Is(err, application.ErrNotFound) {
				continue
			}
			return fmt.Errorf("mark reminder sending: %w", err)
		}

		if err := tx1.Commit(ctx); err != nil {
			return fmt.Errorf("commit claim transaction: %w", err)
		}

		if err := w.dispatchReminder(ctx, r, now); err != nil {
			if errors.Is(err, application.ErrConcurrentUpdate) {
				w.logger.InfoContext(ctx, "reminder changed concurrently, skipping", "reminder_id", r.ID)
				continue
			}
			w.logger.ErrorContext(ctx, "dispatch reminder failed", "reminder_id", r.ID, "event_type", r.EventType, "error", err)
			if recErr := w.recoverFinalizeFailure(ctx, r.ID); recErr != nil {
				w.logger.ErrorContext(ctx, "reminder finalize recovery failed", "reminder_id", r.ID, "error", recErr)
			}
			continue
		}
	}
	return nil
}

func (w *ReminderWorker) dispatchReminder(ctx context.Context, r domain.Reminder, now time.Time) error {
	contact, err := w.resolver.Resolve(ctx, r.OwnerID)
	if err != nil {
		w.logger.ErrorContext(ctx, "resolve contact failed", "reminder_id", r.ID, "event_type", r.EventType, "error", err)
		return w.finalizeFailure(ctx, r, now)
	}
	if contact.Channel != application.ChannelSMS {
		w.logger.ErrorContext(ctx, "unsupported contact channel", "reminder_id", r.ID, "event_type", r.EventType, "channel", contact.Channel)
		return w.finalizeFailure(ctx, r, now)
	}

	notifyCtx, cancel := context.WithTimeout(ctx, w.dispatchTimeout)
	defer cancel()
	providerResponse, err := w.notifier.Notify(notifyCtx, application.Notification{
		RecipientID: r.OwnerID,
		ReminderID:  r.ID,
		EventType:   r.EventType,
		Title:       r.MessageTitle,
		Body:        r.MessageBody,
		Contact:     &contact,
	})
	if err != nil {
		w.logger.ErrorContext(ctx, "notify reminder failed", "reminder_id", r.ID, "event_type", r.EventType, "error", err)
		return w.finalizeFailure(ctx, r, now)
	}

	return w.finalizeSuccess(ctx, r, now, contact.Address, providerResponse)
}

func (w *ReminderWorker) finalizeSuccess(ctx context.Context, r domain.Reminder, now time.Time, phone, providerResponse string) error {
	tx2, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize transaction: %w", err)
	}
	defer func() { _ = tx2.Rollback(ctx) }()

	txRepo := w.repo.WithTx(tx2)

	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("generate sent sms id: %w", err)
	}
	if err := txRepo.SaveSentSMSReminder(ctx, id, r.ID, r.OwnerID, phone, r.MessageBody, providerResponse, now); err != nil {
		return fmt.Errorf("save sent sms reminder: %w", err)
	}
	if err := txRepo.MarkSent(ctx, r.ID, now); err != nil {
		if errors.Is(err, application.ErrConcurrentUpdate) {
			w.logger.InfoContext(ctx, "reminder already marked sent by another worker", "reminder_id", r.ID)
			return nil
		}
		return fmt.Errorf("mark reminder sent: %w", err)
	}

	if err := tx2.Commit(ctx); err != nil {
		return fmt.Errorf("commit finalize transaction: %w", err)
	}
	return nil
}

func (w *ReminderWorker) finalizeFailure(ctx context.Context, r domain.Reminder, now time.Time) error {
	tx2, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize transaction: %w", err)
	}
	defer func() { _ = tx2.Rollback(ctx) }()

	txRepo := w.repo.WithTx(tx2)

	attempts := r.FailedAttempts + 1
	terminal := attempts >= w.maxAttempts

	var next *time.Time
	if !terminal {
		n := now.Add(w.backoff.Next(attempts))
		next = &n
	}

	if err := txRepo.MarkFailed(ctx, r.ID, next, terminal); err != nil {
		return fmt.Errorf("mark reminder failed: %w", err)
	}

	if err := tx2.Commit(ctx); err != nil {
		return fmt.Errorf("commit finalize transaction: %w", err)
	}
	return nil
}

func (w *ReminderWorker) recoverFinalizeFailure(ctx context.Context, id uuid.UUID) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize recovery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := w.repo.WithTx(tx)
	if err := txRepo.MarkSendingReminderPending(ctx, id); err != nil {
		return fmt.Errorf("mark sending reminder pending: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit finalize recovery transaction: %w", err)
	}
	return nil
}

func (w *ReminderWorker) recoverStaleSending(ctx context.Context, now time.Time) error {
	staleBefore := now.Add(-staleSendingTimeout)

	for {
		tx, err := w.db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin stale recovery transaction: %w", err)
		}

		txRepo := w.repo.WithTx(tx)
		reminders, err := txRepo.ListStaleSendingReminders(ctx, staleBefore, 1)
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("list stale sending reminders: %w", err)
		}
		if len(reminders) == 0 {
			_ = tx.Rollback(ctx)
			break
		}

		r := reminders[0]
		if err := txRepo.ResetReminderSending(ctx, r.ID); err != nil {
			_ = tx.Rollback(ctx)
			if errors.Is(err, application.ErrConcurrentUpdate) {
				w.logger.InfoContext(ctx, "stale sending reminder changed concurrently, skipping", "reminder_id", r.ID)
				continue
			}
			return fmt.Errorf("reset stale sending reminder: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit stale recovery transaction: %w", err)
		}
	}
	return nil
}
