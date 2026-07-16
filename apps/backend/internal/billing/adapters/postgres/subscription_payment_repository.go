package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
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
func (r *SubscriptionPaymentRepository) WithTx(tx transaction.Tx) (application.SubscriptionPaymentRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("billing.SubscriptionPaymentRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewSubscriptionPaymentRepository(dbtx), nil
}

// Create inserts a new subscription payment.
func (r *SubscriptionPaymentRepository) Create(ctx context.Context, payment domain.SubscriptionPayment) (domain.SubscriptionPayment, error) {
	row, err := r.q().CreateSubscriptionPayment(ctx, mapCreateSubscriptionPaymentParams(payment))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return domain.SubscriptionPayment{}, application.ErrAlreadyExists
		}
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

// GetByIDAdmin returns a subscription payment by ID together with the user's phone.
func (r *SubscriptionPaymentRepository) GetByIDAdmin(ctx context.Context, id uuid.UUID) (application.SubscriptionPaymentWithUser, error) {
	row, err := r.q().GetSubscriptionPaymentByIDAdmin(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.SubscriptionPaymentWithUser{}, application.ErrNotFound
		}
		return application.SubscriptionPaymentWithUser{}, fmt.Errorf("get subscription payment by id admin: %w", err)
	}
	return mapSubscriptionPaymentWithUser(postgres.SubscriptionPayment{
		ID:                    row.ID,
		UserID:                row.UserID,
		SubscriptionID:        row.SubscriptionID,
		TariffID:              row.TariffID,
		PaymentMethodID:       row.PaymentMethodID,
		Period:                row.Period,
		AmountKopecks:         row.AmountKopecks,
		Provider:              row.Provider,
		ProviderPaymentID:     row.ProviderPaymentID,
		PaymentUrl:            row.PaymentUrl,
		Status:                row.Status,
		RefundedAmountKopecks: row.RefundedAmountKopecks,
		ChargeAttempts:        row.ChargeAttempts,
		ErrorCode:             row.ErrorCode,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
		SucceededAt:           row.SucceededAt,
	}, row.UserPhone), nil
}

// GetByIDForUpdate returns a subscription payment by ID, locking the row for update.
func (r *SubscriptionPaymentRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	row, err := r.q().GetSubscriptionPaymentByIDForUpdate(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SubscriptionPayment{}, application.ErrNotFound
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("get subscription payment by id for update: %w", err)
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

// ListPendingSubscriptionPaymentsByUserID returns pending subscription payments
// for a user ordered by creation date descending.
func (r *SubscriptionPaymentRepository) ListPendingSubscriptionPaymentsByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	rows, err := r.q().ListPendingSubscriptionPaymentsByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list pending subscription payments by user id: %w", err)
	}
	return mapSubscriptionPayments(rows), nil
}

// ListPendingUpgradePayments returns pending subscription payments whose tariff
// differs from the current subscription tariff, indicating an unfinished upgrade
// that may need to be reconciled with the provider.
func (r *SubscriptionPaymentRepository) ListPendingUpgradePayments(ctx context.Context, createdBefore time.Time, limit int32) ([]domain.SubscriptionPayment, error) {
	rows, err := r.q().ListPendingUpgradePayments(ctx, postgres.ListPendingUpgradePaymentsParams{
		CreatedAt: pgtype.Timestamptz{Time: createdBefore.UTC(), Valid: true},
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list pending upgrade payments: %w", err)
	}
	return mapSubscriptionPayments(rows), nil
}

// ListPendingPayments returns pending subscription payments older than the
// provided cutoff, ordered by creation time ascending, limited to the given
// number of rows. It is used by the reconciliation worker to find stuck
// payments that need to be checked with the provider.
func (r *SubscriptionPaymentRepository) ListPendingPayments(ctx context.Context, createdBefore time.Time, limit int32) ([]domain.SubscriptionPayment, error) {
	rows, err := r.q().ListPendingPayments(ctx, postgres.ListPendingPaymentsParams{
		CreatedAt: pgtype.Timestamptz{Time: createdBefore.UTC(), Valid: true},
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list pending payments: %w", err)
	}
	return mapSubscriptionPayments(rows), nil
}

// ListStaleRefundingPayments returns refunding subscription payments whose
// refund reservation was taken before the provided cutoff, ordered by the
// reservation time ascending, limited to the given number of rows. It is used
// by the reconciliation worker to find refunds that were neither finalized nor
// reverted and need to be resolved with the provider.
func (r *SubscriptionPaymentRepository) ListStaleRefundingPayments(ctx context.Context, updatedBefore time.Time, limit int32) ([]domain.SubscriptionPayment, error) {
	rows, err := r.q().ListStaleRefundingPayments(ctx, postgres.ListStaleRefundingPaymentsParams{
		UpdatedAt: pgtype.Timestamptz{Time: updatedBefore.UTC(), Valid: true},
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list stale refunding payments: %w", err)
	}
	return mapSubscriptionPayments(rows), nil
}

// ListAll returns all subscription payments for admin view.
func (r *SubscriptionPaymentRepository) ListAll(ctx context.Context, status string, userID uuid.UUID, limit, offset int) ([]application.SubscriptionPaymentWithUser, int64, error) {
	pgUserID := pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil}
	rows, err := r.q().ListSubscriptionPaymentsAdmin(ctx, postgres.ListSubscriptionPaymentsAdminParams{
		Status: status,
		UserID: pgUserID,
		//nolint:gosec // Limit and offset are validated by the HTTP layer.
		Limit: int32(limit),
		//nolint:gosec // Limit and offset are validated by the HTTP layer.
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list subscription payments admin: %w", err)
	}

	items := make([]application.SubscriptionPaymentWithUser, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapSubscriptionPaymentWithUser(postgres.SubscriptionPayment{
			ID:                    row.ID,
			UserID:                row.UserID,
			SubscriptionID:        row.SubscriptionID,
			TariffID:              row.TariffID,
			PaymentMethodID:       row.PaymentMethodID,
			Period:                row.Period,
			AmountKopecks:         row.AmountKopecks,
			Provider:              row.Provider,
			ProviderPaymentID:     row.ProviderPaymentID,
			PaymentUrl:            row.PaymentUrl,
			Status:                row.Status,
			RefundedAmountKopecks: row.RefundedAmountKopecks,
			ChargeAttempts:        row.ChargeAttempts,
			ErrorCode:             row.ErrorCode,
			CreatedAt:             row.CreatedAt,
			UpdatedAt:             row.UpdatedAt,
			SucceededAt:           row.SucceededAt,
		}, row.UserPhone))
	}

	total, err := r.q().CountSubscriptionPaymentsAdmin(ctx, postgres.CountSubscriptionPaymentsAdminParams{
		Status: status,
		UserID: pgUserID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count subscription payments admin: %w", err)
	}

	return items, total, nil
}

// GetLastSucceededBySubscriptionID returns the most recent succeeded payment
// for a subscription, or ErrNotFound if there is none.
func (r *SubscriptionPaymentRepository) GetLastSucceededBySubscriptionID(ctx context.Context, subscriptionID uuid.UUID) (domain.SubscriptionPayment, error) {
	row, err := r.q().GetLastSucceededSubscriptionPaymentBySubscriptionID(ctx, pgtype.UUID{Bytes: subscriptionID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SubscriptionPayment{}, application.ErrNotFound
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("get last succeeded subscription payment: %w", err)
	}
	return mapSubscriptionPayment(row), nil
}

// MarkSucceeded transitions a pending subscription payment to succeeded.
func (r *SubscriptionPaymentRepository) MarkSucceeded(ctx context.Context, id uuid.UUID, now time.Time) error {
	row, err := r.q().GetSubscriptionPaymentByIDForUpdate(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("get subscription payment: %w", err)
	}

	payment := mapSubscriptionPayment(row)
	if err := payment.MarkSucceeded(now); err != nil {
		return err
	}

	if _, err := r.q().MarkSubscriptionPaymentSucceeded(ctx, postgres.MarkSubscriptionPaymentSucceededParams{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		SucceededAt: pgconv.TimePtrToPgtype(payment.SucceededAt),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalidPaymentStatus
		}
		return fmt.Errorf("mark subscription payment succeeded: %w", err)
	}
	return nil
}

// MarkFailed transitions a pending subscription payment to failed and records the error code.
func (r *SubscriptionPaymentRepository) MarkFailed(ctx context.Context, id uuid.UUID, errorCode *string, now time.Time) error {
	row, err := r.q().GetSubscriptionPaymentByIDForUpdate(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("get subscription payment: %w", err)
	}

	payment := mapSubscriptionPayment(row)
	if err := payment.MarkFailed(errorCode, now); err != nil {
		return err
	}

	if _, err := r.q().MarkSubscriptionPaymentFailed(ctx, postgres.MarkSubscriptionPaymentFailedParams{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		ErrorCode: pgconv.StringPtrToPgtype(errorCode),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalidPaymentStatus
		}
		return fmt.Errorf("mark subscription payment failed: %w", err)
	}
	return nil
}

// MarkRefunded transitions a succeeded, pending, or refunding subscription payment
// to refunded, recording the full payment amount as refunded. Partial refunds are
// no longer initiated by the system.
func (r *SubscriptionPaymentRepository) MarkRefunded(ctx context.Context, id uuid.UUID, now time.Time) error {
	row, err := r.q().GetSubscriptionPaymentByIDForUpdate(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("get subscription payment: %w", err)
	}

	payment := mapSubscriptionPayment(row)
	if err := payment.MarkRefunded(now); err != nil {
		return err
	}

	if _, err := r.q().MarkSubscriptionPaymentRefunded(ctx, postgres.MarkSubscriptionPaymentRefundedParams{
		ID:                    pgtype.UUID{Bytes: id, Valid: true},
		Status:                string(payment.Status),
		RefundedAmountKopecks: pgconv.Int8PtrToPgtype(payment.RefundedAmountKopecks),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalidPaymentStatus
		}
		return fmt.Errorf("mark subscription payment refunded: %w", err)
	}
	return nil
}

// MarkReconciledSucceeded transitions a failed subscription payment to succeeded
// after an explicit provider-side status check.
func (r *SubscriptionPaymentRepository) MarkReconciledSucceeded(ctx context.Context, id uuid.UUID, now time.Time) error {
	row, err := r.q().GetSubscriptionPaymentByIDForUpdate(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("get subscription payment: %w", err)
	}

	payment := mapSubscriptionPayment(row)
	if err := payment.ReconcileToSucceeded(now); err != nil {
		return err
	}

	if _, err := r.q().MarkSubscriptionPaymentReconciledSucceeded(ctx, postgres.MarkSubscriptionPaymentReconciledSucceededParams{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		SucceededAt: pgconv.TimePtrToPgtype(payment.SucceededAt),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalidPaymentStatus
		}
		return fmt.Errorf("mark subscription payment reconciled succeeded: %w", err)
	}
	return nil
}

// MarkReconciledRefunded transitions a failed subscription payment to refunded
// after an explicit provider-side status check.
func (r *SubscriptionPaymentRepository) MarkReconciledRefunded(ctx context.Context, id uuid.UUID, now time.Time) error {
	row, err := r.q().GetSubscriptionPaymentByIDForUpdate(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("get subscription payment: %w", err)
	}

	payment := mapSubscriptionPayment(row)
	if err := payment.ReconcileToRefunded(now); err != nil {
		return err
	}

	if _, err := r.q().MarkSubscriptionPaymentReconciledRefunded(ctx, postgres.MarkSubscriptionPaymentReconciledRefundedParams{
		ID:                    pgtype.UUID{Bytes: id, Valid: true},
		RefundedAmountKopecks: pgconv.Int8PtrToPgtype(payment.RefundedAmountKopecks),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalidPaymentStatus
		}
		return fmt.Errorf("mark subscription payment reconciled refunded: %w", err)
	}
	return nil
}

// BeginRefund atomically reserves a succeeded or pending subscription payment for
// an in-flight refund by moving it to the refunding status. It returns
// domain.ErrInvalidPaymentStatus when the payment is not in a refundable state,
// which is what rejects concurrent refund attempts. updated_at is maintained by
// the table trigger, so the now argument is not written directly.
func (r *SubscriptionPaymentRepository) BeginRefund(ctx context.Context, id uuid.UUID, _ time.Time) error {
	tag, err := r.q().BeginSubscriptionPaymentRefund(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		return fmt.Errorf("begin subscription payment refund: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidPaymentStatus
	}
	return nil
}

// RevertRefund rolls back an in-flight refund reservation, restoring the payment
// to its previous status. It returns domain.ErrInvalidPaymentStatus when the
// payment is no longer in the refunding state. updated_at is maintained by the
// table trigger, so the now argument is not written directly.
func (r *SubscriptionPaymentRepository) RevertRefund(ctx context.Context, id uuid.UUID, prev domain.PaymentStatus, _ time.Time) error {
	tag, err := r.q().RevertSubscriptionPaymentRefund(ctx, postgres.RevertSubscriptionPaymentRefundParams{
		ID:     pgtype.UUID{Bytes: id, Valid: true},
		Status: string(prev),
	})
	if err != nil {
		return fmt.Errorf("revert subscription payment refund: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidPaymentStatus
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

// UpdatePaymentURL updates the provider payment URL of a subscription payment.
func (r *SubscriptionPaymentRepository) UpdatePaymentURL(ctx context.Context, id uuid.UUID, paymentURL string) (domain.SubscriptionPayment, error) {
	row, err := r.q().UpdateSubscriptionPaymentPaymentURL(ctx, postgres.UpdateSubscriptionPaymentPaymentURLParams{
		ID:         pgtype.UUID{Bytes: id, Valid: true},
		PaymentUrl: pgtype.Text{String: paymentURL, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SubscriptionPayment{}, application.ErrNotFound
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("update subscription payment confirm url: %w", err)
	}
	return mapSubscriptionPayment(row), nil
}

// UpdatePaymentMethodAndProviderID updates both the payment method and the
// provider payment ID of a subscription payment.
func (r *SubscriptionPaymentRepository) UpdatePaymentMethodAndProviderID(ctx context.Context, id, paymentMethodID uuid.UUID, providerPaymentID string) (domain.SubscriptionPayment, error) {
	row, err := r.q().UpdateSubscriptionPaymentMethodAndProviderID(ctx, postgres.UpdateSubscriptionPaymentMethodAndProviderIDParams{
		ID:                pgtype.UUID{Bytes: id, Valid: true},
		PaymentMethodID:   pgtype.UUID{Bytes: paymentMethodID, Valid: true},
		ProviderPaymentID: pgtype.Text{String: providerPaymentID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SubscriptionPayment{}, application.ErrNotFound
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("update subscription payment method and provider payment id: %w", err)
	}
	return mapSubscriptionPayment(row), nil
}

// UpdatePaymentMethodID updates only the payment method reference of a
// subscription payment. It is used when a pending renewal is reused but the
// active card has changed since the payment was created.
func (r *SubscriptionPaymentRepository) UpdatePaymentMethodID(ctx context.Context, id, paymentMethodID uuid.UUID) (domain.SubscriptionPayment, error) {
	row, err := r.q().UpdateSubscriptionPaymentMethodID(ctx, postgres.UpdateSubscriptionPaymentMethodIDParams{
		ID:              pgtype.UUID{Bytes: id, Valid: true},
		PaymentMethodID: pgtype.UUID{Bytes: paymentMethodID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SubscriptionPayment{}, application.ErrNotFound
		}
		return domain.SubscriptionPayment{}, fmt.Errorf("update subscription payment method id: %w", err)
	}
	return mapSubscriptionPayment(row), nil
}

// IncrementChargeAttempts atomically increments the renewal charge attempt
// counter of a subscription payment and returns the new value.
func (r *SubscriptionPaymentRepository) IncrementChargeAttempts(ctx context.Context, id uuid.UUID) (int, error) {
	attempts, err := r.q().IncrementSubscriptionPaymentChargeAttempts(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, application.ErrNotFound
		}
		return 0, fmt.Errorf("increment subscription payment charge attempts: %w", err)
	}
	return int(attempts), nil
}

func mapCreateSubscriptionPaymentParams(payment domain.SubscriptionPayment) postgres.CreateSubscriptionPaymentParams {
	return postgres.CreateSubscriptionPaymentParams{
		ID:                pgtype.UUID{Bytes: payment.ID, Valid: true},
		UserID:            pgtype.UUID{Bytes: payment.UserID, Valid: true},
		SubscriptionID:    pgtype.UUID{Bytes: payment.SubscriptionID, Valid: true},
		TariffID:          pgtype.UUID{Bytes: payment.TariffID, Valid: true},
		PaymentMethodID:   pgconv.UUIDToPgtypePtr(payment.PaymentMethodID),
		Period:            string(payment.Period),
		AmountKopecks:     payment.AmountKopecks,
		Provider:          string(payment.Provider),
		ProviderPaymentID: pgconv.StringPtrToPgtype(payment.ProviderPaymentID),
		PaymentUrl:        pgconv.StringPtrToPgtype(payment.PaymentURL),
		Status:            string(payment.Status),
		ErrorCode:         pgconv.StringPtrToPgtype(payment.ErrorCode),
	}
}

func mapSubscriptionPayment(row postgres.SubscriptionPayment) domain.SubscriptionPayment {
	payment := domain.SubscriptionPayment{
		ID:                    uuid.UUID(row.ID.Bytes),
		UserID:                uuid.UUID(row.UserID.Bytes),
		SubscriptionID:        uuid.UUID(row.SubscriptionID.Bytes),
		TariffID:              uuid.UUID(row.TariffID.Bytes),
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
		CreatedAt:             row.CreatedAt.Time,
		UpdatedAt:             row.UpdatedAt.Time,
	}
	if row.SucceededAt.Valid {
		payment.SucceededAt = &row.SucceededAt.Time
	}
	return payment
}

func mapSubscriptionPayments(rows []postgres.SubscriptionPayment) []domain.SubscriptionPayment {
	result := make([]domain.SubscriptionPayment, len(rows))
	for i, row := range rows {
		result[i] = mapSubscriptionPayment(row)
	}
	return result
}

func mapSubscriptionPaymentWithUser(payment postgres.SubscriptionPayment, phone string) application.SubscriptionPaymentWithUser {
	return application.SubscriptionPaymentWithUser{
		Payment:   mapSubscriptionPayment(payment),
		UserPhone: phone,
	}
}
