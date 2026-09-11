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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared"
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

// GetByUserID returns the user's subscription.
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

// GetByUserIDForUpdate returns the user's subscription, locking the row for
// update. Must only be called inside a transaction.
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

// List returns a batch of subscriptions matching the worker selection (issue
// #286): the parameterized query every phase lists and re-checks its batch
// through. The predicate lives here, in SQL.
func (r *SubscriptionRepository) List(ctx context.Context, sel application.SubscriptionSelection) ([]domain.Subscription, error) {
	rows, err := r.q().ListSubscriptionsBySelection(ctx, subscriptionSelectionParams(sel))
	if err != nil {
		return nil, fmt.Errorf("list subscriptions by selection: %w", err)
	}
	return mapSubscriptions(rows)
}

// subscriptionSelectionParams maps the selection values to the query
// parameters; a nil optional bound becomes an invalid (dropped) pgtype.
func subscriptionSelectionParams(sel application.SubscriptionSelection) postgres.ListSubscriptionsBySelectionParams {
	params := postgres.ListSubscriptionsBySelectionParams{
		Status:     string(sel.Status),
		Unreminded: sel.Unreminded,
		BatchLimit: batchLimit(sel.Limit),
	}
	if sel.UserID != nil {
		params.UserID = pgtype.UUID{Bytes: *sel.UserID, Valid: true}
	}
	if sel.AutoRenewEnabled != nil {
		params.AutoRenew = pgtype.Bool{Bool: *sel.AutoRenewEnabled, Valid: true}
	}
	params.ValidUntilBefore = pgconv.TimePtrToPgtype(sel.ValidUntilBefore)
	params.ValidUntilAfter = pgconv.TimePtrToPgtype(sel.ValidUntilAfter)
	params.PendingChangeDue = pgconv.TimePtrToPgtype(sel.PendingChangeDue)
	params.GraceRetryDue = pgconv.TimePtrToPgtype(sel.GraceRetryDue)
	return params
}

// batchLimit converts the configured worker batch size to the sqlc parameter
// type. The value is a small operational constant from the billing config
// (default 100), never near the int32 bounds.
func batchLimit(limit int) int32 {
	return shared.ToInt32Clamped(limit)
}

// mapSubscriptions converts mapped rows, preserving the query order.
func mapSubscriptions(rows []postgres.UserSubscription) ([]domain.Subscription, error) {
	subs := make([]domain.Subscription, 0, len(rows))
	for _, row := range rows {
		sub, err := mapSubscription(row)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// Create inserts a new subscription. If a subscription already exists for the
// user (unique user_id), the existing row is returned without error —
// onboarding relies on this for redelivered registration events.
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

func periodTextPtr(p *domain.SubscriptionPeriod) pgtype.Text {
	if p == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: string(*p), Valid: true}
}

func mapCreateSubscriptionParams(sub domain.Subscription) postgres.CreateSubscriptionParams {
	return postgres.CreateSubscriptionParams{
		ID:                       pgtype.UUID{Bytes: sub.ID, Valid: true},
		UserID:                   pgtype.UUID{Bytes: sub.UserID, Valid: true},
		TariffID:                 pgtype.UUID{Bytes: sub.TariffID, Valid: true},
		Source:                   string(sub.Source),
		Status:                   string(sub.Status),
		ValidUntil:               pgconv.TimePtrToPgtype(sub.ValidUntil),
		AutoRenewEnabled:         sub.AutoRenewEnabled,
		PendingTariffID:          pgconv.UUIDToPgtypePtr(sub.PendingTariffID),
		PendingChangeAt:          pgconv.TimePtrToPgtype(sub.PendingChangeAt),
		PendingPeriod:            periodTextPtr(sub.PendingPeriod),
		ActivePaymentMethodID:    pgconv.UUIDToPgtypePtr(sub.ActivePaymentMethodID),
		LastAppliedPaymentID:     pgconv.UUIDToPgtypePtr(sub.LastAppliedPaymentID),
		CurrentPeriod:            periodTextPtr(sub.CurrentPeriod),
		GraceArchivedPropertyIds: pgconv.UUIDSliceToPgtype(sub.GraceArchivedPropertyIDs),
	}
}

func mapUpdateSubscriptionParams(sub domain.Subscription) postgres.UpdateSubscriptionParams {
	return postgres.UpdateSubscriptionParams{
		ID:                       pgtype.UUID{Bytes: sub.ID, Valid: true},
		TariffID:                 pgtype.UUID{Bytes: sub.TariffID, Valid: true},
		Source:                   string(sub.Source),
		Status:                   string(sub.Status),
		ValidUntil:               pgconv.TimePtrToPgtype(sub.ValidUntil),
		AutoRenewEnabled:         sub.AutoRenewEnabled,
		PendingTariffID:          pgconv.UUIDToPgtypePtr(sub.PendingTariffID),
		PendingChangeAt:          pgconv.TimePtrToPgtype(sub.PendingChangeAt),
		PendingPeriod:            periodTextPtr(sub.PendingPeriod),
		ActivePaymentMethodID:    pgconv.UUIDToPgtypePtr(sub.ActivePaymentMethodID),
		LastAppliedPaymentID:     pgconv.UUIDToPgtypePtr(sub.LastAppliedPaymentID),
		CurrentPeriod:            periodTextPtr(sub.CurrentPeriod),
		GraceRemindedAt:          pgconv.TimePtrToPgtype(sub.GraceRemindedAt),
		GraceArchivedPropertyIds: pgconv.UUIDSliceToPgtype(sub.GraceArchivedPropertyIDs),
	}
}

func mapSubscription(row postgres.UserSubscription) (domain.Subscription, error) {
	var pendingPeriod *domain.SubscriptionPeriod
	if row.PendingPeriod.Valid {
		p := domain.SubscriptionPeriod(row.PendingPeriod.String)
		pendingPeriod = &p
	}
	var currentPeriod *domain.SubscriptionPeriod
	if row.CurrentPeriod.Valid {
		p := domain.SubscriptionPeriod(row.CurrentPeriod.String)
		currentPeriod = &p
	}
	return domain.ReconstituteSubscription(domain.Subscription{
		ID:                       pgconv.UUIDFromPgtype(row.ID),
		UserID:                   pgconv.UUIDFromPgtype(row.UserID),
		TariffID:                 pgconv.UUIDFromPgtype(row.TariffID),
		Source:                   domain.SubscriptionSource(row.Source),
		Status:                   domain.SubscriptionStatus(row.Status),
		ValidUntil:               pgconv.TimestamptzToPtrTime(row.ValidUntil),
		AutoRenewEnabled:         row.AutoRenewEnabled,
		PendingTariffID:          pgconv.UUIDFromPgtypePtr(row.PendingTariffID),
		PendingChangeAt:          pgconv.TimestamptzToPtrTime(row.PendingChangeAt),
		PendingPeriod:            pendingPeriod,
		ActivePaymentMethodID:    pgconv.UUIDFromPgtypePtr(row.ActivePaymentMethodID),
		LastAppliedPaymentID:     pgconv.UUIDFromPgtypePtr(row.LastAppliedPaymentID),
		CurrentPeriod:            currentPeriod,
		GraceRemindedAt:          pgconv.TimestamptzToPtrTime(row.GraceRemindedAt),
		GraceArchivedPropertyIDs: pgconv.UUIDSliceFromPgtype(row.GraceArchivedPropertyIds),
	})
}
