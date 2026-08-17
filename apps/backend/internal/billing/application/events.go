package application

import (
	"context"
	"time"

	"github.com/google/uuid"
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

// EventPublisher publishes the billing domain events. The initial
// implementation is an in-process dispatcher adapter; the application seam
// keeps publishing testable through a capture publisher (issue #244 testing
// decisions).
type EventPublisher interface {
	PublishGraceEntered(ctx context.Context, event GraceEntered) error
	PublishGraceExpiring(ctx context.Context, event GraceExpiring) error
}
