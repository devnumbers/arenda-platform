package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// Grace lifecycle events (issue #253, ADR 0008). Billing publishes them at the
// subscription transitions; the notifications context delivers them to the
// user over push and email with the per-channel preferences of ADR 0030.
// The publication semantics — capture inside the transaction, dispatch
// strictly after the commit, once per window, best-effort — are owned by the
// grace-events module (grace_events.go, issue #284).

// GraceEntered is emitted once when a subscription moves into its grace window
// (the failed renewal charge of ADR 0008): the immediate "payment failed, fix
// the card" notice.
type GraceEntered struct {
	UserID uuid.UUID
	// SubscriptionID identifies the subscription that entered grace.
	SubscriptionID uuid.UUID
	// GraceUntil is the end of the grace window (the subscription's ValidUntil
	// while it is in grace).
	GraceUntil time.Time
	At         time.Time
}

// GraceExpiring is emitted while a grace window approaches its end: the
// last-mile reminder to update the payment method before the subscription
// falls to the basic tariff.
type GraceExpiring struct {
	UserID         uuid.UUID
	SubscriptionID uuid.UUID
	GraceUntil     time.Time
	At             time.Time
}

// The tariff events of the notifications catalog (карта #734, решение #737
// №13–№14, #752). They ride the same publication seam as the grace events:
// captured inside the causing transaction, dispatched strictly after the
// commit, best-effort. The events are clean ids and figures — the receiving
// context resolves the display snapshot at publication.

// PaymentSucceeded is emitted when a succeeded subscription payment is
// applied to the subscription (№13 «Оплата прошла»): every path that applies
// a success — the webhook flow, the reconciliation and renewal workers —
// lands on the shared application seam, so the event fires once wherever the
// success came from. The payment is the event's identity — a duplicate
// delivery applies nothing and emits nothing.
type PaymentSucceeded struct {
	UserID    uuid.UUID
	PaymentID uuid.UUID
	// TariffID is the plan the payment bought; AmountKopecks is what it
	// charged (BIGINT kopecks, ADR 0008); Period is the bought period.
	TariffID      uuid.UUID
	AmountKopecks int64
	Period        domain.SubscriptionPeriod
	// ActiveUntil is the validity the application just set (the «активна
	// до» line of the copy).
	ActiveUntil time.Time
}

// PlanUpgraded is emitted when an applied payment switched the tariff to a
// better plan (№14 «Тариф изменён», upgrade leg): the new plan is active
// immediately — «Тариф „{тариф}“ активирован». The applied transition is the
// event's identity.
type PlanUpgraded struct {
	UserID       uuid.UUID
	TransitionID uuid.UUID
	// TariffID is the plan the payment activated; Period is its billing
	// period and ActiveUntil the validity the application just set.
	TariffID    uuid.UUID
	Period      domain.SubscriptionPeriod
	ActiveUntil time.Time
}

// PlanDowngradeScheduled is emitted when a user schedules a downgrade for
// the end of the paid period (№14 «Тариф изменён», downgrade leg — решение
// #249 flow): the change was assigned now and lands at EffectiveAt — «С
// {дата} тариф сменится на „{тариф}“». The scheduling transition is the
// event's identity; the application of the scheduled change at period end
// stays silent (the fact was already reported at assignment).
type PlanDowngradeScheduled struct {
	UserID       uuid.UUID
	TransitionID uuid.UUID
	// TariffID is the plan the downgrade lands on; Period is its billing
	// period and EffectiveAt the moment the change takes effect (the paid
	// period's end).
	TariffID    uuid.UUID
	Period      domain.SubscriptionPeriod
	EffectiveAt time.Time
}

// EventPublisher publishes the billing domain events. The initial
// implementation is an in-process dispatcher adapter; the application seam
// keeps publishing testable through a capture publisher (issue #244 testing
// decisions).
type EventPublisher interface {
	PublishGraceEntered(ctx context.Context, event GraceEntered) error
	PublishGraceExpiring(ctx context.Context, event GraceExpiring) error
	PublishPaymentSucceeded(ctx context.Context, event PaymentSucceeded) error
	PublishPlanUpgraded(ctx context.Context, event PlanUpgraded) error
	PublishPlanDowngradeScheduled(ctx context.Context, event PlanDowngradeScheduled) error
}
