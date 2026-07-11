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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
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
func (r *SubscriptionRepository) WithTx(tx transaction.Tx) (application.SubscriptionRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("billing.SubscriptionRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewSubscriptionRepository(dbtx), nil
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
	return mapSubscription(row)
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
	return mapSubscription(row)
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
	return mapSubscription(row)
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
	return mapSubscription(row)
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
			return mapSubscription(existing)
		}
		return domain.Subscription{}, fmt.Errorf("create subscription: %w", err)
	}
	return mapSubscription(row)
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

// ListUpForRenewal returns active subscriptions with auto-renew enabled whose
// validity period has ended.
func (r *SubscriptionRepository) ListUpForRenewal(ctx context.Context, now time.Time, limit int32) ([]domain.Subscription, error) {
	rows, err := r.q().ListSubscriptionsUpForRenewal(ctx, postgres.ListSubscriptionsUpForRenewalParams{
		ValidUntil: pgtype.Timestamptz{Time: now.UTC(), Valid: true},
		Limit:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list subscriptions up for renewal: %w", err)
	}
	subs := make([]domain.Subscription, 0, len(rows))
	for _, row := range rows {
		sub, err := mapSubscription(row)
		if err != nil {
			return nil, fmt.Errorf("map subscription row: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// ListInExpiredGrace returns subscriptions whose grace period has ended.
func (r *SubscriptionRepository) ListInExpiredGrace(ctx context.Context, now time.Time, limit int32) ([]domain.Subscription, error) {
	rows, err := r.q().ListSubscriptionsInExpiredGrace(ctx, postgres.ListSubscriptionsInExpiredGraceParams{
		ValidUntil: pgtype.Timestamptz{Time: now.UTC(), Valid: true},
		Limit:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list subscriptions in expired grace: %w", err)
	}
	subs := make([]domain.Subscription, 0, len(rows))
	for _, row := range rows {
		sub, err := mapSubscription(row)
		if err != nil {
			return nil, fmt.Errorf("map subscription row: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// ListExpiredNonRenewing returns active paid subscriptions whose validity period
// has ended and that have auto-renew disabled. They must be downgraded to basic.
func (r *SubscriptionRepository) ListExpiredNonRenewing(ctx context.Context, now time.Time, limit int32) ([]domain.Subscription, error) {
	rows, err := r.q().ListExpiredNonRenewingSubscriptions(ctx, postgres.ListExpiredNonRenewingSubscriptionsParams{
		ValidUntil: pgtype.Timestamptz{Time: now.UTC(), Valid: true},
		Limit:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list expired non-renewing subscriptions: %w", err)
	}
	subs := make([]domain.Subscription, 0, len(rows))
	for _, row := range rows {
		sub, err := mapSubscription(row)
		if err != nil {
			return nil, fmt.Errorf("map subscription row: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// ListExpiredCancelled returns cancelled subscriptions whose retained validity
// period has ended. They must be downgraded to basic.
func (r *SubscriptionRepository) ListExpiredCancelled(ctx context.Context, now time.Time, limit int32) ([]domain.Subscription, error) {
	rows, err := r.q().ListExpiredCancelledSubscriptions(ctx, postgres.ListExpiredCancelledSubscriptionsParams{
		ValidUntil: pgtype.Timestamptz{Time: now.UTC(), Valid: true},
		Limit:      limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list expired cancelled subscriptions: %w", err)
	}
	subs := make([]domain.Subscription, 0, len(rows))
	for _, row := range rows {
		sub, err := mapSubscription(row)
		if err != nil {
			return nil, fmt.Errorf("map subscription row: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// ListPendingChanges returns active subscriptions whose scheduled tariff change
// is due. They are locked with SKIP LOCKED for worker processing.
func (r *SubscriptionRepository) ListPendingChanges(ctx context.Context, now time.Time, limit int32) ([]domain.Subscription, error) {
	rows, err := r.q().ListSubscriptionsWithPendingChange(ctx, postgres.ListSubscriptionsWithPendingChangeParams{
		PendingChangeAt: pgtype.Timestamptz{Time: now.UTC(), Valid: true},
		Limit:           limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list subscriptions with pending change: %w", err)
	}
	subs := make([]domain.Subscription, 0, len(rows))
	for _, row := range rows {
		sub, err := mapSubscription(row)
		if err != nil {
			return nil, fmt.Errorf("map subscription row: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, nil
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
		ValidUntil:            pgconv.TimePtrToPgtype(sub.ValidUntil),
		AutoRenewEnabled:      sub.AutoRenewEnabled,
		PendingTariffID:       pgconv.UUIDToPgtypePtr(sub.PendingTariffID),
		PendingChangeAt:       pgconv.TimePtrToPgtype(sub.PendingChangeAt),
		PendingPeriod:         periodTextPtr(sub.PendingPeriod),
		ActivePaymentMethodID: pgconv.UUIDToPgtypePtr(sub.ActivePaymentMethodID),
		LastAppliedPaymentID:  pgconv.UUIDToPgtypePtr(sub.LastAppliedPaymentID),
	}
}

func mapUpdateSubscriptionParams(sub domain.Subscription) postgres.UpdateSubscriptionParams {
	return postgres.UpdateSubscriptionParams{
		ID:                    pgtype.UUID{Bytes: sub.ID, Valid: true},
		TariffID:              pgtype.UUID{Bytes: sub.TariffID, Valid: true},
		Source:                string(sub.Source),
		Status:                string(sub.Status),
		ValidUntil:            pgconv.TimePtrToPgtype(sub.ValidUntil),
		AutoRenewEnabled:      sub.AutoRenewEnabled,
		PendingTariffID:       pgconv.UUIDToPgtypePtr(sub.PendingTariffID),
		PendingChangeAt:       pgconv.TimePtrToPgtype(sub.PendingChangeAt),
		PendingPeriod:         periodTextPtr(sub.PendingPeriod),
		ActivePaymentMethodID: pgconv.UUIDToPgtypePtr(sub.ActivePaymentMethodID),
		LastAppliedPaymentID:  pgconv.UUIDToPgtypePtr(sub.LastAppliedPaymentID),
	}
}

func mapSubscription(row postgres.UserSubscription) (domain.Subscription, error) {
	var pendingPeriod *domain.SubscriptionPeriod
	if row.PendingPeriod.Valid {
		p := domain.SubscriptionPeriod(row.PendingPeriod.String)
		pendingPeriod = &p
	}
	return domain.ReconstituteSubscription(domain.Subscription{
		ID:                    pgconv.UUIDFromPgtype(row.ID),
		UserID:                pgconv.UUIDFromPgtype(row.UserID),
		TariffID:              pgconv.UUIDFromPgtype(row.TariffID),
		Source:                domain.SubscriptionSource(row.Source),
		Status:                domain.SubscriptionStatus(row.Status),
		ValidUntil:            pgconv.TimestamptzToPtrTime(row.ValidUntil),
		AutoRenewEnabled:      row.AutoRenewEnabled,
		PendingTariffID:       pgconv.UUIDFromPgtypePtr(row.PendingTariffID),
		PendingChangeAt:       pgconv.TimestamptzToPtrTime(row.PendingChangeAt),
		PendingPeriod:         pendingPeriod,
		ActivePaymentMethodID: pgconv.UUIDFromPgtypePtr(row.ActivePaymentMethodID),
		LastAppliedPaymentID:  pgconv.UUIDFromPgtypePtr(row.LastAppliedPaymentID),
	})
}


