package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// SubscriptionRepository persists subscriptions.
type SubscriptionRepository struct {
	db postgres.DBTX
}

// NewSubscriptionRepository creates a new subscription repository.
func NewSubscriptionRepository(db postgres.DBTX) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *SubscriptionRepository) WithTx(tx transaction.Tx) application.SubscriptionRepository {
	return NewSubscriptionRepository(tx.(postgres.DBTX))
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub domain.Subscription) error {
	_, err := r.q().CreateSubscription(ctx, postgres.CreateSubscriptionParams{
		UserID:   pgtype.UUID{Bytes: sub.UserID, Valid: true},
		TariffID: pgtype.UUID{Bytes: sub.TariffID, Valid: true},
		Source:   string(sub.Source),
		Status:   string(sub.Status),
	})
	return err
}
