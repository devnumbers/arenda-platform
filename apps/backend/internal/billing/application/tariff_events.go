package application

import (
	"context"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// The tariff-events module (карта #734, #752): the carrier the payment and
// subscription flows capture the tariff events (решение #737 №13–№14)
// through, shaped after the grace-events module (issue #284) — an event is
// captured inside the transaction that causes it and published strictly
// after that transaction commits, best-effort: a publication failure is
// logged and never fails or rolls back the applied payment or the scheduled
// downgrade.
//
// The capture points are the seams that own the facts:
//   - applySucceededPayment (payment_service.go) — the shared application
//     seam every success lands on: PaymentSucceeded for the applied payment
//     itself, PlanUpgraded when the application switched the tariff to a
//     better plan (the direction is classified where both plans are in
//     hand);
//   - scheduleDeferredDowngrade (subscription_service.go) — the assignment
//     of a period-end downgrade: PlanDowngradeScheduled. The application of
//     the scheduled change at period end stays silent — the fact was
//     reported at assignment.

// tariffEvents carries the tariff events of one transaction: the flows
// capture them into its fields, and publishAfterCommit dispatches them
// strictly after the commit. One instance spans one transaction — reusing it
// across transactions would leak a captured event into the next publication.
type tariffEvents struct {
	publisher EventPublisher
	log       *slog.Logger
	// Succeeded is the PaymentSucceeded event of the payment this
	// transaction applied; nil when nothing was applied, so a duplicate
	// delivery (the idempotent no-op) emits nothing.
	succeeded *PaymentSucceeded
	// Upgraded is the PlanUpgraded event of an applied upgrade; nil when the
	// application was a renewal or the subscription kept its plan.
	upgraded *PlanUpgraded
	// DowngradeScheduled is the PlanDowngradeScheduled event of a downgrade
	// assignment; nil when the transaction scheduled none.
	downgradeScheduled *PlanDowngradeScheduled
}

// newTariffEvents creates the module for one transaction. A nil publisher
// keeps the pre-#752 behaviour — the events are captured but never
// dispatched; a nil logger defaults to the standard one.
func newTariffEvents(publisher EventPublisher, log *slog.Logger) *tariffEvents {
	if log == nil {
		log = slog.Default()
	}
	return &tariffEvents{publisher: publisher, log: log}
}

// publishAfterCommit dispatches the captured events. It runs strictly after
// the causing transaction committed and swallows every publication error
// with a log line: a broken publisher must never fail or roll back a
// committed change.
func (t *tariffEvents) publishAfterCommit(ctx context.Context) {
	if t.publisher == nil {
		return
	}
	if t.succeeded != nil {
		event := *t.succeeded
		if err := t.publisher.PublishPaymentSucceeded(ctx, event); err != nil {
			t.log.ErrorContext(ctx, "publish payment-succeeded event failed",
				slog.String("payment_id", event.PaymentID.String()),
				slog.String("user_id", event.UserID.String()),
				slog.String("error", sanitize.Error(err)))
		}
	}
	if t.upgraded != nil {
		event := *t.upgraded
		if err := t.publisher.PublishPlanUpgraded(ctx, event); err != nil {
			t.log.ErrorContext(ctx, "publish plan-upgraded event failed",
				slog.String("transition_id", event.TransitionID.String()),
				slog.String("user_id", event.UserID.String()),
				slog.String("error", sanitize.Error(err)))
		}
	}
	if t.downgradeScheduled != nil {
		event := *t.downgradeScheduled
		if err := t.publisher.PublishPlanDowngradeScheduled(ctx, event); err != nil {
			t.log.ErrorContext(ctx, "publish plan-downgrade-scheduled event failed",
				slog.String("transition_id", event.TransitionID.String()),
				slog.String("user_id", event.UserID.String()),
				slog.String("error", sanitize.Error(err)))
		}
	}
}
