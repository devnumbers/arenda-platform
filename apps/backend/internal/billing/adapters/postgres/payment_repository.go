// Package postgres holds the billing persistence adapters: repositories for tariffs, subscriptions, payments,
// payment methods, card binding sessions and the subscription transition log.
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

// onePendingFormIndex is the user-level partial unique index backing the
// one-pending-form-per-user invariant (issue #690): at most one pending
// payment with a payer form per user, whatever the tariff and period.
const onePendingFormIndex = "idx_subscription_payments_one_pending_form"

// Create inserts a new payment. Unique violations are narrowed by index: the
// user-level form index means a concurrent initiation's live form already
// holds the user's slot — the pending-exists conflict; any other index (the
// same-target pending backstop, the provider reference) means a duplicate
// initiation the caller resolves to the existing pending payment.
func (r *SubscriptionPaymentRepository) Create(
	ctx context.Context, payment domain.SubscriptionPayment,
) (domain.SubscriptionPayment, error) {
	row, err := r.q().CreateSubscriptionPayment(ctx, mapCreatePaymentParams(payment))
	if err != nil {
		if pgerr.IsUniqueViolationOnConstraint(err, onePendingFormIndex) {
			return domain.SubscriptionPayment{}, application.ErrPendingPaymentExists
		}
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

// ListByUserIDWithCard returns the user's payments, newest first, each with
// the masked card it was charged with resolved for display (issue #619): the
// payment's own snapshot first, the bound method's mask as the fallback for
// payments created before the snapshot existed.
func (r *SubscriptionPaymentRepository) ListByUserIDWithCard(
	ctx context.Context, userID uuid.UUID,
) ([]application.SubscriptionPaymentWithCard, error) {
	rows, err := r.q().ListSubscriptionPaymentsWithCardByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list subscription payments with card: %w", err)
	}
	result := make([]application.SubscriptionPaymentWithCard, 0, len(rows))
	for _, row := range rows {
		payment, err := mapSubscriptionPaymentFromColumns(
			row.ID, row.UserID, row.SubscriptionID, row.TariffID, row.PaymentMethodID,
			row.Period, row.AmountKopecks, row.Provider, row.ProviderPaymentID, row.PaymentUrl,
			row.Status, row.RefundedAmountKopecks, row.ChargeAttempts, row.ErrorCode,
			row.CreatedAt, row.UpdatedAt, row.SucceededAt, row.ExpiresAt, row.CardMask,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, application.SubscriptionPaymentWithCard{
			Payment:          payment,
			ResolvedCardMask: pgconv.TextToPtrString(row.ResolvedCardMask),
		})
	}
	return result, nil
}

// ListByUserID returns the user's payments without the resolved card — the
// inspection convenience over ListByUserIDWithCard.
func (r *SubscriptionPaymentRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.SubscriptionPayment, error) {
	rows, err := r.ListByUserIDWithCard(ctx, userID)
	if err != nil {
		return nil, err
	}
	payments := make([]domain.SubscriptionPayment, 0, len(rows))
	for _, row := range rows {
		payments = append(payments, row.Payment)
	}
	return payments, nil
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

// ListExpiredPending returns a batch of still-pending payments whose form
// deadline ran out (issue #616): the TTL-expiry phase locks and fails each
// row in its own transaction. No provider reference is required — a crashed
// initiation must expire too.
func (r *SubscriptionPaymentRepository) ListExpiredPending(
	ctx context.Context, before time.Time, limit int,
) ([]domain.SubscriptionPayment, error) {
	rows, err := r.q().ListExpiredPendingSubscriptionPayments(ctx, postgres.ListExpiredPendingSubscriptionPaymentsParams{
		ExpiresAt: pgconv.TimePtrToPgtype(&before),
		Limit:     batchLimit(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list expired pending subscription payments: %w", err)
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

// Count returns how many payments match the worker selection ignoring its
// limit (ticket #433): the stuck-payment gauges of the hygiene phase count
// the whole batch. The predicate mirrors List's one in SQL.
func (r *SubscriptionPaymentRepository) Count(ctx context.Context, sel application.PaymentSelection) (int64, error) {
	count, err := r.q().CountSubscriptionPaymentsBySelection(ctx, postgres.CountSubscriptionPaymentsBySelectionParams{
		Status:           string(sel.Status),
		CreatedBefore:    pgconv.TimePtrToPgtype(sel.CreatedBefore),
		UpdatedBefore:    pgconv.TimePtrToPgtype(sel.UpdatedBefore),
		TariffChangeOnly: sel.TariffChangeOnly,
	})
	if err != nil {
		return 0, fmt.Errorf("count subscription payments by selection: %w", err)
	}
	return count, nil
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
		row.CreatedAt, row.UpdatedAt, row.SucceededAt, row.ExpiresAt, row.CardMask,
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
			row.CreatedAt, row.UpdatedAt, row.SucceededAt, row.ExpiresAt, row.CardMask,
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
	succeededAt, expiresAt pgtype.Timestamptz,
	cardMask pgtype.Text,
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
		ExpiresAt:             expiresAt,
		CardMask:              cardMask,
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

// ShiftCreatedAt moves created_at of every payment of the subscription by the
// signed delta (issue #665) — the stand-only time travel that keeps the
// dunning retry predicate's relative order (payments counted from the
// grace-entry anchor) invariant under the shift. Runs inside the caller's
// transaction. Returns how many rows moved.
func (r *SubscriptionPaymentRepository) ShiftCreatedAt(
	ctx context.Context, subscriptionID uuid.UUID, delta time.Duration,
) (int64, error) {
	moved, err := r.q().ShiftSubscriptionPaymentsCreatedAt(ctx, postgres.ShiftSubscriptionPaymentsCreatedAtParams{
		DeltaSeconds:   delta.Seconds(),
		SubscriptionID: pgtype.UUID{Bytes: subscriptionID, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("shift subscription payments created_at: %w", err)
	}
	return moved, nil
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
		ExpiresAt:      pgconv.TimePtrToPgtype(p.ExpiresAt),
		CardMask:       pgconv.StringPtrToPgtype(p.CardMask),
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
		ExpiresAt:      pgconv.TimePtrToPgtype(p.ExpiresAt),
		CardMask:       pgconv.StringPtrToPgtype(p.CardMask),
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
		ExpiresAt:             pgconv.TimestamptzToPtrTime(row.ExpiresAt),
		CardMask:              pgconv.TextToPtrString(row.CardMask),
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
