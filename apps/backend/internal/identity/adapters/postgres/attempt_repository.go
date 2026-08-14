package postgres

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
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
	repoBase
}

// NewAttemptRepository creates a new attempt repository.
func NewAttemptRepository(db pgen.DBTX, enc encryption.Encryptor) *AttemptRepository {
	return &AttemptRepository{repoBase{db: db, enc: enc}}
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *AttemptRepository) WithTx(tx transaction.Tx) (application.AttemptRepository, error) {
	dbtx, err := assertTxDB(tx)
	if err != nil {
		return nil, fmt.Errorf("identity.AttemptRepository.WithTx: %w", err)
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
		if notFound(err) {
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
		if notFound(err) {
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

// Save persists the attempt window for phone. When delta > 0 the failure counter
// is incremented atomically by the database (IncrementLoginAttempt) so that
// concurrent upserts on the same phone cannot lose an increment even without a
// preceding ForUpdate lock — the protection lives at the data layer, not only
// in the transactional locking convention (issue #215). When delta <= 0 the
// counter is set to the absolute window.Failures value (ResetLoginAttempt) — the
// reset path taken when the window is new or has expired past its TTL.
//
// The window's first_failure_at, last_failure_at, user_id, and phone_encrypted
// fields are always written as absolutes; only failures follows the
// delta/absolute split.
func (r *AttemptRepository) Save(ctx context.Context, phone domain.Phone, userID uuid.UUID, window domain.AttemptWindow, delta int) error {
	encryptedPhone, phoneEncrypted, err := phoneToColumns(ctx, r.enc, phone.String())
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate login attempt id: %w", err)
	}
	params := pgen.ResetLoginAttemptParams{
		ID:             pgconv.UUIDToPgtype(id),
		Phone:          encryptedPhone,
		FirstFailureAt: pgtype.Timestamptz{Time: window.FirstFailureAt, Valid: true},
		LastFailureAt:  pgtype.Timestamptz{Time: window.LastFailureAt, Valid: true},
		UserID:         pgconv.UUIDToPgtype(userID),
		PhoneEncrypted: phoneEncrypted,
	}
	if delta <= 0 {
		params.Failures = int32(min(window.Failures, math.MaxInt32)) //nolint:gosec // clamped to math.MaxInt32 by min above
		if err := r.q().ResetLoginAttempt(ctx, params); err != nil {
			return fmt.Errorf("reset login attempt: %w", err)
		}
		return nil
	}
	// Increment path: failures carries the delta, not the absolute value.
	params.Failures = int32(min(delta, math.MaxInt32)) //nolint:gosec // clamped to math.MaxInt32 by min above
	if err := r.q().IncrementLoginAttempt(ctx, pgen.IncrementLoginAttemptParams(params)); err != nil {
		return fmt.Errorf("increment login attempt: %w", err)
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

func (r *AttemptRepository) DeleteStaleBefore(ctx context.Context, before time.Time) (int64, error) {
	total, err := deleteBatched(ctx, before, func(ctx context.Context, before time.Time, limit int32) (int64, error) {
		n, err := r.q().DeleteStaleLoginAttemptsBatch(ctx, pgen.DeleteStaleLoginAttemptsBatchParams{
			LastFailureAt: pgtype.Timestamptz{Time: before, Valid: true},
			Limit:         limit,
		})
		if err != nil {
			return 0, fmt.Errorf("delete stale login attempts batch: %w", err)
		}
		return n, nil
	})
	if err != nil {
		return 0, fmt.Errorf("delete stale login attempts: %w", err)
	}
	return total, nil
}
