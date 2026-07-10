package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	pgen "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// AttemptRepository persists login attempt windows.
type AttemptRepository struct {
	db  pgen.DBTX
	enc encryption.Encryptor
}

// NewAttemptRepository creates a new attempt repository.
func NewAttemptRepository(db pgen.DBTX, enc encryption.Encryptor) *AttemptRepository {
	return &AttemptRepository{db: db, enc: enc}
}

func (r *AttemptRepository) q() *pgen.Queries {
	return pgen.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *AttemptRepository) WithTx(tx transaction.Tx) (application.AttemptRepository, error) {
	dbtx, ok := tx.(pgen.DBTX)
	if !ok {
		return nil, fmt.Errorf("identity.AttemptRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewAttemptRepository(dbtx, r.enc), nil
}

func (r *AttemptRepository) GetByPhone(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.AttemptWindow{}, err
	}
	row, err := r.q().GetLoginAttemptByPhone(ctx, encryptedPhone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AttemptWindow{}, application.ErrNotFound
		}
		return domain.AttemptWindow{}, fmt.Errorf("get login attempt by phone: %w", err)
	}
	return domain.AttemptWindow{
		Failures:       int(row.Failures),
		FirstFailureAt: row.FirstFailureAt.Time,
		LastFailureAt:  row.LastFailureAt.Time,
	}, nil
}

func (r *AttemptRepository) GetByPhoneForUpdate(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.AttemptWindow{}, err
	}
	row, err := r.q().GetLoginAttemptByPhoneForUpdate(ctx, encryptedPhone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AttemptWindow{}, application.ErrNotFound
		}
		return domain.AttemptWindow{}, fmt.Errorf("get login attempt by phone for update: %w", err)
	}
	return domain.AttemptWindow{
		Failures:       int(row.Failures),
		FirstFailureAt: row.FirstFailureAt.Time,
		LastFailureAt:  row.LastFailureAt.Time,
	}, nil
}

func (r *AttemptRepository) Save(ctx context.Context, phone domain.Phone, userID uuid.UUID, window domain.AttemptWindow) error {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return err
	}
	failures := min(window.Failures, math.MaxInt32)
	if err := r.q().UpsertLoginAttempt(ctx, pgen.UpsertLoginAttemptParams{
		Phone:          encryptedPhone,
		Failures:       int32(failures), //nolint:gosec // clamped to math.MaxInt32 by min above
		FirstFailureAt: pgtype.Timestamptz{Time: window.FirstFailureAt, Valid: true},
		LastFailureAt:  pgtype.Timestamptz{Time: window.LastFailureAt, Valid: true},
		UserID:         pgconv.UUIDToPgtype(userID),
		PhoneEncrypted: !r.enc.IsNoop(),
	}); err != nil {
		return fmt.Errorf("upsert login attempt: %w", err)
	}
	return nil
}

func (r *AttemptRepository) DeleteByPhone(ctx context.Context, phone domain.Phone) error {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return err
	}
	if err := r.q().DeleteLoginAttemptByPhone(ctx, encryptedPhone); err != nil {
		return fmt.Errorf("delete login attempt by phone: %w", err)
	}
	return nil
}

func (r *AttemptRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	if err := r.q().DeleteLoginAttemptsByUserID(ctx, pgconv.UUIDToPgtype(userID)); err != nil {
		return fmt.Errorf("delete login attempts by user id: %w", err)
	}
	return nil
}

func (r *AttemptRepository) DeleteStaleBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error) {
	n, err := r.q().DeleteStaleLoginAttemptsBatch(ctx, pgen.DeleteStaleLoginAttemptsBatchParams{
		LastFailureAt: pgtype.Timestamptz{Time: before, Valid: true},
		Limit:         batchSize,
	})
	if err != nil {
		return 0, fmt.Errorf("delete stale login attempts batch: %w", err)
	}
	return n, nil
}
