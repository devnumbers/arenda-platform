package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// TariffView is the display snapshot of the tariff a billing event names:
// the slug the display name resolves from (TariffDisplayName's vocabulary).
type TariffView struct {
	Name string
}

// TariffEventViewSource resolves the tariff snapshot of the billing tariff
// events (#752). Consumer-declared (CODING_STANDARDS), answered over the
// owning tariffs table.
type TariffEventViewSource interface {
	TariffView(ctx context.Context, tariffID uuid.UUID) (TariffView, error)
}

// tariffDisplayNames is the user-facing plan names of the catalog copy: the
// frontend's own vocabulary (get-tariff-label). An unknown slug renders as
// itself — an admin-created plan is never blank in the copy.
var tariffDisplayNames = map[string]string{
	"basic":    "Базовый",
	"pro":      "Про",
	"business": "Бизнес",
}

// tariffDisplayName renders the plan name the copy quotes: the shared label
// vocabulary where the plan has one, the slug where it has not (per-context
// copy, the канон #748).
func tariffDisplayName(slug string) string {
	if name, ok := tariffDisplayNames[slug]; ok {
		return name
	}
	return slug
}

// TariffPublisher is the billing tariff events' publisher (#752) on the
// delivery pipeline (карта #734, #740): a succeeded subscription payment
// (№13 «Оплата прошла») and a tariff change (№14 «Тариф изменён» — the
// upgrade applied right after the payment, the downgrade scheduled for the
// period's end) arrive here and become the Тариф catalog rows (решение #737)
// for their single recipient — the owner. The Тариф category is a service
// one: always on, outside the settings screen (ADR 0056), the per-category
// matrix never gates these rows.
//
// The billing context captures the events inside its transaction and the
// composition root dispatches them strictly after the commit — the
// grace-events canon (#741); a publish failure is logged there and never
// fails the applied payment or the scheduled downgrade. The tariff name
// snapshot is resolved at publication: the row is the moment's snapshot
// (канон #751), a plan rename never rewrites stored rows.
type TariffPublisher struct {
	pipeline *Publisher
	views    TariffEventViewSource
}

// NewTariffPublisher builds the tariff events' publisher over the pipeline
// creation service and the view source.
func NewTariffPublisher(pipeline *Publisher, views TariffEventViewSource) *TariffPublisher {
	return &TariffPublisher{pipeline: pipeline, views: views}
}

// NotifyPaymentSucceeded publishes the «Оплата прошла» row (решение #737,
// тип №13) for an applied subscription payment — every path that applies a
// success lands on it, the renewal and the upgrade alike. ActiveUntil is the
// subscription's validity the payment just set; the zero instant (no known
// validity) drops the tail and the payload's date, like the grace deadline
// rendering does.
func (p *TariffPublisher) NotifyPaymentSucceeded(
	ctx context.Context,
	userID, paymentID, tariffID uuid.UUID,
	amountKopecks int64,
	period string,
	activeUntil time.Time,
) error {
	view, err := p.views.TariffView(ctx, tariffID)
	if err != nil {
		return fmt.Errorf("resolve tariff view %s: %w", tariffID, err)
	}
	name := tariffDisplayName(view.Name)
	body := "Оплата тарифа «" + name + "» на " + formatAmountKopecks(amountKopecks) + " прошла успешно."
	payload := domain.TariffRef{Slug: view.Name, Period: period, AmountKopecks: amountKopecks}
	if !activeUntil.IsZero() {
		body += " Подписка активна до " + formatGraceDeadline(activeUntil)
		until := activeUntil
		payload.ActiveUntil = &until
	}
	return p.pipeline.Publish(ctx, Publication{
		EventType:    domain.EventSubscriptionPaymentSucceeded,
		DedupKey:     domain.DedupKey("subscription_payment_succeeded:" + paymentID.String()),
		Title:        "Оплата прошла",
		Body:         body,
		ContextLabel: name,
		Payload:      domain.Payload{Tariff: &payload},
		Recipients:   []uuid.UUID{userID},
	})
}

// NotifyPlanUpgraded publishes the upgrade leg of «Тариф изменён» (решение
// #737, тип №14): the payment just activated the new plan — «Тариф „{тариф}“
// активирован». ActiveUntil is the validity the payment set.
func (p *TariffPublisher) NotifyPlanUpgraded(
	ctx context.Context,
	userID, transitionID, tariffID uuid.UUID,
	period string,
	activeUntil time.Time,
) error {
	view, err := p.views.TariffView(ctx, tariffID)
	if err != nil {
		return fmt.Errorf("resolve tariff view %s: %w", tariffID, err)
	}
	name := tariffDisplayName(view.Name)
	payload := domain.TariffRef{Slug: view.Name, Period: period}
	if !activeUntil.IsZero() {
		until := activeUntil
		payload.ActiveUntil = &until
	}
	return p.pipeline.Publish(ctx, Publication{
		EventType:    domain.EventSubscriptionPlanChanged,
		DedupKey:     domain.DedupKey("subscription_plan_changed:" + transitionID.String()),
		Title:        "Тариф изменён",
		Body:         fmt.Sprintf("Тариф „%s“ активирован", name),
		ContextLabel: name,
		Payload:      domain.Payload{Tariff: &payload},
		Recipients:   []uuid.UUID{userID},
	})
}

// NotifyPlanDowngradeScheduled publishes the downgrade leg of «Тариф
// изменён» (решение #737, тип №14): the change is scheduled now and lands
// when the paid period ends — «С {дата} тариф сменится на „{тариф}“».
// EffectiveAt is that period end; the payload carries no validity — the
// target is not active yet.
func (p *TariffPublisher) NotifyPlanDowngradeScheduled(
	ctx context.Context,
	userID, transitionID, tariffID uuid.UUID,
	period string,
	effectiveAt time.Time,
) error {
	view, err := p.views.TariffView(ctx, tariffID)
	if err != nil {
		return fmt.Errorf("resolve tariff view %s: %w", tariffID, err)
	}
	name := tariffDisplayName(view.Name)
	return p.pipeline.Publish(ctx, Publication{
		EventType:    domain.EventSubscriptionPlanChanged,
		DedupKey:     domain.DedupKey("subscription_plan_changed:" + transitionID.String()),
		Title:        "Тариф изменён",
		Body:         fmt.Sprintf("С %s тариф сменится на „%s“", formatGraceDeadline(effectiveAt), name),
		ContextLabel: name,
		Payload:      domain.Payload{Tariff: &domain.TariffRef{Slug: view.Name, Period: period}},
		Recipients:   []uuid.UUID{userID},
	})
}
