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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/pgerr"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// SubscriptionPaymentRepository persists subscription payments. The encryptor
// resolves the payer's phone for the admin views: the users table stores
// either plaintext (phone_encrypted = false) or ciphertext, and the phone
// filter has to match both forms.
type SubscriptionPaymentRepository struct {
	db  postgres.DBTX
	enc encryption.Encryptor
}

// NewSubscriptionPaymentRepository creates a new payment repository. A nil
// encryptor keeps the payment aggregate methods fully functional; only the
// admin listing of encrypted phones fails fast.
func NewSubscriptionPaymentRepository(db postgres.DBTX, enc encryption.Encryptor) *SubscriptionPaymentRepository {
	return &SubscriptionPaymentRepository{db: db, enc: enc}
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
	return NewSubscriptionPaymentRepository(dbtx, r.enc), nil
}

// Create inserts a new payment. A unique violation on the pending-payments
// partial index is narrowed to ErrAlreadyExists: a concurrent request won the
// initiation race and the caller returns the existing pending payment.
func (r *SubscriptionPaymentRepository) Create(
	ctx context.Context, payment domain.SubscriptionPayment,
) (domain.SubscriptionPayment, error) {
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

// List returns a batch of payments matching the worker selection (issue #286):
// the parameterized query every reconciliation phase lists its batch through.
// The predicate lives here, in SQL.
func (r *SubscriptionPaymentRepository) List(ctx context.Context, sel application.PaymentSelection) ([]domain.SubscriptionPayment, error) {
	rows, err := r.q().ListSubscriptionPaymentsBySelection(ctx, postgres.ListSubscriptionPaymentsBySelectionParams{
		Status:           string(sel.Status),
		CreatedBefore:    pgconv.TimePtrToPgtype(sel.CreatedBefore),
		UpdatedBefore:    pgconv.TimePtrToPgtype(sel.UpdatedBefore),
		TariffChangeOnly: sel.TariffChangeOnly,
		BatchLimit:       batchLimit(sel.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list subscription payments by selection: %w", err)
	}
	return mapSubscriptionPayments(rows)
}

// adminPaymentQueryParams maps the validated application filters to the SQL
// parameters. The phone filter is encrypted deterministically so it matches
// the stored ciphertext; a plaintext row still matches its own form.
func (r *SubscriptionPaymentRepository) adminPaymentQueryParams(
	ctx context.Context, filters application.AdminPaymentFilters,
) (postgres.ListSubscriptionPaymentsAdminParams, error) {
	params := postgres.ListSubscriptionPaymentsAdminParams{
		Status:             filters.Status,
		UserPhone:          filters.UserPhone,
		SubscriptionStatus: filters.SubscriptionStatus,
		Sort:               filters.Sort,
		Order:              filters.Order,
		Offset:             shared.ToInt32Clamped(filters.Offset),
		Limit:              shared.ToInt32Clamped(filters.Limit),
	}
	if filters.UserID != nil {
		params.UserID = pgtype.UUID{Bytes: *filters.UserID, Valid: true}
	}
	if filters.UserPhone != "" {
		encrypted, err := r.enc.DeterministicEncrypt(ctx, filters.UserPhone)
		if err != nil {
			return params, fmt.Errorf("encrypt phone filter: %w", err)
		}
		params.UserPhoneEnc = encrypted
	}
	return params, nil
}

// ListAdminPayments implements the admin payment listing (issue #254): every
// user's payments joined with the payer's phone, filtered and sorted by the
// validated filters, with the total count of the filtered set.
func (r *SubscriptionPaymentRepository) ListAdminPayments(
	ctx context.Context, filters application.AdminPaymentFilters,
) ([]application.AdminPaymentRow, int64, error) {
	params, err := r.adminPaymentQueryParams(ctx, filters)
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q().CountSubscriptionPaymentsAdmin(ctx, postgres.CountSubscriptionPaymentsAdminParams{
		UserID:             params.UserID,
		Status:             params.Status,
		UserPhone:          params.UserPhone,
		UserPhoneEnc:       params.UserPhoneEnc,
		SubscriptionStatus: params.SubscriptionStatus,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count admin subscription payments: %w", err)
	}
	rows, err := r.q().ListSubscriptionPaymentsAdmin(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin subscription payments: %w", err)
	}
	mapped, err := mapAdminPaymentRows(ctx, r, rows)
	if err != nil {
		return nil, 0, err
	}
	return mapped, total, nil
}

// GetAdminPayment implements the single-payment admin read (issue #254).
func (r *SubscriptionPaymentRepository) GetAdminPayment(ctx context.Context, paymentID uuid.UUID) (application.AdminPaymentRow, error) {
	row, err := r.q().GetSubscriptionPaymentAdmin(ctx, pgtype.UUID{Bytes: paymentID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.AdminPaymentRow{}, application.ErrNotFound
		}
		return application.AdminPaymentRow{}, fmt.Errorf("get admin subscription payment: %w", err)
	}
	payment, err := mapSubscriptionPaymentFromColumns(
		row.ID, row.UserID, row.SubscriptionID, row.TariffID, row.PaymentMethodID,
		row.Period, row.AmountKopecks, row.Provider, row.ProviderPaymentID, row.PaymentUrl,
		row.Status, row.RefundedAmountKopecks, row.ChargeAttempts, row.ErrorCode,
		row.CreatedAt, row.UpdatedAt, row.SucceededAt,
	)
	if err != nil {
		return application.AdminPaymentRow{}, err
	}
	phone, err := r.resolveUserPhone(ctx, row.UserPhone, row.UserPhoneEncrypted)
	if err != nil {
		return application.AdminPaymentRow{}, err
	}
	return application.AdminPaymentRow{Payment: payment, UserPhone: phone}, nil
}

// mapAdminPaymentRows maps the admin listing rows to the application shape,
// decrypting the payer's phone where it is stored as ciphertext.
func mapAdminPaymentRows(
	ctx context.Context, r *SubscriptionPaymentRepository, rows []postgres.ListSubscriptionPaymentsAdminRow,
) ([]application.AdminPaymentRow, error) {
	result := make([]application.AdminPaymentRow, 0, len(rows))
	for _, row := range rows {
		payment, err := mapSubscriptionPaymentFromColumns(
			row.ID, row.UserID, row.SubscriptionID, row.TariffID, row.PaymentMethodID,
			row.Period, row.AmountKopecks, row.Provider, row.ProviderPaymentID, row.PaymentUrl,
			row.Status, row.RefundedAmountKopecks, row.ChargeAttempts, row.ErrorCode,
			row.CreatedAt, row.UpdatedAt, row.SucceededAt,
		)
		if err != nil {
			return nil, err
		}
		phone, err := r.resolveUserPhone(ctx, row.UserPhone, row.UserPhoneEncrypted)
		if err != nil {
			return nil, err
		}
		result = append(result, application.AdminPaymentRow{Payment: payment, UserPhone: phone})
	}
	return result, nil
}

// resolveUserPhone decrypts the payer's phone when it is stored as
// ciphertext; a plaintext row (phone_encrypted = false) passes through.
func (r *SubscriptionPaymentRepository) resolveUserPhone(ctx context.Context, phone string, encrypted bool) (string, error) {
	if !encrypted {
		return phone, nil
	}
	if r.enc == nil {
		return "", errors.New("billing.SubscriptionPaymentRepository: encrypted phone cannot be resolved without an encryptor")
	}
	decrypted, err := r.enc.Decrypt(ctx, phone)
	if err != nil {
		return "", fmt.Errorf("decrypt phone: %w", err)
	}
	return decrypted, nil
}

// mapSubscriptionPaymentFromColumns maps the shared payment column set of the
// admin queries through the canonical reconstitution.
func mapSubscriptionPaymentFromColumns(
	id, userID, subscriptionID, tariffID pgtype.UUID,
	paymentMethodID pgtype.UUID,
	period string,
	amountKopecks int64,
	provider string,
	providerPaymentID, paymentURL pgtype.Text,
	status string,
	refundedAmountKopecks pgtype.Int8,
	chargeAttempts int32,
	errorCode pgtype.Text,
	createdAt, updatedAt pgtype.Timestamptz,
	succeededAt pgtype.Timestamptz,
) (domain.SubscriptionPayment, error) {
	return mapSubscriptionPayment(postgres.SubscriptionPayment{
		ID:                    id,
		UserID:                userID,
		SubscriptionID:        subscriptionID,
		TariffID:              tariffID,
		PaymentMethodID:       paymentMethodID,
		Period:                period,
		AmountKopecks:         amountKopecks,
		Provider:              provider,
		ProviderPaymentID:     providerPaymentID,
		PaymentUrl:            paymentURL,
		Status:                status,
		RefundedAmountKopecks: refundedAmountKopecks,
		ChargeAttempts:        chargeAttempts,
		ErrorCode:             errorCode,
		CreatedAt:             createdAt,
		UpdatedAt:             updatedAt,
		SucceededAt:           succeededAt,
	})
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
		// The charge_attempts counter is bounded (config limit is 3).
		ChargeAttempts: shared.ToInt32Clamped(p.ChargeAttempts),
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
		// The charge_attempts counter is bounded (config limit is 3).
		ChargeAttempts: shared.ToInt32Clamped(p.ChargeAttempts),
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
