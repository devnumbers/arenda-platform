package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// SubscriptionLimiter enforces per-user active property limits based on the
// user's billing subscription and its tariff.
type SubscriptionLimiter struct {
	db postgres.DBTX
}

// NewSubscriptionLimiter creates a subscription-backed property limiter.
func NewSubscriptionLimiter(db postgres.DBTX) *SubscriptionLimiter {
	return &SubscriptionLimiter{db: db}
}

func (l *SubscriptionLimiter) q() *postgres.Queries {
	return postgres.New(l.db)
}

// ActivePropertyLimit returns the maximum number of active properties the user
// is allowed to own. If the user has no active subscription, the limit is 0.
func (l *SubscriptionLimiter) ActivePropertyLimit(ctx context.Context, userID uuid.UUID) (int, error) {
	sub, err := l.q().GetSubscriptionByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get subscription by user id: %w", err)
	}

	if sub.Status != string(domain.SubscriptionStatusActive) {
		return 0, nil
	}

	tariff, err := l.q().GetTariffByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get tariff by id: %w", err)
	}

	limit := int(tariff.ActivePropertyLimit)
	if limit < 0 {
		return math.MaxInt32, nil
	}
	return limit, nil
}

// Compile-time check that SubscriptionLimiter implements the properties port.
var _ propertiesapp.SubscriptionLimiter = (*SubscriptionLimiter)(nil)
