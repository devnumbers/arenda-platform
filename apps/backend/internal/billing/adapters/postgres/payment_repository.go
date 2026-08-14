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
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/pgerr"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// SubscriptionPaymentRepository persists subscription payments.
type SubscriptionPaymentRepository struct {
	db postgres.DBTX
}

// NewSubscriptionPaymentRepository creates a new payment repository.
func NewSubscriptionPaymentRepository(db postgres.DBTX) *SubscriptionPaymentRepository {
	return &SubscriptionPaymentRepository{db: db}
}

func (r *SubscriptionPaymentRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *SubscriptionPaymentRepository) WithTx(tx transaction.Tx) (application.SubscriptionPaymentRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("billing.SubscriptionPaymentRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewSubscriptionPaymentRepository(dbtx), nil
}

// Create inserts a new payment. A unique violation on the pending-payments
// partial index is narrowed to ErrAlreadyExists: a concurrent request won the
// initiation race and the caller returns the existing pending payment.
func (r *SubscriptionPaymentRepository) Create(ctx context.Context, payment domain.SubscriptionPayment) (domain.SubscriptionPayment, error) {
	row, err := r.q().CreateSubscriptionPayment(ctx, mapCreatePaymentParams(payment))
	if err != nil {
		if pgerr.IsUniqueViolation(err) {
			return domain.SubscriptionPayment{}, application.ErrAlreadyExists
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("create subscription payment: %w", err)
	}
	return mapSubscriptionPayment(row)
}

// GetByID returns a payment by its identifier.
func (r *SubscriptionPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	row, err := r.q().GetSubscriptionPaymentByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SubscriptionPayment{}, application.ErrNotFound
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("get subscription payment: %w", err)
	}
	return mapSubscriptionPayment(row)
}

// GetByIDForUpdate returns a payment by its identifier, locking the row for
// update. Must only be called inside a transaction.
func (r *SubscriptionPaymentRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	row, err := r.q().GetSubscriptionPaymentByIDForUpdate(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SubscriptionPayment{}, application.ErrNotFound
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("get subscription payment for update: %w", err)
	}
	return mapSubscriptionPayment(row)
}

// ListByUserID returns the user's payments, newest first.
func (r *SubscriptionPaymentRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	rows, err := r.q().ListSubscriptionPaymentsByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list subscription payments: %w", err)
	}
	return mapSubscriptionPayments(rows)
}

// ListPendingByUserID returns the user's pending payments, newest first. The
// payment flow uses it to return an existing pending initiation instead of
// creating a duplicate.
func (r *SubscriptionPaymentRepository) ListPendingByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	rows, err := r.q().ListPendingSubscriptionPaymentsByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list pending subscription payments: %w", err)
	}
	return mapSubscriptionPayments(rows)
}

// Update persists the mutable fields of a payment. Callers hold the row lock
// (GetByIDForUpdate) in the same transaction before mutating the aggregate.
func (r *SubscriptionPaymentRepository) Update(ctx context.Context, payment domain.SubscriptionPayment) error {
	if _, err := r.q().UpdateSubscriptionPayment(ctx, mapUpdatePaymentParams(payment)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("update subscription payment: %w", err)
	}
	return nil
}

func mapCreatePaymentParams(p domain.SubscriptionPayment) postgres.CreateSubscriptionPaymentParams {
	return postgres.CreateSubscriptionPaymentParams{
		ID:                    pgtype.UUID{Bytes: p.ID, Valid: true},
		UserID:                pgtype.UUID{Bytes: p.UserID, Valid: true},
		SubscriptionID:        pgtype.UUID{Bytes: p.SubscriptionID, Valid: true},
		TariffID:              pgtype.UUID{Bytes: p.TariffID, Valid: true},
		PaymentMethodID:       pgconv.UUIDToPgtypePtr(p.PaymentMethodID),
		Period:                string(p.Period),
		AmountKopecks:         p.AmountKopecks,
		Provider:              string(p.Provider),
		ProviderPaymentID:     pgconv.StringPtrToPgtype(p.ProviderPaymentID),
		PaymentUrl:            pgconv.StringPtrToPgtype(p.PaymentURL),
		Status:                string(p.Status),
		RefundedAmountKopecks: pgconv.Int8PtrToPgtype(p.RefundedAmountKopecks),
		// charge_attempts is a bounded retry counter (config limit is 3).
		ChargeAttempts: int32(p.ChargeAttempts), //nolint:gosec // small bounded counter, never overflows int32
		ErrorCode:      pgconv.StringPtrToPgtype(p.ErrorCode),
		SucceededAt:    pgconv.TimePtrToPgtype(p.SucceededAt),
	}
}

func mapUpdatePaymentParams(p domain.SubscriptionPayment) postgres.UpdateSubscriptionPaymentParams {
	return postgres.UpdateSubscriptionPaymentParams{
		ID:                    pgtype.UUID{Bytes: p.ID, Valid: true},
		PaymentMethodID:       pgconv.UUIDToPgtypePtr(p.PaymentMethodID),
		ProviderPaymentID:     pgconv.StringPtrToPgtype(p.ProviderPaymentID),
		PaymentUrl:            pgconv.StringPtrToPgtype(p.PaymentURL),
		Status:                string(p.Status),
		RefundedAmountKopecks: pgconv.Int8PtrToPgtype(p.RefundedAmountKopecks),
		// charge_attempts is a bounded retry counter (config limit is 3).
		ChargeAttempts: int32(p.ChargeAttempts), //nolint:gosec // small bounded counter, never overflows int32
		ErrorCode:      pgconv.StringPtrToPgtype(p.ErrorCode),
		SucceededAt:    pgconv.TimePtrToPgtype(p.SucceededAt),
	}
}

func mapSubscriptionPayment(row postgres.SubscriptionPayment) (domain.SubscriptionPayment, error) {
	return domain.ReconstituteSubscriptionPayment(domain.SubscriptionPayment{
		ID:                    pgconv.UUIDFromPgtype(row.ID),
		UserID:                pgconv.UUIDFromPgtype(row.UserID),
		SubscriptionID:        pgconv.UUIDFromPgtype(row.SubscriptionID),
		TariffID:              pgconv.UUIDFromPgtype(row.TariffID),
		PaymentMethodID:       pgconv.UUIDFromPgtypePtr(row.PaymentMethodID),
		Period:                domain.SubscriptionPeriod(row.Period),
		AmountKopecks:         row.AmountKopecks,
		Provider:              domain.PaymentProvider(row.Provider),
		ProviderPaymentID:     pgconv.TextToPtrString(row.ProviderPaymentID),
		PaymentURL:            pgconv.TextToPtrString(row.PaymentUrl),
		Status:                domain.PaymentStatus(row.Status),
		RefundedAmountKopecks: pgconv.Int8ToPtr(row.RefundedAmountKopecks),
		ChargeAttempts:        int(row.ChargeAttempts),
		ErrorCode:             pgconv.TextToPtrString(row.ErrorCode),
		CreatedAt:             pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:             pgconv.TimestamptzToTime(row.UpdatedAt),
		SucceededAt:           pgconv.TimestamptzToPtrTime(row.SucceededAt),
	})
}

func mapSubscriptionPayments(rows []postgres.SubscriptionPayment) ([]domain.SubscriptionPayment, error) {
	payments := make([]domain.SubscriptionPayment, 0, len(rows))
	for _, row := range rows {
		payment, err := mapSubscriptionPayment(row)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}
	return payments, nil
}
