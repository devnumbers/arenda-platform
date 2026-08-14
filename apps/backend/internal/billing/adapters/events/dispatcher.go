// Package events holds the billing module's outbound event adapters: the
// publisher that carries the grace lifecycle events (issue #253) from the
// billing application to the shared in-process dispatcher. The composition
// root subscribes the notifications context's delivery service to the same
// event types, so billing never imports notifications.
package events

import (
	"context"

	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	platformevents "github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
)

// Publisher implements billingapp.EventPublisher on top of the generic
// platform event dispatcher.
type Publisher struct {
	dispatcher   platformevents.Dispatcher
	enteredType  platformevents.EventType
	expiringType platformevents.EventType
}

// NewPublisher creates a Publisher that publishes the billing grace events
// through d.
func NewPublisher(dispatcher platformevents.Dispatcher) *Publisher {
	return &Publisher{
		dispatcher:   dispatcher,
		enteredType:  platformevents.EventType("subscription_grace_entered"),
		expiringType: platformevents.EventType("subscription_grace_expiring"),
	}
}

var _ billingapp.EventPublisher = (*Publisher)(nil)

// PublishGraceEntered publishes the GraceEntered event.
func (p *Publisher) PublishGraceEntered(ctx context.Context, event billingapp.GraceEntered) error {
	return p.dispatcher.Publish(ctx, p.enteredType, event)
}

// PublishGraceExpiring publishes the GraceExpiring event.
func (p *Publisher) PublishGraceExpiring(ctx context.Context, event billingapp.GraceExpiring) error {
	return p.dispatcher.Publish(ctx, p.expiringType, event)
}
