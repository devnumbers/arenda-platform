package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// GracePublisher is the billing grace events' publisher (issue #253) on the
// delivery pipeline (карта #734, #741): each grace event becomes a feed row
// for the owner (the Тариф recipient slot) with both channel deliveries —
// email and push — enqueued through the River queue (ADR 0057). The former
// direct channel (DirectNotificationService) is gone: same copy, same
// payment-methods destination, but the notification is now stored, retried
// and observable per channel. The per-category matrix (#743) never gates
// these rows: the Тариф category is a service category — always on, outside
// the settings screen (ADR 0056).
//
// The call stays on the grace-events canon: billing captures the event
// inside its transaction and publishes strictly after the commit, best-effort
// — a Publish error is returned to the caller (billing's post-commit wrapper
// logs and swallows it) and never fails the transition.
type GracePublisher struct {
	pipeline *Publisher
}

// NewGracePublisher creates the grace events' publisher over the pipeline
// creation service.
func NewGracePublisher(pipeline *Publisher) *GracePublisher {
	return &GracePublisher{pipeline: pipeline}
}

// NotifyGraceEntered publishes the "renewal charge failed, subscription
// entered grace" notification (issue #253): update the payment method before
// the grace window ends.
func (g *GracePublisher) NotifyGraceEntered(ctx context.Context, userID, subscriptionID uuid.UUID, graceUntil time.Time) error {
	title := "Не удалось списание за подписку"
	body := "Мы не смогли списать оплату за подписку. Привяжите другую карту, чтобы тариф не прервался."
	if !graceUntil.IsZero() {
		body = fmt.Sprintf("%s Льготный период действует до %s.", body, formatGraceDeadline(graceUntil))
	}
	return g.publish(ctx, domain.EventSubscriptionGraceEntered, userID, subscriptionID, graceUntil, title, body)
}

// NotifyGraceExpiring publishes the "grace window is closing" reminder
// (issue #253): the last-mile push to update the payment method.
func (g *GracePublisher) NotifyGraceExpiring(ctx context.Context, userID, subscriptionID uuid.UUID, graceUntil time.Time) error {
	title := "Подписка скоро истекает"
	body := "Льготный период подписки заканчивается — обновите способ оплаты, чтобы сохранить тариф."
	if !graceUntil.IsZero() {
		body = fmt.Sprintf("%s Он действует до %s.", body, formatGraceDeadline(graceUntil))
	}
	return g.publish(ctx, domain.EventSubscriptionGraceExpiring, userID, subscriptionID, graceUntil, title, body)
}

// publish fans the grace event out to its single recipient. The dedup key
// carries the window identity — (event type, subscription, until date, the
// date taken in UTC so the key does not depend on the instant's location) —
// so a repeated publication of the same window creates nothing, while a new
// grace window of the same subscription notifies again. A zero graceUntil
// (a window without a known end) drops the date part it does not have.
func (g *GracePublisher) publish(
	ctx context.Context,
	eventType domain.EventType,
	userID, subscriptionID uuid.UUID,
	graceUntil time.Time,
	title, body string,
) error {
	key := string(eventType) + ":" + subscriptionID.String()
	if !graceUntil.IsZero() {
		key += ":" + graceUntil.UTC().Format("2006-01-02")
	}
	return g.pipeline.Publish(ctx, Publication{
		EventType:  eventType,
		DedupKey:   domain.DedupKey(key),
		Title:      title,
		Body:       body,
		Recipients: []uuid.UUID{userID},
	})
}

// graceMonths are the Russian month names in the nominative case, used by the
// user-facing grace deadline ("21 августа").
var graceMonths = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

// formatGraceDeadline renders a grace deadline as a Russian day-month string
// ("21 августа"). The deadline is a UTC instant; a date-only rendering does
// not need the user's timezone — the grace window ends at a fixed instant and
// the copy names the day, not the hour.
func formatGraceDeadline(t time.Time) string {
	// The platform's users are in Russia (RUB-only product, ADR 0036), so
	// Moscow's offset approximates their calendar day for a deadline label.
	msk := t.In(time.FixedZone("MSK", 3*60*60))
	return formatDayMonth(msk)
}

// formatDayMonth renders a date as a Russian day-month string ("20
// сентября") — the scan publishers' shared body rendering: the copy names
// the day, not the year (payments #749, tasks #750).
func formatDayMonth(t time.Time) string {
	return fmt.Sprintf("%d %s", t.Day(), graceMonths[int(t.Month())-1])
}
