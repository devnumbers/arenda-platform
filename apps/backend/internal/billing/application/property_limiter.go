package application

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// SubscriptionPropertyLimiter computes the active property limit from the
// user's subscription and tariff. It implements PropertyLimiter.
type SubscriptionPropertyLimiter struct {
	subscriptions SubscriptionRepository
	tariffs       TariffRepository
}

// NewSubscriptionPropertyLimiter creates a new limiter.
func NewSubscriptionPropertyLimiter(subscriptions SubscriptionRepository, tariffs TariffRepository) *SubscriptionPropertyLimiter {
	return &SubscriptionPropertyLimiter{subscriptions: subscriptions, tariffs: tariffs}
}

// ActivePropertyLimit returns the limit for the user. If the user has no paid
// active subscription, the limit is 0. Unlimited tariffs are represented as
// math.MaxInt32.
func (l *SubscriptionPropertyLimiter) ActivePropertyLimit(ctx context.Context, userID uuid.UUID) (int, error) {
	sub, err := l.subscriptions.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil
		}
		return 0, fmt.Errorf("get subscription: %w", err)
	}

	if sub.Status != domain.SubscriptionStatusActive && sub.Status != domain.SubscriptionStatusGrace {
		return 0, nil
	}

	tariff, err := l.tariffs.GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil
		}
		return 0, fmt.Errorf("get tariff: %w", err)
	}

	if tariff.ActivePropertyLimit < 0 {
		return math.MaxInt32, nil
	}
	return tariff.ActivePropertyLimit, nil
}

var _ PropertyLimiter = (*SubscriptionPropertyLimiter)(nil)
