// Package events holds the billing module's outbound event adapters: the
// publisher that carries the grace lifecycle events (issue #253) and the
// tariff catalog events (#752, решение #737 №13–№14) from the billing
// application to the shared in-process dispatcher. The composition root
// subscribes the notifications context's delivery service to the same event
// types, so billing never imports notifications.
package events

import (
	"context"

	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	platformevents "github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
)

// The billing events' dispatcher names, exported so the composition root
// subscribes by these constants — an op-syllable mistake in a literal on
// either side would be a silently dead subscription otherwise (the #751
// review's finding).
var (
	// EventGraceEntered carries billingapp.GraceEntered.
	EventGraceEntered = platformevents.EventType("subscription_grace_entered")
	// EventGraceExpiring carries billingapp.GraceExpiring.
	EventGraceExpiring = platformevents.EventType("subscription_grace_expiring")
	// EventPaymentSucceeded carries billingapp.PaymentSucceeded (№13).
	EventPaymentSucceeded = platformevents.EventType("subscription_payment_succeeded")
	// EventPlanUpgraded carries billingapp.PlanUpgraded (№14, upgrade leg).
	EventPlanUpgraded = platformevents.EventType("subscription_plan_upgraded")
	// EventPlanDowngradeScheduled carries billingapp.PlanDowngradeScheduled
	// (№14, downgrade leg).
	EventPlanDowngradeScheduled = platformevents.EventType("subscription_plan_downgrade_scheduled")
)

// Publisher implements billingapp.EventPublisher on top of the generic
// platform event dispatcher.
type Publisher struct {
	dispatcher platformevents.Dispatcher
}

// NewPublisher creates a Publisher that publishes the billing events through
// d.
func NewPublisher(dispatcher platformevents.Dispatcher) *Publisher {
	return &Publisher{dispatcher: dispatcher}
}

var _ billingapp.EventPublisher = (*Publisher)(nil)

// PublishGraceEntered publishes the GraceEntered event.
func (p *Publisher) PublishGraceEntered(ctx context.Context, event billingapp.GraceEntered) error {
	return p.dispatcher.Publish(ctx, EventGraceEntered, event)
}

// PublishGraceExpiring publishes the GraceExpiring event.
func (p *Publisher) PublishGraceExpiring(ctx context.Context, event billingapp.GraceExpiring) error {
	return p.dispatcher.Publish(ctx, EventGraceExpiring, event)
}

// PublishPaymentSucceeded publishes the PaymentSucceeded event.
func (p *Publisher) PublishPaymentSucceeded(ctx context.Context, event billingapp.PaymentSucceeded) error {
	return p.dispatcher.Publish(ctx, EventPaymentSucceeded, event)
}

// PublishPlanUpgraded publishes the PlanUpgraded event.
func (p *Publisher) PublishPlanUpgraded(ctx context.Context, event billingapp.PlanUpgraded) error {
	return p.dispatcher.Publish(ctx, EventPlanUpgraded, event)
}

// PublishPlanDowngradeScheduled publishes the PlanDowngradeScheduled event.
func (p *Publisher) PublishPlanDowngradeScheduled(ctx context.Context, event billingapp.PlanDowngradeScheduled) error {
	return p.dispatcher.Publish(ctx, EventPlanDowngradeScheduled, event)
}
