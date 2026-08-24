// Package postgres holds the notifications persistence adapters: preference and push-subscription repositories
// and the owner contact resolver.
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PreferenceRepository persists the per-channel notification preferences
// (ADR 0030) using generated sqlc queries.
type PreferenceRepository struct {
	db postgres.DBTX
}

// NewPreferenceRepository creates a new preference repository.
func NewPreferenceRepository(db postgres.DBTX) *PreferenceRepository {
	return &PreferenceRepository{db: db}
}

func (r *PreferenceRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *PreferenceRepository) WithTx(tx transaction.Tx) application.PreferenceRepository {
	pgtx, ok := tx.(postgres.DBTX)
	if !ok {
		panic(fmt.Sprintf("notifications.PreferenceRepository: expected postgres.DBTX, got %T", tx))
	}
	return NewPreferenceRepository(pgtx)
}

// ListChannelPreferences returns the stored per-channel preference rows of a
// user (ADR 0030). A missing row means the (event type, channel) pair is
// allowed.
func (r *PreferenceRepository) ListChannelPreferences(
	ctx context.Context, userID uuid.UUID,
) ([]domain.NotificationChannelPreference, error) {
	rows, err := r.q().ListNotificationChannelPreferences(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, fmt.Errorf("list notification channel preferences: %w", err)
	}
	out := make([]domain.NotificationChannelPreference, len(rows))
	for i, row := range rows {
		out[i] = domain.NotificationChannelPreference{
			EventType: domain.EventType(row.EventType),
			Channel:   domain.NotificationChannel(row.Channel),
			Allowed:   row.Allowed,
		}
	}
	return out, nil
}

// UpsertChannelPreference inserts or updates one per-channel preference row.
func (r *PreferenceRepository) UpsertChannelPreference(
	ctx context.Context,
	userID uuid.UUID,
	pref domain.NotificationChannelPreference,
) error {
	if err := r.q().UpsertNotificationChannelPreference(ctx, postgres.UpsertNotificationChannelPreferenceParams{
		UserID:    pgconv.UUIDToPgtype(userID),
		EventType: postgres.NotificationEventType(pref.EventType),
		Channel:   postgres.NotificationChannel(pref.Channel),
		Allowed:   pref.Allowed,
	}); err != nil {
		return fmt.Errorf("upsert notification channel preference: %w", err)
	}
	return nil
}

// IsChannelAllowed reports whether the user permits sending notifications of
// the given event type over the given channel. A missing row means allowed.
func (r *PreferenceRepository) IsChannelAllowed(
	ctx context.Context,
	userID uuid.UUID,
	eventType domain.EventType,
	channel domain.NotificationChannel,
) (bool, error) {
	allowed, err := r.q().IsNotificationChannelAllowed(ctx, postgres.IsNotificationChannelAllowedParams{
		UserID:    pgconv.UUIDToPgtype(userID),
		EventType: postgres.NotificationEventType(eventType),
		Channel:   postgres.NotificationChannel(channel),
	})
	if err != nil {
		return false, fmt.Errorf("check notification channel allowed: %w", err)
	}
	return allowed, nil
}
