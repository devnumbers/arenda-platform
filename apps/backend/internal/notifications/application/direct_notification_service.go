package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// Direct notifications (issue #253): messages the platform delivers to one
// user immediately, outside the reminder lifecycle. They carry no reminders
// row, no claim and no retry — the sender (a billing grace event subscriber)
// publishes once, this service dispatches both channels once. Both channels
// honour the user's per-channel preferences (ADR 0030); both are best-effort:
// a failing channel is logged and never fails the other channel or the
// publisher's transition.

// graceNotificationPath routes the push tap and the email button to the
// payment methods page — the one screen where the user fixes the failed
// charge. Pushes carry the relative path (the service worker resolves it
// against the app origin); emails need the absolute URL built from
// appBaseURL.
const graceNotificationPath = "/profile/tariff/payment-methods"

// DirectNotificationService delivers one-off notifications to a single user
// over push and email. Push requires a wired PushSender (nil keeps the service
// email-only, mirroring the reminder worker's local-dev behaviour).
type DirectNotificationService struct {
	prefs      ReminderRepository
	resolver   ContactResolver
	emailer    DirectEmailSender
	pushSender PushSender
	pushSubs   PushSubscriptionRepository
	appBaseURL string
	log        *slog.Logger
}

// NewDirectNotificationService creates a direct-notification delivery service.
// The appBaseURL parameter is the public web app URL the email buttons point at.
func NewDirectNotificationService(
	prefs ReminderRepository,
	resolver ContactResolver,
	emailer DirectEmailSender,
	pushSender PushSender,
	pushSubs PushSubscriptionRepository,
	appBaseURL string,
	logger *slog.Logger,
) *DirectNotificationService {
	if logger == nil {
		logger = slog.Default()
	}
	return &DirectNotificationService{
		prefs:      prefs,
		resolver:   resolver,
		emailer:    emailer,
		pushSender: pushSender,
		pushSubs:   pushSubs,
		appBaseURL: appBaseURL,
		log:        logger,
	}
}

// NotifyGraceEntered delivers the "renewal charge failed, subscription entered
// grace" notice (issue #253): update the payment method before the grace
// window ends.
func (s *DirectNotificationService) NotifyGraceEntered(ctx context.Context, userID uuid.UUID, graceUntil time.Time) error {
	title := "Не удалось списание за подписку"
	body := "Мы не смогли списать оплату за подписку. Привяжите другую карту, чтобы тариф не прервался."
	if !graceUntil.IsZero() {
		body = fmt.Sprintf("%s Льготный период действует до %s.", body, formatDate(graceUntil))
	}
	return s.notify(ctx, domain.EventSubscriptionGrace, userID, title, body)
}

// NotifyGraceExpiring delivers the "grace window is closing" reminder
// (issue #253): the last-mile push to update the payment method.
func (s *DirectNotificationService) NotifyGraceExpiring(ctx context.Context, userID uuid.UUID, graceUntil time.Time) error {
	title := "Подписка скоро истекает"
	body := "Льготный период подписки заканчивается — обновите способ оплаты, чтобы сохранить тариф."
	if !graceUntil.IsZero() {
		body = fmt.Sprintf("%s Он действует до %s.", body, formatDate(graceUntil))
	}
	return s.notify(ctx, domain.EventSubscriptionGrace, userID, title, body)
}

// notify dispatches one direct notification over both channels
// best-effort: every per-channel failure is logged and swallowed, so one
// broken channel never silences the other (and never propagates to the
// publisher's payment transition).
func (s *DirectNotificationService) notify(ctx context.Context, eventType domain.EventType, userID uuid.UUID, title, body string) error {
	logAttrs := []any{
		slog.String("recipient_id", userID.String()),
		slog.String("event_type", string(eventType)),
	}
	s.dispatchPush(ctx, eventType, userID, title, body, logAttrs)
	s.dispatchEmail(ctx, eventType, userID, title, body, logAttrs)
	return nil
}

// dispatchEmail delivers the email leg: the per-channel preference gates it
// (ADR 0030), a user without a verified contact is skipped, and a send
// failure is logged without affecting push.
func (s *DirectNotificationService) dispatchEmail(
	ctx context.Context,
	eventType domain.EventType,
	userID uuid.UUID,
	title, body string,
	logAttrs []any,
) {
	allowed, err := s.prefs.IsChannelAllowed(ctx, userID, eventType, domain.ChannelEmail)
	if err != nil {
		s.log.ErrorContext(ctx, "check email notification permission failed", append(logAttrs, slog.String("error", sanitize.Error(err)))...)
		return
	}
	if !allowed {
		return
	}
	contact, err := s.resolver.Resolve(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNoContact) {
			s.log.InfoContext(ctx, "direct notification skipped: no contact", logAttrs...)
			return
		}
		s.log.ErrorContext(ctx, "resolve contact failed", append(logAttrs, slog.String("error", sanitize.Error(err)))...)
		return
	}
	if err := s.emailer.SendDirect(ctx, contact.Email, title, "notification", map[string]any{
		"Subject":   title,
		"Title":     title,
		"Body":      body,
		"ActionURL": s.appBaseURL + graceNotificationPath,
	}); err != nil {
		s.log.ErrorContext(ctx, "send direct notification email failed", append(logAttrs, slog.String("error", sanitize.Error(err)))...)
	}
}

// dispatchPush delivers the Web Push leg to every subscription of the user.
// Dead subscriptions (404/410) are deleted; rate limiting stops the device
// fan-out for this notification; other failures are logged — none of them
// affect the email leg (mirrors the reminder worker's push semantics).
func (s *DirectNotificationService) dispatchPush(
	ctx context.Context,
	eventType domain.EventType,
	userID uuid.UUID,
	title, body string,
	logAttrs []any,
) {
	if s.pushSender == nil {
		return
	}
	allowed, err := s.prefs.IsChannelAllowed(ctx, userID, eventType, domain.ChannelPush)
	if err != nil {
		s.log.ErrorContext(ctx, "check push notification permission failed", append(logAttrs, slog.String("error", sanitize.Error(err)))...)
		return
	}
	if !allowed {
		return
	}
	subs, err := s.pushSubs.ListByUser(ctx, userID)
	if err != nil {
		s.log.ErrorContext(ctx, "list push subscriptions failed", append(logAttrs, slog.String("error", sanitize.Error(err)))...)
		return
	}
	if len(subs) == 0 {
		return
	}
	payload := PushPayload{
		Title:     title,
		Body:      body,
		Tag:       string(eventType),
		URL:       graceNotificationPath,
		EventType: eventType,
	}
pushSubs:
	for _, sub := range subs {
		sendErr := s.pushSender.Send(ctx, sub, payload)
		switch {
		case sendErr == nil:
		case errors.Is(sendErr, ErrSubscriptionGone):
			if delErr := s.pushSubs.Delete(ctx, sub.UserID, sub.Endpoint); delErr != nil {
				s.log.ErrorContext(ctx, "delete dead push subscription failed", append(logAttrs, slog.String("error", sanitize.Error(delErr)))...)
			} else {
				s.log.InfoContext(ctx, "push subscription removed (gone)", logAttrs...)
			}
		case errors.Is(sendErr, ErrRateLimited):
			s.log.WarnContext(ctx, "push rate limited, skipping remaining devices",
				append(logAttrs, slog.String("error", sanitize.Error(sendErr)))...)
			break pushSubs
		default:
			s.log.ErrorContext(ctx, "send push failed", append(logAttrs, slog.String("error", sanitize.Error(sendErr)))...)
		}
	}
}

// graceMonths are the Russian month names in the nominative case, used by the
// user-facing grace deadline ("21 августа").
var graceMonths = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

// formatDate renders a deadline as a Russian day-month string ("21 августа").
// The deadline is a UTC instant; a date-only rendering does not need the
// user's timezone — the grace window ends at a fixed instant and the copy
// names the day, not the hour.
func formatDate(t time.Time) string {
	// The platform's users are in Russia (RUB-only product, ADR 0036), so
	// Moscow's offset approximates their calendar day for a deadline label.
	msk := t.In(time.FixedZone("MSK", 3*60*60))
	return fmt.Sprintf("%d %s", msk.Day(), graceMonths[int(msk.Month())-1])
}
