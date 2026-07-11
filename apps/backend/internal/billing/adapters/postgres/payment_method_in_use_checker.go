package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
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

// WithTx returns a checker bound to the given transaction.
func (c *PaymentMethodInUseChecker) WithTx(tx transaction.Tx) (application.PaymentMethodInUseChecker, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("postgres.PaymentMethodInUseChecker.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewPaymentMethodInUseChecker(dbtx), nil
}

var _ application.PaymentMethodInUseChecker = (*PaymentMethodInUseChecker)(nil)
