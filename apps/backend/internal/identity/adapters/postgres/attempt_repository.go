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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// AttemptRepository persists login attempt windows.
type AttemptRepository struct {
	db  postgres.DBTX
	enc encryption.Encryptor
}

// NewAttemptRepository creates a new attempt repository.
func NewAttemptRepository(db postgres.DBTX, enc encryption.Encryptor) *AttemptRepository {
	return &AttemptRepository{db: db, enc: enc}
}

func (r *AttemptRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *AttemptRepository) WithTx(tx transaction.Tx) (application.AttemptRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
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
		return domain.AttemptWindow{}, err
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
		return domain.AttemptWindow{}, err
	}
	return domain.AttemptWindow{
		Failures:       int(row.Failures),
		FirstFailureAt: row.FirstFailureAt.Time,
		LastFailureAt:  row.LastFailureAt.Time,
	}, nil
}

func (r *AttemptRepository) Save(ctx context.Context, phone domain.Phone, userID uuid.UUID, window domain.AttemptWindow) error {
	failures := window.Failures
	if failures < 0 {
		failures = 0
	}
	if failures > math.MaxInt32 {
		failures = math.MaxInt32
	}
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return err
	}
	return r.q().UpsertLoginAttempt(ctx, postgres.UpsertLoginAttemptParams{
		Phone:          encryptedPhone,
		Failures:       int32(failures),
		FirstFailureAt: pgtype.Timestamptz{Time: window.FirstFailureAt, Valid: true},
		LastFailureAt:  pgtype.Timestamptz{Time: window.LastFailureAt, Valid: true},
		UserID:         pgconv.UUIDToPgtype(userID),
		PhoneEncrypted: !r.enc.IsNoop(),
	})
}

func (r *AttemptRepository) DeleteByPhone(ctx context.Context, phone domain.Phone) error {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return err
	}
	return r.q().DeleteLoginAttemptByPhone(ctx, encryptedPhone)
}

func (r *AttemptRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.q().DeleteLoginAttemptsByUserID(ctx, pgconv.UUIDToPgtype(userID))
}

func (r *AttemptRepository) DeleteStaleBefore(ctx context.Context, before time.Time) error {
	return r.q().DeleteStaleLoginAttempts(ctx, pgtype.Timestamptz{Time: before, Valid: true})
}

func (r *AttemptRepository) DeleteStaleBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error) {
	return r.q().DeleteStaleLoginAttemptsBatch(ctx, postgres.DeleteStaleLoginAttemptsBatchParams{
		LastFailureAt: pgtype.Timestamptz{Time: before, Valid: true},
		Limit:         batchSize,
	})
}
