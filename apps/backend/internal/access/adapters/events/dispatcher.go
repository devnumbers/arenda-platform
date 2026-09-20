// Package events holds the access module's outbound event adapters: the
// publisher that carries the access lifecycle events (карта #734, #751) from
// the access application to the shared in-process dispatcher. The composition
// root subscribes the notifications context's access publisher to the same
// event types, so access never imports notifications.
package events

import (
	"context"

	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	platformevents "github.com/nambers/arenda-planform/apps/backend/internal/platform/events"
)

// The access lifecycle events' dispatcher names: stable, coarse — the same
// contract the grace events' names follow.
const (
	invitationActivatedType = "access_invitation_activated"
	membershipSuspendedType = "access_membership_suspended"
	membershipResumedType   = "access_membership_resumed"
	membershipRevokedType   = "access_membership_revoked"
	memberLeftType          = "access_member_left"
)

// Publisher implements accessapp.AccessEventPublisher on top of the generic
// platform event dispatcher.
type Publisher struct {
	dispatcher platformevents.Dispatcher
}

// NewPublisher creates a Publisher that publishes the access lifecycle
// events through d.
func NewPublisher(dispatcher platformevents.Dispatcher) *Publisher {
	return &Publisher{dispatcher: dispatcher}
}

var _ accessapp.AccessEventPublisher = (*Publisher)(nil)

// PublishInvitationActivated publishes the InvitationActivated event.
func (p *Publisher) PublishInvitationActivated(ctx context.Context, event accessapp.InvitationActivated) error {
	return p.dispatcher.Publish(ctx, invitationActivatedType, event)
}

// PublishMembershipSuspended publishes the MembershipSuspended event.
func (p *Publisher) PublishMembershipSuspended(ctx context.Context, event accessapp.MembershipSuspended) error {
	return p.dispatcher.Publish(ctx, membershipSuspendedType, event)
}

// PublishMembershipResumed publishes the MembershipResumed event.
func (p *Publisher) PublishMembershipResumed(ctx context.Context, event accessapp.MembershipResumed) error {
	return p.dispatcher.Publish(ctx, membershipResumedType, event)
}

// PublishMembershipRevoked publishes the MembershipRevoked event.
func (p *Publisher) PublishMembershipRevoked(ctx context.Context, event accessapp.MembershipRevoked) error {
	return p.dispatcher.Publish(ctx, membershipRevokedType, event)
}

// PublishMemberLeft publishes the MemberLeft event.
func (p *Publisher) PublishMemberLeft(ctx context.Context, event accessapp.MemberLeft) error {
	return p.dispatcher.Publish(ctx, memberLeftType, event)
}
