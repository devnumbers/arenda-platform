package notificationsjob

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/riverqueue/river"
)

// pushRateSnoozeStep scales the rate-limited push job's snooze with the
// attempt number: the push service throttled the fan-out, so each retry
// waits a little longer. Snoozes do not consume attempts.
const pushRateSnoozeStep = 30 * time.Second

// DeliverPushWorker delivers a notification's Web Push leg: it reloads the
// committed feed row and fans the payload out over the recipient's current
// subscriptions. Subscription resolution happens at delivery time — devices
// subscribed after the enqueue still receive the push, dead ones are
// dropped; the per-device settings matrix lands with #743.
type DeliverPushWorker struct {
	river.WorkerDefaults[DeliverPushArgs]
	feed     application.NotificationRepository
	sender   application.PushSender
	pushSubs application.PushSubscriptionRepository
	log      *slog.Logger
}

// NewDeliverPushWorker builds the push delivery worker.
func NewDeliverPushWorker(
	feed application.NotificationRepository,
	sender application.PushSender,
	pushSubs application.PushSubscriptionRepository,
	log *slog.Logger,
) *DeliverPushWorker {
	if log == nil {
		log = slog.Default()
	}
	return &DeliverPushWorker{feed: feed, sender: sender, pushSubs: pushSubs, log: log}
}

// Work fans one notification out over the recipient's subscriptions.
// Per-device outcomes follow the best-effort side-channel contract
// (CONTEXT.md): a dead subscription (404/410) is deleted on the spot, other
// per-device failures are logged and never fail the job. A push-service rate
// limit pauses the whole fan-out — the job snoozes and re-runs it; devices
// already notified get the same payload again, which the service worker's
// tag (the notification id) collapses into a replacement.
func (w *DeliverPushWorker) Work(ctx context.Context, job *river.Job[DeliverPushArgs]) error {
	n, err := w.feed.GetByID(ctx, job.Args.NotificationID)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			w.log.WarnContext(ctx, "notification push skipped: feed row gone",
				slog.String("notification_id", job.Args.NotificationID.String()))
			return river.JobCancel(fmt.Errorf("notification %s not found", job.Args.NotificationID))
		}
		return fmt.Errorf("load notification: %w", err)
	}

	subs, err := w.pushSubs.ListByUser(ctx, n.UserID)
	if err != nil {
		return fmt.Errorf("list push subscriptions: %w", err)
	}
	if len(subs) == 0 {
		return nil
	}

	payload := application.PushPayload{
		Title:     n.Title,
		Body:      n.Body,
		Tag:       n.ID.String(),
		URL:       DeepLinkFor(n.EventType),
		EventType: n.EventType,
	}
	for _, sub := range subs {
		sendErr := w.sender.Send(ctx, sub, payload)
		switch {
		case sendErr == nil:
		case errors.Is(sendErr, application.ErrSubscriptionGone):
			if delErr := w.pushSubs.Delete(ctx, sub.UserID, sub.Endpoint); delErr != nil {
				w.log.ErrorContext(ctx, "delete dead push subscription failed",
					slog.String("recipient_id", n.UserID.String()),
					slog.String("error", sanitize.Error(delErr)))
			} else {
				w.log.InfoContext(ctx, "push subscription removed (gone)",
					slog.String("recipient_id", n.UserID.String()))
			}
		case errors.Is(sendErr, application.ErrRateLimited):
			w.log.WarnContext(ctx, "push service rate limited the fan-out, snoozing",
				slog.String("notification_id", n.ID.String()),
				slog.Int("attempt", job.Attempt),
				slog.String("error", sanitize.Error(sendErr)))
			return river.JobSnooze(time.Duration(job.Attempt) * pushRateSnoozeStep)
		default:
			w.log.ErrorContext(ctx, "send push failed",
				slog.String("notification_id", n.ID.String()),
				slog.String("endpoint", sub.Endpoint),
				slog.String("error", sanitize.Error(sendErr)))
		}
	}
	return nil
}
