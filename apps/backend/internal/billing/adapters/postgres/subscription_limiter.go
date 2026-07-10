package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// SubscriptionLimiter enforces per-user active property limits by delegating to
// the billing application layer. It adapts application.PropertyLimiter to the
// properties context's SubscriptionLimiter port.
type SubscriptionLimiter struct {
	limiter application.PropertyLimiter
}

// NewSubscriptionLimiter creates a subscription-backed property limiter.
func NewSubscriptionLimiter(limiter application.PropertyLimiter) *SubscriptionLimiter {
	return &SubscriptionLimiter{limiter: limiter}
}

// ActivePropertyLimit returns the maximum number of active properties the user
// is allowed to own.
func (l *SubscriptionLimiter) ActivePropertyLimit(ctx context.Context, userID uuid.UUID) (int, error) {
	return l.limiter.ActivePropertyLimit(ctx, userID)
}

// WithTx returns a limiter instance bound to the provided transaction. The
// underlying application rule is transaction-agnostic, so the adapter simply
// returns a new wrapper around the same limiter.
func (l *SubscriptionLimiter) WithTx(tx transaction.Tx) (propertiesapp.SubscriptionLimiter, error) {
	_ = tx
	return &SubscriptionLimiter{limiter: l.limiter}, nil
}

// Compile-time check that SubscriptionLimiter implements the properties port.
var _ propertiesapp.SubscriptionLimiter = (*SubscriptionLimiter)(nil)
