package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// SubscriptionLimiter enforces per-user active property limits by delegating to
// the billing application's subscription property limiter. It adapts the
// billing limiter to the properties context's SubscriptionLimiter port so the
// properties application never imports billing.
type SubscriptionLimiter struct {
	limiter *application.SubscriptionPropertyLimiter
}

// NewSubscriptionLimiter creates a subscription-backed property limiter.
func NewSubscriptionLimiter(limiter *application.SubscriptionPropertyLimiter) *SubscriptionLimiter {
	return &SubscriptionLimiter{limiter: limiter}
}

// ActivePropertyLimit returns the maximum number of active properties the user
// is allowed to own.
func (l *SubscriptionLimiter) ActivePropertyLimit(ctx context.Context, userID uuid.UUID) (int, error) {
	return l.limiter.ActivePropertyLimit(ctx, userID)
}

// WithTx returns a limiter bound to the provided transaction. It binds the
// inner billing limiter to the transaction so the subscription is read with a
// row lock, serializing concurrent property-limit checks.
func (l *SubscriptionLimiter) WithTx(tx transaction.Tx) (propertiesapp.SubscriptionLimiter, error) {
	txLimiter, err := l.limiter.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind property limiter transaction: %w", err)
	}
	return &SubscriptionLimiter{limiter: txLimiter}, nil
}

// Compile-time check that SubscriptionLimiter implements the properties port.
var _ propertiesapp.SubscriptionLimiter = (*SubscriptionLimiter)(nil)
