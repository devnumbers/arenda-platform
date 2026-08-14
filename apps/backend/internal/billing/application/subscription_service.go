package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// SubscriptionService serves the user's view of their own subscription.
type SubscriptionService struct {
	txStoreFactory
}

// NewSubscriptionService creates a subscription service over the shared
// factory.
func NewSubscriptionService(factory txStoreFactory) *SubscriptionService {
	return &SubscriptionService{txStoreFactory: factory}
}

// GetSubscription assembles the user's subscription view: the subscription
// aggregate with its tariff and any scheduled (pending) tariff resolved. The
// subscription lifecycle mutations (cancel, auto-renew, downgrade scheduling)
// land with issue #249; this core slice only reads.
func (s *SubscriptionService) GetSubscription(ctx context.Context, userID uuid.UUID) (SubscriptionView, error) {
	sub, err := s.subscriptions.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SubscriptionView{}, ErrSubscriptionNotFound
		}
		return SubscriptionView{}, fmt.Errorf("get subscription: %w", err)
	}

	tariff, err := s.tariffs.GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SubscriptionView{}, fmt.Errorf("subscription %s references missing tariff %s: %w", sub.ID, sub.TariffID, ErrTariffNotFound)
		}
		return SubscriptionView{}, fmt.Errorf("get tariff: %w", err)
	}

	view := SubscriptionView{
		Subscription: sub,
		Tariff:       tariff,
	}
	if sub.PendingTariffID != nil {
		pending, err := s.tariffs.GetByID(ctx, *sub.PendingTariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return SubscriptionView{}, fmt.Errorf("subscription %s references missing pending tariff %s: %w", sub.ID, *sub.PendingTariffID, ErrTariffNotFound)
			}
			return SubscriptionView{}, fmt.Errorf("get pending tariff: %w", err)
		}
		view.PendingTariff = &pending
	}
	return view, nil
}
