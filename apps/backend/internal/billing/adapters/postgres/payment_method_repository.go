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
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/pgerr"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PaymentMethodRepository persists payment methods (issue #251). The charge
// token, provider card id and expiry date are encrypted at rest; duplicate
// detection runs on the HMAC token hash, so the plaintext token never sits in
// an index.
type PaymentMethodRepository struct {
	db        postgres.DBTX
	encryptor encryption.Encryptor
}

// NewPaymentMethodRepository creates a new payment method repository.
func NewPaymentMethodRepository(db postgres.DBTX, encryptor encryption.Encryptor) *PaymentMethodRepository {
	return &PaymentMethodRepository{db: db, encryptor: encryptor}
}

func (r *PaymentMethodRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *PaymentMethodRepository) WithTx(tx transaction.Tx) (application.PaymentMethodRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("billing.PaymentMethodRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewPaymentMethodRepository(dbtx, r.encryptor), nil
}

// UpsertByTokenHash inserts the method or converges on the row with the same
// (user_id, token_hash): a re-bound card updates its token and display fields
// instead of duplicating. Sensitive fields are encrypted before persistence.
func (r *PaymentMethodRepository) UpsertByTokenHash(ctx context.Context, method domain.PaymentMethod) (domain.PaymentMethod, error) {
	params, err := r.upsertParams(ctx, method)
	if err != nil {
		return domain.PaymentMethod{}, err
	}
	row, err := r.q().UpsertPaymentMethodByTokenHash(ctx, params)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("upsert payment method by token hash: %w", err)
	}
	return r.mapMethod(ctx, row)
}

// GetByID returns a payment method by its identifier.
func (r *PaymentMethodRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.PaymentMethod, error) {
	row, err := r.q().GetPaymentMethodByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PaymentMethod{}, application.ErrNotFound
		}
		return domain.PaymentMethod{}, fmt.Errorf("get payment method by id: %w", err)
	}
	return r.mapMethod(ctx, row)
}

// GetByIDForUpdate returns a payment method by its identifier, locking the
// row for update. Must only be called inside a transaction.
func (r *PaymentMethodRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.PaymentMethod, error) {
	row, err := r.q().GetPaymentMethodByIDForUpdate(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PaymentMethod{}, application.ErrNotFound
		}
		return domain.PaymentMethod{}, fmt.Errorf("get payment method for update: %w", err)
	}
	return r.mapMethod(ctx, row)
}

// ListByUserID returns the user's payment methods, newest first.
func (r *PaymentMethodRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	rows, err := r.q().ListPaymentMethodsByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list payment methods by user id: %w", err)
	}
	methods := make([]domain.PaymentMethod, 0, len(rows))
	for _, row := range rows {
		method, err := r.mapMethod(ctx, row)
		if err != nil {
			return nil, err
		}
		methods = append(methods, method)
	}
	return methods, nil
}

// SetActive makes the method the user's single active one: the user's rows
// are locked first so concurrent switches serialize (the one-active partial
// unique index would otherwise reject the second committer), then every other
// method is deactivated and this one activated. Must run inside a
// transaction.
func (r *PaymentMethodRepository) SetActive(ctx context.Context, userID, methodID uuid.UUID) error {
	if _, err := r.q().LockPaymentMethodsByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true}); err != nil {
		return fmt.Errorf("lock payment methods: %w", err)
	}
	if err := r.q().DeactivateAllPaymentMethodsForUser(ctx, pgtype.UUID{Bytes: userID, Valid: true}); err != nil {
		return fmt.Errorf("deactivate payment methods for user: %w", err)
	}
	if _, err := r.q().UpdatePaymentMethodActiveByID(ctx, postgres.UpdatePaymentMethodActiveByIDParams{
		ID:       pgtype.UUID{Bytes: methodID, Valid: true},
		IsActive: true,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("activate payment method: %w", err)
	}
	return nil
}

// Delete removes the user's payment method. The
// active_payment_method_id FK (ON DELETE RESTRICT) is the durable backstop of
// the "active method cannot be deleted" rule: a still-referenced method
// surfaces as ErrPaymentMethodInUse instead of disappearing under the
// subscription.
func (r *PaymentMethodRepository) Delete(ctx context.Context, userID, methodID uuid.UUID) error {
	if err := r.q().DeletePaymentMethodByID(ctx, postgres.DeletePaymentMethodByIDParams{
		ID:     pgtype.UUID{Bytes: methodID, Valid: true},
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
	}); err != nil {
		if pgerr.IsForeignKeyViolation(err) {
			return application.ErrPaymentMethodInUse
		}
		return fmt.Errorf("delete payment method: %w", err)
	}
	return nil
}

// upsertParams encrypts the sensitive fields, derives the token hash and
// builds the upsert parameters.
func (r *PaymentMethodRepository) upsertParams(ctx context.Context, method domain.PaymentMethod) (postgres.UpsertPaymentMethodByTokenHashParams, error) {
	token, err := r.encryptor.Encrypt(ctx, method.ProviderToken)
	if err != nil {
		return postgres.UpsertPaymentMethodByTokenHashParams{}, fmt.Errorf("encrypt provider token: %w", err)
	}
	cardID, err := encryptOptional(ctx, r.encryptor, method.ProviderCardID)
	if err != nil {
		return postgres.UpsertPaymentMethodByTokenHashParams{}, fmt.Errorf("encrypt provider card id: %w", err)
	}
	expDate, err := encryptOptional(ctx, r.encryptor, method.ExpDate)
	if err != nil {
		return postgres.UpsertPaymentMethodByTokenHashParams{}, fmt.Errorf("encrypt expiry date: %w", err)
	}
	return postgres.UpsertPaymentMethodByTokenHashParams{
		ID:             pgtype.UUID{Bytes: method.ID, Valid: true},
		UserID:         pgtype.UUID{Bytes: method.UserID, Valid: true},
		Provider:       string(method.Provider),
		ProviderToken:  token,
		TokenHash:      r.encryptor.HashToken(method.ProviderToken),
		DisplayMask:    pgtype.Text{String: method.DisplayMask, Valid: method.DisplayMask != ""},
		ProviderCardID: pgtype.Text{String: cardID, Valid: cardID != ""},
		ExpDate:        pgtype.Text{String: expDate, Valid: expDate != ""},
		IsActive:       method.IsActive,
	}, nil
}

func (r *PaymentMethodRepository) mapMethod(ctx context.Context, row postgres.PaymentMethod) (domain.PaymentMethod, error) {
	providerToken, err := r.encryptor.Decrypt(ctx, row.ProviderToken)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("decrypt provider token: %w", err)
	}
	providerCardID, err := decryptOptional(ctx, r.encryptor, pgconv.TextToString(row.ProviderCardID))
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("decrypt provider card id: %w", err)
	}
	expDate, err := decryptOptional(ctx, r.encryptor, pgconv.TextToString(row.ExpDate))
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("decrypt expiry date: %w", err)
	}
	return domain.ReconstitutePaymentMethod(domain.PaymentMethod{
		ID:             pgconv.UUIDFromPgtype(row.ID),
		UserID:         pgconv.UUIDFromPgtype(row.UserID),
		Provider:       domain.PaymentProvider(row.Provider),
		ProviderToken:  providerToken,
		ProviderCardID: providerCardID,
		DisplayMask:    pgconv.TextToString(row.DisplayMask),
		ExpDate:        expDate,
		IsActive:       row.IsActive,
		CreatedAt:      pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:      pgconv.TimestamptzToTime(row.UpdatedAt),
	})
}

func encryptOptional(ctx context.Context, encryptor encryption.Encryptor, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	return encryptor.Encrypt(ctx, plaintext)
}

func decryptOptional(ctx context.Context, encryptor encryption.Encryptor, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	return encryptor.Decrypt(ctx, ciphertext)
}
