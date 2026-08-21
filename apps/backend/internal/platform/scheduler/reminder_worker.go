package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	repo       application.ReminderRepository
	renderer   *mailer.Renderer
	notifiers  map[application.Channel]application.Notifier
	resolver   application.ContactResolver
	recipients application.PropertyRecipientLister
	// The pushSender field dispatches Web Push messages. When nil, push
	// delivery is disabled (e.g. local dev without VAPID keys); the worker
	// still delivers email and manages the reminder lifecycle.
	pushSender application.PushSender
	// The pushSubRepo field reads per-user push subscriptions for fan-out to
	// all devices.
	pushSubRepo     application.PushSubscriptionRepository
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
	pushSender application.PushSender,
	pushSubRepo application.PushSubscriptionRepository,
	db transaction.Beginner,
	clk clock.Clock,
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
		pushSender:      pushSender,
		pushSubRepo:     pushSubRepo,
		db:              db,
		clock:           clk,
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
			w.rollbackClaimTx(ctx, tx1)
			return fmt.Errorf("list due reminders: %w", err)
		}
		if len(reminders) == 0 {
			w.rollbackClaimTx(ctx, tx1)
			break
		}

		r := reminders[0]
		if _, err := txRepo.MarkReminderSending(ctx, r.ID); err != nil {
			w.rollbackClaimTx(ctx, tx1)
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

	recipientIDs, err := w.reminderRecipients(dispatchCtx, r)
	if err != nil {
		w.logger.ErrorContext(dispatchCtx, "list property reminder recipients failed",
			"reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return w.finalizeFailure(dispatchCtx, r, now)
	}

	totals := w.dispatchToRecipients(dispatchCtx, r, now, recipientIDs)
	return w.finalizeDispatch(dispatchCtx, r, now, len(recipientIDs), totals)
}

// reminderRecipients collects the fan-out list (issue #159): the owner plus
// every active member of the property, deduplicated by the owner id.
func (w *ReminderWorker) reminderRecipients(ctx context.Context, r domain.Reminder) ([]uuid.UUID, error) {
	recipientIDs := []uuid.UUID{r.OwnerID}
	if r.PropertyID == nil {
		return recipientIDs, nil
	}
	memberIDs, err := w.recipients.ListActiveRecipientIDs(ctx, *r.PropertyID)
	if err != nil {
		return nil, err
	}
	for _, id := range memberIDs {
		if id != r.OwnerID {
			recipientIDs = append(recipientIDs, id)
		}
	}
	return recipientIDs, nil
}

// reminderDispatchTotals aggregates the per-recipient fan-out outcome the
// finalize decision reads.
type reminderDispatchTotals struct {
	delivered      int
	skippedByPrefs int
	failures       int
	pushDelivered  bool
}

func (w *ReminderWorker) dispatchToRecipients(
	ctx context.Context,
	r domain.Reminder,
	now time.Time,
	recipientIDs []uuid.UUID,
) reminderDispatchTotals {
	var totals reminderDispatchTotals
	for _, recipientID := range recipientIDs {
		outcome, pushDelivered := w.dispatchToRecipient(ctx, r, now, recipientID)
		if pushDelivered {
			totals.pushDelivered = true
		}
		switch outcome {
		case recipientDelivered:
			totals.delivered++
		case recipientDuplicate:
			totals.delivered++
		case recipientSkippedByPrefs:
			totals.skippedByPrefs++
		case recipientFailed:
			totals.failures++
		case recipientNoContact:
			// Neither delivered nor failed: nothing to count.
		}
	}
	return totals
}

// recipientDeliveryOutcome classifies the per-recipient dispatch result for
// the fan-out aggregation.
type recipientDeliveryOutcome int

const (
	// The recipientNoContact outcome means the recipient has no resolvable
	// contact and counts towards neither delivery nor failure.
	recipientNoContact recipientDeliveryOutcome = iota
	recipientDelivered
	recipientDuplicate
	recipientSkippedByPrefs
	recipientFailed
)

// dispatchToRecipient delivers the reminder to one recipient: the email
// permission check, the best-effort push dispatch, contact resolution and the
// email notification itself.
func (w *ReminderWorker) dispatchToRecipient(
	ctx context.Context,
	r domain.Reminder,
	now time.Time,
	recipientID uuid.UUID,
) (outcome recipientDeliveryOutcome, pushDelivered bool) {
	emailAllowed, err := w.repo.IsChannelAllowed(ctx, recipientID, r.EventType, domain.ChannelEmail)
	if err != nil {
		w.logger.ErrorContext(ctx, "check email notification permission failed",
			"reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return recipientFailed, false
	}

	// Push is dispatched independently of email (ADR 0030 per-channel
	// preferences). Push failures are logged but never block email or the
	// reminder lifecycle: email is the authoritative delivery channel for
	// finalize decisions, push is a best-effort side channel.
	if w.pushSender != nil {
		if w.dispatchPushReminder(ctx, r, now, recipientID) {
			pushDelivered = true
		}
	}

	if !emailAllowed {
		w.logger.InfoContext(ctx, "reminder recipient skipped: email not allowed by user preferences",
			"reminder_id", r.ID, "event_type", r.EventType)
		return recipientSkippedByPrefs, pushDelivered
	}

	contact, err := w.resolver.Resolve(ctx, recipientID)
	if err != nil {
		if errors.Is(err, application.ErrNoContact) {
			w.logger.InfoContext(ctx, "reminder recipient skipped: no contact", "reminder_id", r.ID, "event_type", r.EventType)
			return recipientNoContact, pushDelivered
		}
		w.logger.ErrorContext(ctx, "resolve contact failed",
			"reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return recipientFailed, pushDelivered
	}

	notifier, ok := w.notifiers[contact.Channel]
	if !ok {
		w.logger.ErrorContext(ctx, "unsupported contact channel",
			"reminder_id", r.ID, "event_type", r.EventType, "channel", contact.Channel)
		return recipientFailed, pushDelivered
	}

	switch w.dispatchEmailReminder(ctx, r, now, recipientID, contact, notifier) {
	case emailDispatchSent:
		return recipientDelivered, pushDelivered
	case emailDispatchDuplicate:
		return recipientDuplicate, pushDelivered
	default:
		return recipientFailed, pushDelivered
	}
}

// finalizeDispatch applies the fan-out totals to the reminder lifecycle: a
// failure retries, a delivery (email or push-only) finalizes as sent, a
// full preference skip marks it skipped, and no resolvable contact cancels it.
func (w *ReminderWorker) finalizeDispatch(
	ctx context.Context,
	r domain.Reminder,
	now time.Time,
	recipientCount int,
	totals reminderDispatchTotals,
) error {
	switch {
	case totals.failures > 0:
		// The retry re-sends only to recipients without an audit row
		// (per-recipient dedup), so delivered recipients are not notified twice.
		return w.finalizeFailure(ctx, r, now)
	case totals.delivered > 0:
		return w.finalizeSuccess(ctx, r, now)
	case totals.pushDelivered:
		// Email was skipped or undeliverable for every recipient, but push
		// reached at least one device: mark the reminder as sent so it is not
		// retried (push is deduplicated via sent_push_reminders).
		w.logger.InfoContext(ctx, "reminder delivered via push only", "reminder_id", r.ID, "event_type", r.EventType)
		return w.finalizeSuccess(ctx, r, now)
	case totals.skippedByPrefs == recipientCount:
		// The skipped outcome means every recipient revoked permission for the
		// event type.
		return w.finalizeSkipped(ctx, r)
	default:
		// No recipient has a resolvable contact: cancel the reminder.
		return w.finalizeCancel(ctx, r)
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
func (w *ReminderWorker) dispatchEmailReminder(
	ctx context.Context,
	r domain.Reminder,
	now time.Time,
	recipientID uuid.UUID,
	contact application.Contact,
	notifier application.Notifier,
) emailDispatchOutcome {
	// Defensive check: the worker-level sending status already ensures a single
	// processing attempt, but this guards against duplicate sends after
	// stale-sending recovery, failed-run retries or concurrent dispatch races.
	alreadySent, err := w.repo.IsEmailReminderSent(ctx, r.ID, recipientID)
	if err != nil {
		w.logger.ErrorContext(ctx, "check sent email reminder failed",
			"reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return emailDispatchFailed
	}
	if alreadySent {
		w.logger.InfoContext(ctx, "reminder already sent to recipient, skipping notification", "reminder_id", r.ID, "event_type", r.EventType)
		return emailDispatchDuplicate
	}

	plain, _, err := w.renderer.Render("reminder", map[string]any{
		"Subject": r.MessageTitle,
		"Title":   r.MessageTitle,
		"Body":    r.MessageBody,
	})
	if err != nil {
		w.logger.ErrorContext(ctx, "render reminder email failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return emailDispatchFailed
	}

	id, err := uuid.NewV7()
	if err != nil {
		w.logger.ErrorContext(ctx, "generate sent email id failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return emailDispatchFailed
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
		return emailDispatchDuplicate
	} else if err != nil {
		w.logger.ErrorContext(ctx, "save sent email reminder failed",
			"reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return emailDispatchFailed
	}

	if err := notifier.Notify(ctx, application.Notification{
		RecipientID: recipientID,
		ReminderID:  r.ID,
		EventType:   r.EventType,
		Title:       r.MessageTitle,
		Body:        r.MessageBody,
		Contact:     &contact,
	}); err != nil {
		w.logger.ErrorContext(ctx, "notify reminder failed", "reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		if delErr := w.repo.DeleteSentEmailReminder(ctx, r.ID, recipientID); delErr != nil {
			w.logger.ErrorContext(ctx, "failed to delete email audit row after send failure",
				"reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(delErr))
		}
		return emailDispatchFailed
	}

	return emailDispatchSent
}

// dispatchPushReminder delivers the reminder as a Web Push to a single
// recipient's devices. It checks the per-channel push preference, reads all of
// the recipient's subscriptions (fan-out to every device), and deduplicates via
// the sent_push_reminders audit row. It returns true when at least one push was
// accepted by a push service.
//
// Dead subscriptions (404/410) are deleted automatically. Rate-limited (429)
// and transient (5xx) failures are logged; they do not raise a worker-level
// failure because push is a best-effort side channel — the reminder lifecycle
// is driven by email delivery.
func (w *ReminderWorker) dispatchPushReminder(ctx context.Context, r domain.Reminder, now time.Time, recipientID uuid.UUID) bool {
	covered, proceed := w.pushRecipientCovered(ctx, r, recipientID)
	if !proceed {
		return covered
	}

	subs, err := w.pushSubRepo.ListByUser(ctx, recipientID)
	if err != nil {
		w.logger.ErrorContext(ctx, "list push subscriptions failed",
			"reminder_id", r.ID, "recipient_id", recipientID, "error", sanitize.Error(err))
		return false
	}
	if len(subs) == 0 {
		return false
	}

	if saved, alreadyCovered := w.savePushAuditRow(ctx, r, now, recipientID); !saved {
		return alreadyCovered
	}

	if w.pushToSubscriptions(ctx, r, recipientID, subs) == 0 {
		// Roll back the audit row: no push was actually accepted. The recipient
		// will be retried on the next dispatch cycle (or the reminder will be
		// finalized based on email outcome).
		if delErr := w.repo.DeleteSentPushReminder(ctx, r.ID, recipientID); delErr != nil {
			w.logger.ErrorContext(ctx, "delete push audit row after send failure",
				"reminder_id", r.ID, "recipient_id", recipientID, "error", sanitize.Error(delErr))
		}
		return false
	}
	return true
}

// pushRecipientCovered runs the push preflight: the per-channel preference and
// the per-recipient dedup check. The covered result reports the recipient is
// already covered by a prior push; proceed reports the dispatch may continue.
func (w *ReminderWorker) pushRecipientCovered(ctx context.Context, r domain.Reminder, recipientID uuid.UUID) (covered, proceed bool) {
	pushAllowed, err := w.repo.IsChannelAllowed(ctx, recipientID, r.EventType, domain.ChannelPush)
	if err != nil {
		w.logger.ErrorContext(ctx, "check push notification permission failed",
			"reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return false, false
	}
	if !pushAllowed {
		return false, false
	}

	// Per-recipient deduplication: if a push audit row already exists, the
	// recipient was notified on a previous dispatch attempt. Returning true
	// signals "this recipient has been covered by push" so the caller's
	// finalize decision accounts for the prior delivery.
	alreadySent, err := w.repo.IsPushReminderSent(ctx, r.ID, recipientID)
	if err != nil {
		w.logger.ErrorContext(ctx, "check sent push reminder failed",
			"reminder_id", r.ID, "event_type", r.EventType, "error", sanitize.Error(err))
		return false, false
	}
	if alreadySent {
		return true, false
	}
	return false, true
}

// savePushAuditRow inserts the dedup audit row before sending so a crash
// after a successful push does not cause a duplicate on retry. The covered
// result reports a duplicate row: the recipient was already covered by push.
func (w *ReminderWorker) savePushAuditRow(
	ctx context.Context,
	r domain.Reminder,
	now time.Time,
	recipientID uuid.UUID,
) (saved, covered bool) {
	auditID, err := uuid.NewV7()
	if err != nil {
		w.logger.ErrorContext(ctx, "generate push audit id failed", "reminder_id", r.ID, "error", sanitize.Error(err))
		return false, false
	}

	if err := w.repo.SaveSentPushReminder(ctx, application.SaveSentPushReminderParams{
		ID:          auditID,
		ReminderID:  r.ID,
		RecipientID: recipientID,
		SentAt:      now,
	}); err != nil {
		if errors.Is(err, application.ErrDuplicatePushReminder) {
			return false, true
		}
		w.logger.ErrorContext(ctx, "save sent push reminder failed",
			"reminder_id", r.ID, "recipient_id", recipientID, "error", sanitize.Error(err))
		return false, false
	}
	return true, false
}

// pushToSubscriptions sends the payload to every subscription of the
// recipient and returns how many pushes a service accepted.
func (w *ReminderWorker) pushToSubscriptions(
	ctx context.Context,
	r domain.Reminder,
	recipientID uuid.UUID,
	subs []domain.PushSubscription,
) int {
	payload := application.NewPushPayload(r)
	delivered := 0
pushSubs:
	for _, sub := range subs {
		sendErr := w.pushSender.Send(ctx, sub, payload)
		switch {
		case sendErr == nil:
			delivered++
		case errors.Is(sendErr, application.ErrSubscriptionGone):
			// The push service reports the subscription is dead (RFC 8030
			// §7.3): delete it so future dispatches do not waste attempts.
			// The adapter already recorded the "gone" outcome metric.
			if delErr := w.pushSubRepo.Delete(ctx, sub.UserID, sub.Endpoint); delErr != nil {
				w.logger.ErrorContext(ctx, "delete dead push subscription failed",
					"reminder_id", r.ID, "recipient_id", recipientID, "error", sanitize.Error(delErr))
			} else {
				w.logger.InfoContext(ctx, "push subscription removed (gone)", "reminder_id", r.ID, "recipient_id", recipientID)
			}
		case errors.Is(sendErr, application.ErrRateLimited):
			// RFC 8030 §8.4: the push service throttled the request and may
			// carry a Retry-After (parsed and logged by the adapter). Push
			// services rate-limit per endpoint, so the remaining devices for
			// this recipient would almost certainly be throttled too — stop
			// sending to honour the Retry-After and avoid burning the quota.
			// The push audit row is rolled back below so the recipient is
			// retried on the next poll cycle (the worker's natural ~1-min
			// backoff for best-effort side channels).
			w.logger.WarnContext(ctx, "push rate limited, skipping remaining devices for recipient",
				"reminder_id", r.ID, "recipient_id", recipientID, "error", sanitize.Error(sendErr))
			break pushSubs
		default:
			// Transient (5xx) or fatal (400/403) failure. Fatal errors are not
			// retried per push; transient ones are retried on the next poll
			// cycle via the audit rollback below.
			w.logger.ErrorContext(ctx, "send push failed", "reminder_id", r.ID, "recipient_id", recipientID, "error", sanitize.Error(sendErr))
		}
	}
	return delivered
}

func (w *ReminderWorker) finalizeSuccess(
	ctx context.Context,
	r domain.Reminder,
	now time.Time,
) (err error) {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize transaction: %w", err)
	}
	defer w.rollbackOnReturn(ctx, tx, &err)

	txRepo := w.repo.WithTx(tx)

	if err := txRepo.MarkSent(ctx, r.ID, now); err != nil {
		return fmt.Errorf("mark reminder sent: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit finalize transaction: %w", err)
	}
	return nil
}

func (w *ReminderWorker) finalizeFailure(ctx context.Context, r domain.Reminder, now time.Time) (err error) {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize transaction: %w", err)
	}
	defer w.rollbackOnReturn(ctx, tx, &err)

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

func (w *ReminderWorker) finalizeCancel(ctx context.Context, r domain.Reminder) (err error) {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize transaction: %w", err)
	}
	defer w.rollbackOnReturn(ctx, tx, &err)

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
func (w *ReminderWorker) finalizeSkipped(ctx context.Context, r domain.Reminder) (err error) {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize transaction: %w", err)
	}
	defer w.rollbackOnReturn(ctx, tx, &err)

	txRepo := w.repo.WithTx(tx)
	if err := txRepo.MarkReminderSkipped(ctx, r.ID); err != nil {
		return fmt.Errorf("mark reminder skipped: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit finalize transaction: %w", err)
	}
	return nil
}

func (w *ReminderWorker) recoverFinalizeFailure(ctx context.Context, id uuid.UUID) (err error) {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finalize recovery transaction: %w", err)
	}
	defer w.rollbackOnReturn(ctx, tx, &err)

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

// rollbackClaimTx rolls back an open claim-loop transaction. The rollback
// error is logged, not returned: the triggering cause takes precedence.
func (w *ReminderWorker) rollbackClaimTx(ctx context.Context, tx transaction.Tx) {
	if err := tx.Rollback(ctx); err != nil {
		w.logger.WarnContext(ctx, "rollback reminder transaction", "error", sanitize.Error(err))
	}
}

// rollbackOnReturn is deferred by the finalize* transactions: the rollback
// is a pgx v5 no-op (ErrTxClosed) once work committed; any other failure is
// folded into the named return error without masking it.
func (w *ReminderWorker) rollbackOnReturn(ctx context.Context, tx transaction.Tx, err *error) {
	if rbErr := tx.Rollback(ctx); rbErr != nil && *err == nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
		*err = fmt.Errorf("rollback finalize transaction: %w", rbErr)
	}
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
			w.rollbackClaimTx(ctx, tx)
			return fmt.Errorf("list stale sending reminders: %w", err)
		}
		if len(reminders) == 0 {
			w.rollbackClaimTx(ctx, tx)
			break
		}

		r := reminders[0]
		if err := txRepo.ResetReminderSending(ctx, r.ID); err != nil {
			w.rollbackClaimTx(ctx, tx)
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
