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

// SubscriptionPaymentRepository persists subscription payments.
type SubscriptionPaymentRepository struct {
	db postgres.DBTX
}

// NewSubscriptionPaymentRepository creates a new subscription payment repository.
func NewSubscriptionPaymentRepository(db postgres.DBTX) *SubscriptionPaymentRepository {
	return &SubscriptionPaymentRepository{db: db}
}

func (r *SubscriptionPaymentRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *SubscriptionPaymentRepository) WithTx(tx transaction.Tx) application.SubscriptionPaymentRepository {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return &invalidSubscriptionPaymentRepository{tx: tx}
	}
	return NewSubscriptionPaymentRepository(dbtx)
}

// invalidSubscriptionPaymentRepository returns a clear error for every method when an
// unsupported transaction type is passed to WithTx.
type invalidSubscriptionPaymentRepository struct {
	tx transaction.Tx
}

func (r *invalidSubscriptionPaymentRepository) Create(ctx context.Context, payment domain.SubscriptionPayment) (domain.SubscriptionPayment, error) {
	return domain.SubscriptionPayment{}, fmt.Errorf("billing: unsupported transaction type %T for SubscriptionPaymentRepository.Create", r.tx)
}

func (r *invalidSubscriptionPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	return domain.SubscriptionPayment{}, fmt.Errorf("billing: unsupported transaction type %T for SubscriptionPaymentRepository.GetByID", r.tx)
}

func (r *invalidSubscriptionPaymentRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	return nil, fmt.Errorf("billing: unsupported transaction type %T for SubscriptionPaymentRepository.ListByUserID", r.tx)
}

func (r *invalidSubscriptionPaymentRepository) MarkSucceeded(ctx context.Context, id uuid.UUID) error {
	return fmt.Errorf("billing: unsupported transaction type %T for SubscriptionPaymentRepository.MarkSucceeded", r.tx)
}

func (r *invalidSubscriptionPaymentRepository) MarkFailed(ctx context.Context, id uuid.UUID, errorCode *string) error {
	return fmt.Errorf("billing: unsupported transaction type %T for SubscriptionPaymentRepository.MarkFailed", r.tx)
}

func (r *invalidSubscriptionPaymentRepository) UpdateProviderPaymentID(ctx context.Context, id uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error) {
	return domain.SubscriptionPayment{}, fmt.Errorf("billing: unsupported transaction type %T for SubscriptionPaymentRepository.UpdateProviderPaymentID", r.tx)
}

func (r *invalidSubscriptionPaymentRepository) WithTx(tx transaction.Tx) application.SubscriptionPaymentRepository {
	return r
}

// Create inserts a new subscription payment.
func (r *SubscriptionPaymentRepository) Create(ctx context.Context, payment domain.SubscriptionPayment) (domain.SubscriptionPayment, error) {
	row, err := r.q().CreateSubscriptionPayment(ctx, mapCreateSubscriptionPaymentParams(payment))
	if err != nil {
		return domain.SubscriptionPayment{}, fmt.Errorf("create subscription payment: %w", err)
	}
	return mapSubscriptionPayment(row), nil
}

// GetByID returns a subscription payment by ID.
func (r *SubscriptionPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	row, err := r.q().GetSubscriptionPaymentByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SubscriptionPayment{}, application.ErrNotFound
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("get subscription payment by id: %w", err)
	}
	return mapSubscriptionPayment(row), nil
}

// ListByUserID returns all subscription payments for a user ordered by creation date descending.
func (r *SubscriptionPaymentRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	rows, err := r.q().ListSubscriptionPaymentsByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list subscription payments by user id: %w", err)
	}
	return mapSubscriptionPayments(rows), nil
}

// MarkSucceeded transitions a pending subscription payment to succeeded.
func (r *SubscriptionPaymentRepository) MarkSucceeded(ctx context.Context, id uuid.UUID) error {
	row, err := r.q().GetSubscriptionPaymentByIDForUpdate(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("get subscription payment: %w", err)
	}

	payment := mapSubscriptionPayment(row)
	if err := payment.MarkSucceeded(); err != nil {
		return err
	}

	if _, err := r.q().MarkSubscriptionPaymentSucceeded(ctx, postgres.MarkSubscriptionPaymentSucceededParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: payment.UpdatedAt, Valid: true},
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalidPaymentStatus
		}
		return fmt.Errorf("mark subscription payment succeeded: %w", err)
	}
	return nil
}

// MarkFailed transitions a pending subscription payment to failed and records the error code.
func (r *SubscriptionPaymentRepository) MarkFailed(ctx context.Context, id uuid.UUID, errorCode *string) error {
	row, err := r.q().GetSubscriptionPaymentByIDForUpdate(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("get subscription payment: %w", err)
	}

	payment := mapSubscriptionPayment(row)
	if err := payment.MarkFailed(errorCode); err != nil {
		return err
	}

	if _, err := r.q().MarkSubscriptionPaymentFailed(ctx, postgres.MarkSubscriptionPaymentFailedParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		ErrorCode: textPtr(errorCode),
		UpdatedAt: pgtype.Timestamptz{Time: payment.UpdatedAt, Valid: true},
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalidPaymentStatus
		}
		return fmt.Errorf("mark subscription payment failed: %w", err)
	}
	return nil
}

// UpdateProviderPaymentID updates the provider payment ID of a subscription payment.
func (r *SubscriptionPaymentRepository) UpdateProviderPaymentID(ctx context.Context, id uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error) {
	row, err := r.q().UpdateSubscriptionPaymentProviderPaymentID(ctx, postgres.UpdateSubscriptionPaymentProviderPaymentIDParams{
		ID:                pgtype.UUID{Bytes: id, Valid: true},
		ProviderPaymentID: pgtype.Text{String: providerPaymentID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SubscriptionPayment{}, application.ErrNotFound
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("update subscription payment provider payment id: %w", err)
	}
	return mapSubscriptionPayment(row), nil
}

func mapCreateSubscriptionPaymentParams(payment domain.SubscriptionPayment) postgres.CreateSubscriptionPaymentParams {
	return postgres.CreateSubscriptionPaymentParams{
		UserID:            pgtype.UUID{Bytes: payment.UserID, Valid: true},
		SubscriptionID:    pgtype.UUID{Bytes: payment.SubscriptionID, Valid: true},
		TariffID:          pgtype.UUID{Bytes: payment.TariffID, Valid: true},
		PaymentMethodID:   uuidPtr(payment.PaymentMethodID),
		Period:            string(payment.Period),
		AmountKopecks:     payment.AmountKopecks,
		Provider:          string(payment.Provider),
		ProviderPaymentID: textPtr(payment.ProviderPaymentID),
		Status:            string(payment.Status),
		ErrorCode:         textPtr(payment.ErrorCode),
	}
}

func mapSubscriptionPayment(row postgres.SubscriptionPayment) domain.SubscriptionPayment {
	return domain.SubscriptionPayment{
		ID:                uuid.UUID(row.ID.Bytes),
		UserID:            uuid.UUID(row.UserID.Bytes),
		SubscriptionID:    uuid.UUID(row.SubscriptionID.Bytes),
		TariffID:          uuid.UUID(row.TariffID.Bytes),
		PaymentMethodID:   uuidPtrFromPgtype(row.PaymentMethodID),
		Period:            domain.SubscriptionPeriod(row.Period),
		AmountKopecks:     row.AmountKopecks,
		Provider:          domain.PaymentProvider(row.Provider),
		ProviderPaymentID: stringPtrFromPgtype(row.ProviderPaymentID),
		Status:            domain.PaymentStatus(row.Status),
		ErrorCode:         stringPtrFromPgtype(row.ErrorCode),
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

func mapSubscriptionPayments(rows []postgres.SubscriptionPayment) []domain.SubscriptionPayment {
	result := make([]domain.SubscriptionPayment, len(rows))
	for i, row := range rows {
		result[i] = mapSubscriptionPayment(row)
	}
	return result
}
