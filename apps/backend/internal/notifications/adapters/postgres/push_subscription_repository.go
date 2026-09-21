package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// PushSubscriptionRepository persists Web Push subscriptions using generated
// sqlc queries.
type PushSubscriptionRepository struct {
	db postgres.DBTX
}

// NewPushSubscriptionRepository creates a new push subscription repository.
func NewPushSubscriptionRepository(db postgres.DBTX) *PushSubscriptionRepository {
	return &PushSubscriptionRepository{db: db}
}

func (r *PushSubscriptionRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// Upsert inserts a subscription keyed by endpoint, or updates its mutable
// fields when the endpoint already exists (idempotent re-subscribe). The
// per-device settings (master + category flags) travel with every call.
func (r *PushSubscriptionRepository) Upsert(ctx context.Context, sub domain.PushSubscription) (domain.PushSubscription, error) {
	row, err := r.q().UpsertPushSubscription(ctx, postgres.UpsertPushSubscriptionParams{
		ID:                         pgconv.UUIDToPgtype(sub.ID),
		UserID:                     pgconv.UUIDToPgtype(sub.UserID),
		Endpoint:                   sub.Endpoint,
		P256dh:                     sub.P256dh,
		Auth:                       sub.Auth,
		ExpirationTime:             pgconv.TimePtrToPgtype(sub.ExpirationTime),
		Enabled:                    sub.Enabled,
		CategoryRental:             sub.Categories.Rental,
		CategoryPaymentsOperations: sub.Categories.PaymentsOperations,
		CategoryTasks:              sub.Categories.Tasks,
		CategorySharedAccess:       sub.Categories.SharedAccess,
		CreatedAt:                  pgtype.Timestamptz{Time: sub.CreatedAt, Valid: true},
		UpdatedAt:                  pgtype.Timestamptz{Time: sub.UpdatedAt, Valid: true},
	})
	if err != nil {
		return domain.PushSubscription{}, fmt.Errorf("upsert push subscription: %w", err)
	}
	return pushSubscriptionToDomain(row), nil
}

// Delete removes a subscription by endpoint scoped to a user. It returns
// ErrNotFound when no row matched.
func (r *PushSubscriptionRepository) Delete(ctx context.Context, userID uuid.UUID, endpoint string) error {
	rows, err := r.q().DeletePushSubscriptionByEndpointAndUser(ctx, postgres.DeletePushSubscriptionByEndpointAndUserParams{
		Endpoint: endpoint,
		UserID:   pgconv.UUIDToPgtype(userID),
	})
	if err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	if rows == 0 {
		return application.ErrNotFound
	}
	return nil
}

// ListByUser returns all stored subscriptions for a user ordered by created_at.
func (r *PushSubscriptionRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.PushSubscription, error) {
	rows, err := r.q().ListPushSubscriptionsByUser(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions: %w", err)
	}
	out := make([]domain.PushSubscription, len(rows))
	for i, row := range rows {
		out[i] = pushSubscriptionToDomain(row)
	}
	return out, nil
}

// GetByEndpoint returns the user's subscription with its settings state.
// ErrNotFound when the endpoint is not this user's.
func (r *PushSubscriptionRepository) GetByEndpoint(
	ctx context.Context, userID uuid.UUID, endpoint string,
) (domain.PushSubscription, error) {
	row, err := r.q().GetPushSubscriptionByEndpointAndUser(ctx, postgres.GetPushSubscriptionByEndpointAndUserParams{
		Endpoint: endpoint,
		UserID:   pgconv.UUIDToPgtype(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PushSubscription{}, application.ErrNotFound
		}
		return domain.PushSubscription{}, fmt.Errorf("get push subscription by endpoint: %w", err)
	}
	return pushSubscriptionToDomain(row), nil
}

// UpdatePreferences replaces the device's delivery state (master + category
// flags), keeping the subscription's keys. False means the endpoint is not
// this user's.
func (r *PushSubscriptionRepository) UpdatePreferences(
	ctx context.Context, userID uuid.UUID, endpoint string, enabled bool, prefs domain.CategoryPrefs,
) (bool, error) {
	rows, err := r.q().UpdatePushSubscriptionPreferences(ctx, postgres.UpdatePushSubscriptionPreferencesParams{
		Endpoint:                   endpoint,
		UserID:                     pgconv.UUIDToPgtype(userID),
		Enabled:                    enabled,
		CategoryRental:             prefs.Rental,
		CategoryPaymentsOperations: prefs.PaymentsOperations,
		CategoryTasks:              prefs.Tasks,
		CategorySharedAccess:       prefs.SharedAccess,
	})
	if err != nil {
		return false, fmt.Errorf("update push subscription preferences: %w", err)
	}
	return rows > 0, nil
}

func pushSubscriptionToDomain(row postgres.PushSubscription) domain.PushSubscription {
	return domain.PushSubscription{
		ID:             pgconv.UUIDFromPgtype(row.ID),
		UserID:         pgconv.UUIDFromPgtype(row.UserID),
		Endpoint:       row.Endpoint,
		P256dh:         row.P256dh,
		Auth:           row.Auth,
		ExpirationTime: pgconv.TimestamptzToPtrTime(row.ExpirationTime),
		Enabled:        row.Enabled,
		Categories: domain.CategoryPrefs{
			Rental:             row.CategoryRental,
			PaymentsOperations: row.CategoryPaymentsOperations,
			Tasks:              row.CategoryTasks,
			SharedAccess:       row.CategorySharedAccess,
		},
		CreatedAt: pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt: pgconv.TimestamptzToTime(row.UpdatedAt),
	}
}
