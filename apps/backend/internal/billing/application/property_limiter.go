package application

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// SubscriptionPropertyLimiter computes the active property limit from the
// user's subscription and tariff. It implements PropertyLimiter.
type SubscriptionPropertyLimiter struct {
	subscriptions SubscriptionRepository
	tariffs       TariffRepository
	// lock, when set by WithTx, makes ActivePropertyLimit read the subscription
	// with SELECT ... FOR UPDATE so concurrent property-limit checks serialize
	// on the subscription row instead of racing past the limit.
	lock bool
}

// NewSubscriptionPropertyLimiter creates a new limiter.
func NewSubscriptionPropertyLimiter(subscriptions SubscriptionRepository, tariffs TariffRepository) *SubscriptionPropertyLimiter {
	return &SubscriptionPropertyLimiter{subscriptions: subscriptions, tariffs: tariffs}
}

// WithTx returns a limiter bound to the provided transaction. The returned
// limiter reads the subscription with a row lock, serializing concurrent limit
// checks so two racing CreateProperty/UnarchiveProperty calls cannot both pass.
func (l *SubscriptionPropertyLimiter) WithTx(tx transaction.Tx) (PropertyLimiter, error) {
	subRepo, err := l.subscriptions.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind subscription limiter transaction: %w", err)
	}
	tariffRepo, err := l.tariffs.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind tariff limiter transaction: %w", err)
	}
	return &SubscriptionPropertyLimiter{subscriptions: subRepo, tariffs: tariffRepo, lock: true}, nil
}

// ActivePropertyLimit returns the limit for the user. If the user has no paid
// active subscription, the limit is 0. Unlimited tariffs are represented as
// math.MaxInt32.
func (l *SubscriptionPropertyLimiter) ActivePropertyLimit(ctx context.Context, userID uuid.UUID) (int, error) {
	var sub domain.Subscription
	var err error
	if l.lock {
		sub, err = l.subscriptions.GetByUserIDForUpdate(ctx, userID)
	} else {
		sub, err = l.subscriptions.GetByUserID(ctx, userID)
	}
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

	if tariff.ActivePropertyLimit == domain.UnlimitedPropertyLimit {
		return math.MaxInt32, nil
	}
	return tariff.ActivePropertyLimit, nil
}

var _ PropertyLimiter = (*SubscriptionPropertyLimiter)(nil)
