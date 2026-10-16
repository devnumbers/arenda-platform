// Package events publishes identity's outbound events (UserRegistered) through the platform event dispatcher.
package events

import (
	"context"

	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	platformevents "github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
)

// Publisher implements identityapp.EventPublisher on top of the generic
// platform event dispatcher.
type Publisher struct {
	dispatcher platformevents.Dispatcher
	eventType  platformevents.EventType
}

// NewPublisher creates a Publisher that publishes identity events through d.
func NewPublisher(dispatcher platformevents.Dispatcher) *Publisher {
	return &Publisher{
		dispatcher: dispatcher,
		eventType:  platformevents.EventType("user_registered"),
	}
}

var _ identityapp.EventPublisher = (*Publisher)(nil)

// PublishUserRegistered publishes the UserRegistered event.
func (p *Publisher) PublishUserRegistered(ctx context.Context, event identityapp.UserRegistered) error {
	return p.dispatcher.Publish(ctx, p.eventType, event)
}
