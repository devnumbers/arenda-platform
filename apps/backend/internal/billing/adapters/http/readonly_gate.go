package http

import (
	"context"
	"errors"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// MutationGate adapts the billing subscription service to the readonly-gate
// port declared by platform/httpsupport (ADR 0035 consumer-side interface): it
// resolves whether the user's subscription allows data mutations right now,
// keeping httpsupport free of billing domain rules. A user without a
// subscription yet is treated as mutable — the gate only blocks, never grants.
type MutationGate struct {
	subscriptions *billingapp.SubscriptionService
	clock         clock.Clock
}

// NewMutationGate creates the readonly-gate adapter over the subscription
// service.
func NewMutationGate(subscriptions *billingapp.SubscriptionService, clk clock.Clock) *MutationGate {
	if clk == nil {
		clk = clock.Real{}
	}
	return &MutationGate{subscriptions: subscriptions, clock: clk}
}

// CanMutateData reports whether the subscription allows data mutations at the
// current time (ADR 0008: grace keeps mutating within the grace window;
// cancelled keeps mutating until the paid period ends).
func (g *MutationGate) CanMutateData(ctx context.Context, userID uuid.UUID) (bool, error) {
	view, err := g.subscriptions.GetSubscription(ctx, userID)
	if err != nil {
		if errors.Is(err, billingapp.ErrSubscriptionNotFound) {
			return true, nil
		}
		return false, err
	}
	return view.Subscription.CanMutateData(g.clock.Now().UTC()), nil
}
