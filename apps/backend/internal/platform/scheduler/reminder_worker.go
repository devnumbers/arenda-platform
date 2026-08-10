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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// staleSendingTimeout is the age after which a sending reminder is considered stuck
// and reset to pending so another dispatch attempt can be made.
const staleSendingTimeout = 5 * time.Minute

// Backoff computes the next retry delay for a failed dispatch attempt.
type Backoff interface {
	Next(attempt int) time.Duration
}

// ReminderWorker polls for due reminders and dispatches them through Notifiers.
type ReminderWorker struct {
	repo            application.ReminderRepository
	renderer        *mailer.Renderer
	notifiers       map[application.Channel]application.Notifier
	resolver        application.ContactResolver
	recipients      application.PropertyRecipientLister
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
	renderer *mailer.Renderer,
	notifiers map[application.Channel]application.Notifier,
	resolver application.ContactResolver,
	recipients application.PropertyRecipientLister,
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
		renderer:        renderer,
		notifiers:       notifiers,
		resolver:        resolver,
		recipients:      recipients,
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
	w.logger.InfoContext(ctx, "reminder worker started", "interval", w.interval.String())

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	if err := w.tick(ctx); err != nil {
		w.logger.ErrorContext(ctx, "reminder worker tick failed", "error", sanitize.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.ErrorContext(ctx, "reminder worker tick failed", "error", sanitize.Error(err))
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
			w.logger.ErrorContext(ctx, "dispatch reminder failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
			if recErr := w.recoverFinalizeFailure(ctx, r.ID); recErr != nil {
				w.logger.ErrorContext(ctx, "reminder finalize recovery failed", "reminder_id", r.ID, "error", sanitize.Error(recErr))
			}
			continue
		}
	}
	return nil
}

func (w *ReminderWorker) dispatchReminder(ctx context.Context, r domain.Reminder, now time.Time) error {
	dispatchCtx, cancel := context.WithTimeout(ctx, w.dispatchTimeout)
	defer cancel()

	// Fan-out (issue #159): a property reminder goes to the owner and to every
	// active member of the property. Each recipient is checked against their
	// own notification preferences and deduplicated by their own audit row.
	recipientIDs := []uuid.UUID{r.OwnerID}
	if r.PropertyID != nil {
		memberIDs, err := w.recipients.ListActiveRecipientIDs(dispatchCtx, *r.PropertyID)
		if err != nil {
			w.logger.ErrorContext(dispatchCtx, "list property reminder recipients failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
			return w.finalizeFailure(dispatchCtx, r, now)
		}
		for _, id := range memberIDs {
			if id != r.OwnerID {
				recipientIDs = append(recipientIDs, id)
			}
		}
	}

	var delivered, skippedByPrefs, failures int
	var providerResponse string
	for _, recipientID := range recipientIDs {
		// The current dispatch path always sends email, so the per-channel
		// preference check (ADR 0030) is against the email channel. When push
		// dispatch is added, it will check domain.ChannelPush independently.
		allowed, err := w.repo.IsChannelAllowed(dispatchCtx, recipientID, r.EventType, domain.ChannelEmail)
		if err != nil {
			w.logger.ErrorContext(dispatchCtx, "check notification permission failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
			failures++
			continue
		}
		if !allowed {
			w.logger.InfoContext(dispatchCtx, "reminder recipient skipped: event type not allowed by user preferences", "reminder_id", r.ID, "event_type", r.EventType)
			skippedByPrefs++
			continue
		}

		contact, err := w.resolver.Resolve(dispatchCtx, recipientID)
		if err != nil {
			if errors.Is(err, application.ErrNoContact) {
				w.logger.InfoContext(dispatchCtx, "reminder recipient skipped: no contact", "reminder_id", r.ID, "event_type", r.EventType)
				continue
			}
			w.logger.ErrorContext(dispatchCtx, "resolve contact failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
			failures++
			continue
		}

		notifier, ok := w.notifiers[contact.Channel]
		if !ok {
			w.logger.ErrorContext(dispatchCtx, "unsupported contact channel", "reminder_id", r.ID, "event_type", r.EventType, "channel", contact.Channel)
			failures++
			continue
		}

		// TODO: re-add SMS dispatch here when ContactResolver supports ChannelSMS.
		outcome, response := w.dispatchEmailReminder(dispatchCtx, r, now, recipientID, contact, notifier)
		switch outcome {
		case emailDispatchSent:
			delivered++
			providerResponse = response
		case emailDispatchDuplicate:
			delivered++
		default:
			failures++
		}
	}

	switch {
	case failures > 0:
		// The retry re-sends only to recipients without an audit row
		// (per-recipient dedup), so delivered recipients are not notified twice.
		return w.finalizeFailure(dispatchCtx, r, now)
	case delivered > 0:
		return w.finalizeSuccess(dispatchCtx, r, now, providerResponse, application.ChannelEmail)
	case skippedByPrefs == len(recipientIDs):
		// skipped = every recipient revoked permission for the event type.
		return w.finalizeSkipped(dispatchCtx, r)
	default:
		// No recipient has a resolvable contact: cancel the reminder.
		return w.finalizeCancel(dispatchCtx, r)
	}
}

// emailDispatchOutcome is the per-recipient result of dispatchEmailReminder.
type emailDispatchOutcome int

const (
	emailDispatchSent emailDispatchOutcome = iota
	emailDispatchDuplicate
	emailDispatchFailed
)

// dispatchEmailReminder delivers the reminder to a single recipient. It does
// not finalize the reminder: the caller aggregates per-recipient outcomes and
// finalizes once after the fan-out loop.
func (w *ReminderWorker) dispatchEmailReminder(ctx context.Context, r domain.Reminder, now time.Time, recipientID uuid.UUID, contact application.Contact, notifier application.Notifier) (emailDispatchOutcome, string) {
	// Defensive check: the worker-level sending status already ensures a single
	// processing attempt, but this guards against duplicate sends after
	// stale-sending recovery, failed-run retries or concurrent dispatch races.
	alreadySent, err := w.repo.IsEmailReminderSent(ctx, r.ID, recipientID)
	if err != nil {
		w.logger.ErrorContext(ctx, "check sent email reminder failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return emailDispatchFailed, ""
	}
	if alreadySent {
		w.logger.InfoContext(ctx, "reminder already sent to recipient, skipping notification", "reminder_id", r.ID, "event_type", r.EventType)
		return emailDispatchDuplicate, ""
	}

	plain, _, err := w.renderer.Render("reminder", map[string]any{
		"Subject": r.MessageTitle,
		"Title":   r.MessageTitle,
		"Body":    r.MessageBody,
	})
	if err != nil {
		w.logger.ErrorContext(ctx, "render reminder email failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return emailDispatchFailed, ""
	}

	id, err := uuid.NewV7()
	if err != nil {
		w.logger.ErrorContext(ctx, "generate sent email id failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return emailDispatchFailed, ""
	}

	if err := w.repo.SaveSentEmailReminder(ctx, application.SaveSentEmailReminderParams{
		ID:         id,
		ReminderID: r.ID,
		ScopeID:    recipientID,
		Email:      contact.Email,
		Subject:    r.MessageTitle,
		PlainBody:  plain,
		SentAt:     now,
	}); errors.Is(err, application.ErrDuplicateEmailReminder) {
		w.logger.InfoContext(ctx, "reminder already sent to recipient, skipping notification", "reminder_id", r.ID, "event_type", r.EventType)
		return emailDispatchDuplicate, ""
	} else if err != nil {
		w.logger.ErrorContext(ctx, "save sent email reminder failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return emailDispatchFailed, ""
	}

	providerResponse, _, err := notifier.Notify(ctx, application.Notification{
		RecipientID: recipientID,
		ReminderID:  r.ID,
		EventType:   r.EventType,
		Title:       r.MessageTitle,
		Body:        r.MessageBody,
		Contact:     &contact,
	})
	if err != nil {
		w.logger.ErrorContext(ctx, "notify reminder failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		if delErr := w.repo.DeleteSentEmailReminder(ctx, r.ID, recipientID); delErr != nil {
			w.logger.ErrorContext(ctx, "failed to delete email audit row after send failure", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(delErr))
		}
		return emailDispatchFailed, ""
	}

	return emailDispatchSent, providerResponse
}

func (w *ReminderWorker) finalizeSuccess(ctx context.Context, r domain.Reminder, now time.Time, providerResponse string, channel application.Channel) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := w.repo.WithTx(tx)

	if err := txRepo.MarkSent(ctx, r.ID, now); err != nil {
		return fmt.Errorf("mark reminder sent: %w", err)
	}

	if channel == application.ChannelSMS {
		if err := txRepo.UpdateSMSProviderResponse(ctx, r.ID, providerResponse); err != nil {
			return fmt.Errorf("update sms provider response: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit finalize transaction: %w", err)
	}
	return nil
}

func (w *ReminderWorker) finalizeFailure(ctx context.Context, r domain.Reminder, now time.Time) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := w.repo.WithTx(tx)

	attempts := r.FailedAttempts + 1
	terminal := attempts >= w.maxAttempts

	var next *time.Time
	if !terminal {
		next = new(now.Add(w.backoff.Next(attempts)))
	}

	if err := txRepo.MarkFailed(ctx, r.ID, next, terminal); err != nil {
		return fmt.Errorf("mark reminder failed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit finalize transaction: %w", err)
	}
	return nil
}

func (w *ReminderWorker) finalizeCancel(ctx context.Context, r domain.Reminder) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := w.repo.WithTx(tx)
	if _, err := txRepo.CancelByIDAndOwner(ctx, r.OwnerID, r.ID); err != nil {
		return fmt.Errorf("cancel reminder: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit finalize transaction: %w", err)
	}
	return nil
}

// finalizeSkipped marks a reminder as skipped: every recipient revoked
// permission for its event type, so it must not be sent or retried.
func (w *ReminderWorker) finalizeSkipped(ctx context.Context, r domain.Reminder) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := w.repo.WithTx(tx)
	if err := txRepo.MarkReminderSkipped(ctx, r.ID); err != nil {
		return fmt.Errorf("mark reminder skipped: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
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
	r, err := txRepo.GetByIDUnscoped(ctx, id)
	if err != nil {
		return fmt.Errorf("get reminder for recovery: %w", err)
	}
	nextAttemptAt := w.clock.Now().Add(w.backoff.Next(r.FailedAttempts + 1))
	if err := txRepo.MarkSendingReminderPending(ctx, id, nextAttemptAt); err != nil {
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
