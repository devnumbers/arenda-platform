package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// The grace-events module (issue #284): the single interface the payment flows
// — the webhook finalization of a customer-initiated charge and the renewal
// worker's merchant-initiated charge — publish the Grace events through
// (issue #253, ADR 0008). The semantics live here once: an event is captured
// inside the transaction that causes it and published strictly after that
// transaction commits, exactly once per grace window, best-effort — a
// publication failure is logged with the subscription context and never fails
// or rolls back the transition that caused the event.

// graceEvents carries the Grace events of one transaction: the work captures
// them through enterGrace and markReminded, and run publishes them strictly
// after the commit. One instance spans one transaction — reusing it across
// transactions would leak a captured event into the next publication.
type graceEvents struct {
	publisher EventPublisher
	log       *slog.Logger
	// entered is the GraceEntered event of the window this transaction opened;
	// nil when no window was entered, so the event fires exactly once per
	// grace window.
	entered *GraceEntered
	// expiring is the GraceExpiring reminder this transaction marked
	// dispatched; nil when no window was reminded.
	expiring *GraceExpiring
}

// newGraceEvents creates the module for one transaction. A nil publisher keeps
// the pre-#253 behaviour — the events are captured but never dispatched; a nil
// logger defaults to the standard one.
func newGraceEvents(publisher EventPublisher, log *slog.Logger) *graceEvents {
	if log == nil {
		log = slog.Default()
	}
	return &graceEvents{publisher: publisher, log: log}
}

// run executes work in the caller's transaction runner (the billing runInTx
// shape) and publishes the captured Grace events strictly after it commits.
// A rolled-back transaction publishes nothing — the publication is
// unreachable on error — and a publication failure is swallowed, so run
// returns the transaction's error and nothing else.
func (g *graceEvents) run(
	ctx context.Context,
	runTx func(context.Context, func(*txStores) error) error,
	work func(*txStores) error,
) error {
	if err := runTx(ctx, work); err != nil {
		return err
	}
	g.publishAfterCommit(ctx)
	return nil
}

// enterGrace moves the subscription into its grace window inside the caller's
// transaction — the transition-log entry included — and captures the
// GraceEntered event for the post-commit publication. A subscription already
// inside an open window is left untouched (an open window is never
// re-extended) and captures nothing: one event per grace window. Every path
// of the failed renewal charge of ADR 0008 arrives here — the webhook
// finalization, the worker's failed charge and the no-chargeable-method
// planning exit.
func (g *graceEvents) enterGrace(ctx context.Context, stores *txStores, sub domain.Subscription, now time.Time, grace time.Duration) error {
	if sub.Status == domain.SubscriptionStatusGrace && sub.IsInGrace(now) {
		return nil
	}
	if _, err := stores.applyTransition(ctx, &sub,
		func(s *domain.Subscription) error { s.EnterGrace(now, grace); return nil },
		transitionSpec{
			reason:    domain.TransitionReasonGraceEntered,
			initiator: domain.InitiatorSystem,
		},
	); err != nil {
		return err
	}
	g.entered = &GraceEntered{
		UserID:         sub.UserID,
		SubscriptionID: sub.ID,
		GraceUntil:     graceUntilOf(sub),
		At:             now,
	}
	return nil
}

// remindWindow marks the grace-expiry reminder due now inside the caller's
// transaction and captures the GraceExpiring event for the post-commit
// publication. The window is re-checked under the caller's subscription lock
// (the state the listing saw may be gone), and the persisted mark makes the
// reminder once per window: a window already reminded — or not due yet, ended,
// or belonging to a subscription no longer in grace — captures nothing.
func (g *graceEvents) remindWindow(ctx context.Context, stores *txStores, sub domain.Subscription, now time.Time, lead time.Duration) error {
	if !subscriptionInGraceReminderWindow(sub, now, lead) {
		return nil
	}
	sub.MarkGraceReminded(now)
	if err := stores.subscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("mark grace window reminded: %w", err)
	}
	g.expiring = &GraceExpiring{
		UserID:         sub.UserID,
		SubscriptionID: sub.ID,
		GraceUntil:     graceUntilOf(sub),
		At:             now,
	}
	return nil
}

// subscriptionInGraceReminderWindow reports whether the subscription is in
// grace, its window has not ended yet, the reminder lead time has arrived and
// the window was not reminded yet (issue #253). lead is the reminder lead
// duration: the window is [valid_until - lead, valid_until).
func subscriptionInGraceReminderWindow(s domain.Subscription, now time.Time, lead time.Duration) bool {
	return s.Status == domain.SubscriptionStatusGrace &&
		s.ValidUntil != nil &&
		s.ValidUntil.After(now) &&
		!s.ValidUntil.After(now.Add(lead)) &&
		s.GraceRemindedAt == nil
}

// graceUntilOf returns the subscription's validity — the grace window's end
// while the subscription is in grace; the zero time when none is set.
func graceUntilOf(sub domain.Subscription) time.Time {
	if sub.ValidUntil != nil {
		return *sub.ValidUntil
	}
	return time.Time{}
}

// publishAfterCommit dispatches the captured events. It runs strictly after
// the causing transaction committed and swallows every publication error with
// a log line naming the subscription: a broken publisher must never fail or
// roll back a committed transition.
func (g *graceEvents) publishAfterCommit(ctx context.Context) {
	if g.entered != nil && g.publisher != nil {
		event := *g.entered
		if err := g.publisher.PublishGraceEntered(ctx, event); err != nil {
			g.log.ErrorContext(ctx, "publish grace-entered event failed",
				slog.String("subscription_id", event.SubscriptionID.String()),
				slog.String("user_id", event.UserID.String()),
				slog.String("error", sanitize.Error(err)))
		}
	}
	if g.expiring != nil && g.publisher != nil {
		event := *g.expiring
		if err := g.publisher.PublishGraceExpiring(ctx, event); err != nil {
			g.log.ErrorContext(ctx, "publish grace-expiring event failed",
				slog.String("subscription_id", event.SubscriptionID.String()),
				slog.String("user_id", event.UserID.String()),
				slog.String("error", sanitize.Error(err)))
		}
	}
}
