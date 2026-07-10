package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// PaymentMethodInUseChecker implements application.PaymentMethodInUseChecker.
type PaymentMethodInUseChecker struct {
	db postgres.DBTX
}

// NewPaymentMethodInUseChecker creates a new checker.
func NewPaymentMethodInUseChecker(db postgres.DBTX) *PaymentMethodInUseChecker {
	return &PaymentMethodInUseChecker{db: db}
}

// IsInUse reports whether the payment method is referenced as active by any subscription.
func (c *PaymentMethodInUseChecker) IsInUse(ctx context.Context, methodID uuid.UUID) (bool, error) {
	count, err := postgres.New(c.db).CountSubscriptionsByActivePaymentMethodID(ctx, pgtype.UUID{Bytes: methodID, Valid: true})
	if err != nil {
		return false, fmt.Errorf("count subscriptions by active payment method: %w", err)
	}
	return count > 0, nil
}

var _ application.PaymentMethodInUseChecker = (*PaymentMethodInUseChecker)(nil)
