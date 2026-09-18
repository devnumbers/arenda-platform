package notificationsjob

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/riverqueue/river"
)

// Compile-time checks that the workers satisfy River's worker contract for
// their args.
var (
	_ river.Worker[DeliverEmailArgs] = (*DeliverEmailWorker)(nil)
	_ river.Worker[DeliverPushArgs]  = (*DeliverPushWorker)(nil)
)

// notificationEmailTemplate is the shared notification email template
// (templates/email/notification.{html,txt}): one title, one body, one
// optional button built from the event's deep link.
const notificationEmailTemplate = "notification"

// providerLimitKey is the single key the email worker spends the global
// provider budget on: every notification email shares one bucket.
const providerLimitKey = "notifications_email_provider"

// providerSnooze is how long an email job waits when the provider budget is
// spent. Snoozing does not consume an attempt (river.JobSnooze), so a
// throttled job never reaches the discarded state through rate limiting.
const providerSnooze = time.Minute

// DeliverEmailWorker delivers a notification's email leg: it reloads the
// committed feed row, resolves the recipient's contact and sends through the
// platform mailer. Per-channel prefs checks belong here, at delivery time,
// not at enqueue time — the matrix lands with the settings API (#743).
type DeliverEmailWorker struct {
	river.WorkerDefaults[DeliverEmailArgs]
	feed       application.NotificationRepository
	resolver   application.ContactResolver
	emailer    application.TemplateEmailSender
	provider   providerLimiter
	metrics    *EmailMetrics
	appBaseURL string
	log        *slog.Logger
}

// providerLimiter is the global email-provider budget the worker spends
// before every send (*httpsupport.RateLimiter in the wiring).
type providerLimiter interface {
	Allow(key string) bool
}

// NewDeliverEmailWorker builds the email delivery worker.
func NewDeliverEmailWorker(
	feed application.NotificationRepository,
	resolver application.ContactResolver,
	emailer application.TemplateEmailSender,
	provider providerLimiter,
	metrics *EmailMetrics,
	appBaseURL string,
	log *slog.Logger,
) *DeliverEmailWorker {
	if log == nil {
		log = slog.Default()
	}
	return &DeliverEmailWorker{
		feed: feed, resolver: resolver, emailer: emailer,
		provider: provider, metrics: metrics,
		appBaseURL: appBaseURL, log: log,
	}
}

// Work delivers one notification email. Retry classification: a transient
// failure (database, SMTP) returns the error and follows River's backoff
// ladder; a vanished feed row cancels the job — there is nothing to retry.
func (w *DeliverEmailWorker) Work(ctx context.Context, job *river.Job[DeliverEmailArgs]) error {
	n, err := w.feed.GetByID(ctx, job.Args.NotificationID)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			w.metrics.RecordDispatch(ctx, outcomeCancelled)
			w.log.WarnContext(ctx, "notification email skipped: feed row gone",
				slog.String("notification_id", job.Args.NotificationID.String()))
			return river.JobCancel(fmt.Errorf("notification %s not found", job.Args.NotificationID))
		}
		return fmt.Errorf("load notification: %w", err)
	}

	contact, err := w.resolver.Resolve(ctx, n.UserID)
	if err != nil {
		if errors.Is(err, application.ErrNoContact) {
			// A recipient without a verified contact has no email leg — the
			// job succeeded by doing nothing.
			w.metrics.RecordDispatch(ctx, outcomeSkipped)
			w.log.InfoContext(ctx, "notification email skipped: no contact",
				slog.String("notification_id", n.ID.String()),
				slog.String("recipient_id", n.UserID.String()))
			return nil
		}
		return fmt.Errorf("resolve contact: %w", err)
	}

	if !w.provider.Allow(providerLimitKey) {
		// The global provider budget is spent: back off without burning an
		// attempt (research #735 §5).
		w.metrics.RecordDispatch(ctx, outcomeRateLimited)
		return river.JobSnooze(providerSnooze)
	}

	if err := w.emailer.SendTemplate(ctx, contact.Email, n.Title, notificationEmailTemplate, map[string]any{
		"Subject":   n.Title,
		"Title":     n.Title,
		"Body":      n.Body,
		"ActionURL": w.actionURL(n),
	}); err != nil {
		w.metrics.RecordDispatch(ctx, outcomeFailed)
		return fmt.Errorf("send notification email: %w", err)
	}

	w.metrics.RecordDispatch(ctx, outcomeSent)
	return nil
}

// actionURL builds the email button's absolute URL from the event's deep
// link. The link vocabulary belongs to the publishers (deeplink.go): the
// catalog publishers (#748–#752) extend the map per event type; an event
// without an entry renders the email without a button.
func (w *DeliverEmailWorker) actionURL(n domain.Notification) string {
	if path := deeplinkFor(n.EventType); path != "" {
		return w.appBaseURL + path
	}
	return ""
}
