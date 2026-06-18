package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return &invalidSubscriptionRepository{tx: tx}
	}
	return NewSubscriptionRepository(dbtx)
}

// invalidSubscriptionRepository returns a clear error for every method when an
// unsupported transaction type is passed to WithTx.
type invalidSubscriptionRepository struct {
	tx transaction.Tx
}

func (r *invalidSubscriptionRepository) Create(ctx context.Context, sub domain.Subscription) (domain.Subscription, error) {
	return domain.Subscription{}, fmt.Errorf("billing: unsupported transaction type %T for SubscriptionRepository.Create", r.tx)
}

func (r *invalidSubscriptionRepository) Update(ctx context.Context, sub domain.Subscription) error {
	return fmt.Errorf("billing: unsupported transaction type %T for SubscriptionRepository.Update", r.tx)
}

func (r *invalidSubscriptionRepository) WithTx(tx transaction.Tx) application.SubscriptionRepository {
	return r
}

// Create inserts a new subscription. If a subscription already exists for the
// user, the existing row is returned without error.
func (r *SubscriptionRepository) Create(ctx context.Context, sub domain.Subscription) (domain.Subscription, error) {
	row, err := r.q().CreateSubscription(ctx, mapCreateSubscriptionParams(sub))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			existing, err := r.q().GetSubscriptionByUserID(ctx, pgtype.UUID{Bytes: sub.UserID, Valid: true})
			if err != nil {
				return domain.Subscription{}, fmt.Errorf("fetch existing subscription: %w", err)
			}
			return mapSubscription(existing), nil
		}
		return domain.Subscription{}, fmt.Errorf("create subscription: %w", err)
	}
	return mapSubscription(row), nil
}

// Update persists all mutable fields of the subscription.
func (r *SubscriptionRepository) Update(ctx context.Context, sub domain.Subscription) error {
	if _, err := r.q().UpdateSubscription(ctx, mapUpdateSubscriptionParams(sub)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("update subscription: %w", err)
	}
	return nil
}

func mapCreateSubscriptionParams(sub domain.Subscription) postgres.CreateSubscriptionParams {
	return postgres.CreateSubscriptionParams{
		UserID:                pgtype.UUID{Bytes: sub.UserID, Valid: true},
		TariffID:              pgtype.UUID{Bytes: sub.TariffID, Valid: true},
		Source:                string(sub.Source),
		Status:                string(sub.Status),
		ValidUntil:            timestamptzPtr(sub.ValidUntil),
		AutoRenewEnabled:      sub.AutoRenewEnabled,
		PendingTariffID:       uuidPtr(sub.PendingTariffID),
		PendingChangeAt:       timestamptzPtr(sub.PendingChangeAt),
		ActivePaymentMethodID: uuidPtr(sub.ActivePaymentMethodID),
	}
}

func mapUpdateSubscriptionParams(sub domain.Subscription) postgres.UpdateSubscriptionParams {
	return postgres.UpdateSubscriptionParams{
		ID:                    pgtype.UUID{Bytes: sub.ID, Valid: true},
		TariffID:              pgtype.UUID{Bytes: sub.TariffID, Valid: true},
		Source:                string(sub.Source),
		Status:                string(sub.Status),
		ValidUntil:            timestamptzPtr(sub.ValidUntil),
		AutoRenewEnabled:      sub.AutoRenewEnabled,
		PendingTariffID:       uuidPtr(sub.PendingTariffID),
		PendingChangeAt:       timestamptzPtr(sub.PendingChangeAt),
		ActivePaymentMethodID: uuidPtr(sub.ActivePaymentMethodID),
	}
}

func mapSubscription(row postgres.UserSubscription) domain.Subscription {
	return domain.Subscription{
		ID:                    uuid.UUID(row.ID.Bytes),
		UserID:                uuid.UUID(row.UserID.Bytes),
		TariffID:              uuid.UUID(row.TariffID.Bytes),
		Source:                domain.SubscriptionSource(row.Source),
		Status:                domain.SubscriptionStatus(row.Status),
		ValidUntil:            timePtr(row.ValidUntil),
		AutoRenewEnabled:      row.AutoRenewEnabled,
		PendingTariffID:       uuidPtrFromPgtype(row.PendingTariffID),
		PendingChangeAt:       timePtr(row.PendingChangeAt),
		ActivePaymentMethodID: uuidPtrFromPgtype(row.ActivePaymentMethodID),
	}
}

func uuidPtr(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func uuidPtrFromPgtype(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	id := uuid.UUID(u.Bytes)
	return &id
}

func timestamptzPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
