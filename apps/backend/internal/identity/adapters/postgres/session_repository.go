package postgres

import (
	"context"
	"errors"
	"fmt"
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

// SessionRepository persists sessions.
type SessionRepository struct {
	db  pgen.DBTX
	enc encryption.Encryptor
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(db pgen.DBTX, enc encryption.Encryptor) *SessionRepository {
	return &SessionRepository{db: db, enc: enc}
}

func (r *SessionRepository) q() *pgen.Queries {
	return pgen.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *SessionRepository) WithTx(tx transaction.Tx) (application.SessionRepository, error) {
	dbtx, ok := tx.(pgen.DBTX)
	if !ok {
		return nil, fmt.Errorf("identity.SessionRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewSessionRepository(dbtx, r.enc), nil
}

func (r *SessionRepository) Create(ctx context.Context, session domain.Session) error {
	_, err := r.q().CreateSession(ctx, pgen.CreateSessionParams{
		ID:         pgconv.UUIDToPgtype(session.ID),
		UserID:     pgconv.UUIDToPgtype(session.UserID),
		TokenHash:  session.TokenHash,
		ExpiresAt:  pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
		LastUsedAt: pgtype.Timestamptz{Time: session.LastUsedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *SessionRepository) Update(ctx context.Context, session domain.Session) error {
	if err := r.q().UpdateSession(ctx, pgen.UpdateSessionParams{
		ExpiresAt:  pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
		LastUsedAt: pgtype.Timestamptz{Time: session.LastUsedAt, Valid: true},
		TokenHash:  session.TokenHash,
	}); err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	if err := r.q().DeleteSessionByTokenHash(ctx, tokenHash); err != nil {
		return fmt.Errorf("delete session by token hash: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	if err := r.q().DeleteSessionsByUserID(ctx, pgconv.UUIDToPgtype(userID)); err != nil {
		return fmt.Errorf("delete sessions by user id: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteByUserIDExcept(ctx context.Context, userID uuid.UUID, tokenHash string) error {
	if err := r.q().DeleteSessionsByUserIDExcept(ctx, pgen.DeleteSessionsByUserIDExceptParams{
		UserID:    pgconv.UUIDToPgtype(userID),
		TokenHash: tokenHash,
	}); err != nil {
		return fmt.Errorf("delete sessions by user id except: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteExpiredBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error) {
	n, err := r.q().DeleteExpiredSessionsBatch(ctx, pgen.DeleteExpiredSessionsBatchParams{
		ExpiresAt: pgtype.Timestamptz{Time: before, Valid: true},
		Limit:     batchSize,
	})
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions batch: %w", err)
	}
	return n, nil
}

func (r *SessionRepository) GetByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.Session, domain.User, error) {
	row, err := r.q().GetSessionByTokenHash(ctx, pgen.GetSessionByTokenHashParams{
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.User{}, application.ErrNotFound
		}
		return domain.Session{}, domain.User{}, fmt.Errorf("get session by token hash: %w", err)
	}

	user, err := mapUser(ctx, r.enc, userRowFromGetSessionByTokenHashRow(row))
	if err != nil {
		return domain.Session{}, domain.User{}, err
	}

	session := domain.Session{
		ID:         pgconv.UUIDFromPgtype(row.ID),
		UserID:     pgconv.UUIDFromPgtype(row.UserID),
		TokenHash:  row.TokenHash,
		ExpiresAt:  row.ExpiresAt.Time,
		CreatedAt:  row.CreatedAt.Time,
		LastUsedAt: row.LastUsedAt.Time,
	}
	return session, user, nil
}
