package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// Grace lifecycle events (issue #253, ADR 0008). Billing publishes them at the
// subscription transitions; the notifications context delivers them to the
// user over push and email with the per-channel preferences of ADR 0030.
// Publication is best-effort: a failing publisher is logged and never rolls
// back or blocks the payment transition that caused the event.

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

// publishGraceEntered emits the grace-entered event best-effort: the error is
// logged with the subscription context and swallowed, so a broken publisher
// never fails the committed transition (issue #253). A nil publisher keeps the
// pre-#253 behaviour — the event is skipped silently.
func publishGraceEntered(ctx context.Context, publisher EventPublisher, log *slog.Logger, sub domain.Subscription, now time.Time) {
	if publisher == nil {
		return
	}
	graceUntil := time.Time{}
	if sub.ValidUntil != nil {
		graceUntil = *sub.ValidUntil
	}
	if err := publisher.PublishGraceEntered(ctx, GraceEntered{
		UserID:         sub.UserID,
		SubscriptionID: sub.ID,
		GraceUntil:     graceUntil,
		At:             now,
	}); err != nil {
		log.ErrorContext(ctx, "publish grace-entered event failed",
			slog.String("subscription_id", sub.ID.String()),
			slog.String("user_id", sub.UserID.String()),
			slog.String("error", sanitize.Error(err)))
	}
}

// publishGraceExpiring emits the grace-expiring reminder best-effort, with the
// same swallow-and-log contract as publishGraceEntered.
func publishGraceExpiring(ctx context.Context, publisher EventPublisher, log *slog.Logger, sub domain.Subscription, now time.Time) {
	if publisher == nil {
		return
	}
	graceUntil := time.Time{}
	if sub.ValidUntil != nil {
		graceUntil = *sub.ValidUntil
	}
	if err := publisher.PublishGraceExpiring(ctx, GraceExpiring{
		UserID:         sub.UserID,
		SubscriptionID: sub.ID,
		GraceUntil:     graceUntil,
		At:             now,
	}); err != nil {
		log.ErrorContext(ctx, "publish grace-expiring event failed",
			slog.String("subscription_id", sub.ID.String()),
			slog.String("user_id", sub.UserID.String()),
			slog.String("error", sanitize.Error(err)))
	}
}
