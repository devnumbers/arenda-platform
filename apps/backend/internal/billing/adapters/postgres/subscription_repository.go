package postgres

import (
	"context"
	"errors"
	"fmt"

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

func (r *invalidSubscriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	return domain.Subscription{}, fmt.Errorf("billing: unsupported transaction type %T for SubscriptionRepository.GetByID", r.tx)
}

func (r *invalidSubscriptionRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	return domain.Subscription{}, fmt.Errorf("billing: unsupported transaction type %T for SubscriptionRepository.GetByIDForUpdate", r.tx)
}

func (r *invalidSubscriptionRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Subscription, error) {
	return domain.Subscription{}, fmt.Errorf("billing: unsupported transaction type %T for SubscriptionRepository.GetByUserID", r.tx)
}

func (r *invalidSubscriptionRepository) GetByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.Subscription, error) {
	return domain.Subscription{}, fmt.Errorf("billing: unsupported transaction type %T for SubscriptionRepository.GetByUserIDForUpdate", r.tx)
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

// GetByID returns a subscription by ID.
func (r *SubscriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	row, err := r.q().GetSubscriptionByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, application.ErrNotFound
		}
		return domain.Subscription{}, fmt.Errorf("get subscription by id: %w", err)
	}
	return mapSubscription(row), nil
}

// GetByUserID returns a subscription by user ID.
func (r *SubscriptionRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (domain.Subscription, error) {
	row, err := r.q().GetSubscriptionByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, application.ErrNotFound
		}
		return domain.Subscription{}, fmt.Errorf("get subscription by user id: %w", err)
	}
	return mapSubscription(row), nil
}

// GetByIDForUpdate returns a subscription by ID, locking the row for update.
func (r *SubscriptionRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	row, err := r.q().GetSubscriptionByIDForUpdate(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, application.ErrNotFound
		}
		return domain.Subscription{}, fmt.Errorf("get subscription by id for update: %w", err)
	}
	return mapSubscription(row), nil
}

// GetByUserIDForUpdate returns a subscription by user ID, locking the row for update.
func (r *SubscriptionRepository) GetByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.Subscription, error) {
	row, err := r.q().GetSubscriptionByUserIDForUpdate(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, application.ErrNotFound
		}
		return domain.Subscription{}, fmt.Errorf("get subscription by user id for update: %w", err)
	}
	return mapSubscription(row), nil
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

func periodTextPtr(p *domain.SubscriptionPeriod) pgtype.Text {
	if p == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: string(*p), Valid: true}
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
		PendingPeriod:         periodTextPtr(sub.PendingPeriod),
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
		PendingPeriod:         periodTextPtr(sub.PendingPeriod),
		ActivePaymentMethodID: uuidPtr(sub.ActivePaymentMethodID),
	}
}

func mapSubscription(row postgres.UserSubscription) domain.Subscription {
	var pendingPeriod *domain.SubscriptionPeriod
	if row.PendingPeriod.Valid {
		p := domain.SubscriptionPeriod(row.PendingPeriod.String)
		pendingPeriod = &p
	}
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
		PendingPeriod:         pendingPeriod,
		ActivePaymentMethodID: uuidPtrFromPgtype(row.ActivePaymentMethodID),
	}
}


