package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PaymentMethodRepository persists payment methods.
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
func (r *PaymentMethodRepository) WithTx(tx transaction.Tx) application.PaymentMethodRepository {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		panic(fmt.Sprintf("billing.PaymentMethodRepository.WithTx: %T is not a postgres.DBTX", tx))
	}
	return NewPaymentMethodRepository(dbtx, r.encryptor)
}

// Create inserts a new payment method. The provider token is encrypted at rest
// before persistence.
func (r *PaymentMethodRepository) Create(ctx context.Context, pm domain.PaymentMethod) (domain.PaymentMethod, error) {
	encryptedToken, err := r.encryptor.Encrypt(ctx, pm.ProviderToken)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("encrypt provider token: %w", err)
	}

	row, err := r.q().CreatePaymentMethod(ctx, postgres.CreatePaymentMethodParams{
		UserID:        pgtype.UUID{Bytes: pm.UserID, Valid: true},
		Provider:      string(pm.Provider),
		ProviderToken: encryptedToken,
		TokenHash:     r.encryptor.HashToken(pm.ProviderToken),
		DisplayMask:   pgtype.Text{String: pm.DisplayMask, Valid: pm.DisplayMask != ""},
		IsActive:      pm.IsActive,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return domain.PaymentMethod{}, application.ErrPaymentMethodAlreadyExists
		}
		return domain.PaymentMethod{}, fmt.Errorf("create payment method: %w", err)
	}
	return mapPaymentMethod(ctx, row, r.encryptor)
}

// GetByID returns a payment method by ID.
func (r *PaymentMethodRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.PaymentMethod, error) {
	row, err := r.q().GetPaymentMethodByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PaymentMethod{}, application.ErrNotFound
		}
		return domain.PaymentMethod{}, fmt.Errorf("get payment method by id: %w", err)
	}
	return mapPaymentMethod(ctx, row, r.encryptor)
}

// ListByUserID returns all payment methods for a user ordered by creation date descending.
func (r *PaymentMethodRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	rows, err := r.q().ListPaymentMethodsByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list payment methods by user id: %w", err)
	}
	return mapPaymentMethods(ctx, rows, r.encryptor)
}

// SetActive deactivates all payment methods for the user and activates the given one.
// Callers must wrap the call in a transaction via repo.WithTx(tx) when atomicity
// is required.
func (r *PaymentMethodRepository) SetActive(ctx context.Context, userID, methodID uuid.UUID) error {
	if _, err := r.q().LockPaymentMethodsByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true}); err != nil {
		return fmt.Errorf("lock payment methods: %w", err)
	}

	pm, err := r.q().GetPaymentMethodByID(ctx, pgtype.UUID{Bytes: methodID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("get payment method: %w", err)
	}
	if uuid.UUID(pm.UserID.Bytes) != userID {
		return application.ErrNotFound
	}

	if err := r.q().DeactivateAllPaymentMethodsForUser(ctx, pgtype.UUID{Bytes: userID, Valid: true}); err != nil {
		return fmt.Errorf("deactivate payment methods for user: %w", err)
	}
	if _, err := r.q().UpdatePaymentMethodActiveByID(ctx, postgres.UpdatePaymentMethodActiveByIDParams{
		ID:       pgtype.UUID{Bytes: methodID, Valid: true},
		IsActive: true,
	}); err != nil {
		return fmt.Errorf("activate payment method: %w", err)
	}
	return nil
}

// Delete removes a payment method for the given user.
// Returns ErrPaymentMethodInUse if the method is referenced as the active payment
// method of a subscription.
func (r *PaymentMethodRepository) Delete(ctx context.Context, userID, methodID uuid.UUID) error {
	pm, err := r.q().GetPaymentMethodByIDForUpdate(ctx, pgtype.UUID{Bytes: methodID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("get payment method: %w", err)
	}
	if uuid.UUID(pm.UserID.Bytes) != userID {
		return application.ErrNotFound
	}

	if pm.IsActive {
		return application.ErrPaymentMethodInUse
	}

	count, err := r.q().CountSubscriptionsByActivePaymentMethodID(ctx, pgtype.UUID{Bytes: methodID, Valid: true})
	if err != nil {
		return fmt.Errorf("check active payment method usage: %w", err)
	}
	if count > 0 {
		return application.ErrPaymentMethodInUse
	}

	if err := r.q().DeletePaymentMethodByID(ctx, pgtype.UUID{Bytes: methodID, Valid: true}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
			return application.ErrPaymentMethodInUse
		}
		return fmt.Errorf("delete payment method: %w", err)
	}
	return nil
}

func mapPaymentMethod(ctx context.Context, row postgres.PaymentMethod, encryptor encryption.Encryptor) (domain.PaymentMethod, error) {
	providerToken, err := encryptor.Decrypt(ctx, row.ProviderToken)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("decrypt provider token: %w", err)
	}
	return domain.PaymentMethod{
		ID:            uuid.UUID(row.ID.Bytes),
		UserID:        uuid.UUID(row.UserID.Bytes),
		Provider:      domain.PaymentProvider(row.Provider),
		ProviderToken: providerToken,
		DisplayMask:   textString(row.DisplayMask),
		IsActive:      row.IsActive,
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}, nil
}

func mapPaymentMethods(ctx context.Context, rows []postgres.PaymentMethod, encryptor encryption.Encryptor) ([]domain.PaymentMethod, error) {
	result := make([]domain.PaymentMethod, len(rows))
	for i, row := range rows {
		pm, err := mapPaymentMethod(ctx, row, encryptor)
		if err != nil {
			return nil, err
		}
		result[i] = pm
	}
	return result, nil
}
