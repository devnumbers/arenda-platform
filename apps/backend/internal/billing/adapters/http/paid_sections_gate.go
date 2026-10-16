package http

import (
	"context"
	"errors"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// PaidSectionsGate adapts the billing subscription service to the
// paid-sections tariff gate port declared by platform/httpsupport (ADR 0035
// consumer-side interface): it resolves whether the user's current tariff
// covers the platform's paid sections, keeping httpsupport free of billing
// domain rules. The tariff name alone decides (карта #997): grace and
// cancelled keep their paid tariff until the downgrade is actually applied,
// so status needs no clock here. A user without a subscription yet counts as
// basic — the gate only blocks, never grants.
type PaidSectionsGate struct {
	subscriptions *billingapp.SubscriptionService
}

// NewPaidSectionsGate creates the paid-sections gate adapter over the
// subscription service.
func NewPaidSectionsGate(subscriptions *billingapp.SubscriptionService) *PaidSectionsGate {
	return &PaidSectionsGate{subscriptions: subscriptions}
}

var _ httpsupport.PaidSectionsGate = (*PaidSectionsGate)(nil)

// CanUsePaidFeatures reports whether the user's current tariff is a paid one.
// A missing subscription is the free basic state, not an error; a store
// failure propagates so the caller can fail closed with a 500.
func (g *PaidSectionsGate) CanUsePaidFeatures(ctx context.Context, userID uuid.UUID) (bool, error) {
	view, err := g.subscriptions.GetSubscription(ctx, userID)
	if err != nil {
		if errors.Is(err, billingapp.ErrSubscriptionNotFound) {
			return false, nil
		}
		return false, err
	}
	return view.Tariff.IsPaid(), nil
}
