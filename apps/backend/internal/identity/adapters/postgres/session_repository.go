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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// SessionRepository persists sessions.
type SessionRepository struct {
	db  postgres.DBTX
	enc encryption.Encryptor
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(db postgres.DBTX, enc encryption.Encryptor) *SessionRepository {
	return &SessionRepository{db: db, enc: enc}
}

func (r *SessionRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *SessionRepository) WithTx(tx transaction.Tx) (application.SessionRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("identity.SessionRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewSessionRepository(dbtx, r.enc), nil
}

func (r *SessionRepository) Create(ctx context.Context, session domain.Session) error {
	_, err := r.q().CreateSession(ctx, postgres.CreateSessionParams{
		UserID:     pgconv.UUIDToPgtype(session.UserID),
		TokenHash:  session.TokenHash,
		ExpiresAt:  pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
		LastUsedAt: pgtype.Timestamptz{Time: session.LastUsedAt, Valid: true},
	})
	return err
}

func (r *SessionRepository) Update(ctx context.Context, session domain.Session) error {
	return r.q().UpdateSession(ctx, postgres.UpdateSessionParams{
		ExpiresAt:  pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
		LastUsedAt: pgtype.Timestamptz{Time: session.LastUsedAt, Valid: true},
		TokenHash:  session.TokenHash,
	})
}

func (r *SessionRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	return r.q().DeleteSessionByTokenHash(ctx, tokenHash)
}

func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.q().DeleteSessionsByUserID(ctx, pgconv.UUIDToPgtype(userID))
}

func (r *SessionRepository) DeleteByUserIDExcept(ctx context.Context, userID uuid.UUID, tokenHash string) error {
	return r.q().DeleteSessionsByUserIDExcept(ctx, postgres.DeleteSessionsByUserIDExceptParams{
		UserID:    pgconv.UUIDToPgtype(userID),
		TokenHash: tokenHash,
	})
}

func (r *SessionRepository) DeleteExpiredBefore(ctx context.Context, before time.Time) error {
	return r.q().DeleteExpiredSessions(ctx, pgtype.Timestamptz{Time: before, Valid: true})
}

func (r *SessionRepository) DeleteExpiredBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error) {
	return r.q().DeleteExpiredSessionsBatch(ctx, postgres.DeleteExpiredSessionsBatchParams{
		ExpiresAt: pgtype.Timestamptz{Time: before, Valid: true},
		Limit:     batchSize,
	})
}

func (r *SessionRepository) GetByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.Session, domain.User, error) {
	row, err := r.q().GetSessionByTokenHash(ctx, postgres.GetSessionByTokenHashParams{
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.User{}, application.ErrNotFound
		}
		return domain.Session{}, domain.User{}, err
	}
	phone, err := decryptPhone(ctx, r.enc, row.Phone, row.PhoneEncrypted)
	if err != nil {
		return domain.Session{}, domain.User{}, err
	}
	parsedPhone, err := domain.NewPhone(phone)
	if err != nil {
		return domain.Session{}, domain.User{}, err
	}
	role, err := domain.NewRole(row.Role)
	if err != nil {
		return domain.Session{}, domain.User{}, err
	}
	var emailPtr *domain.Email
	if row.Email.Valid && row.Email.String != "" {
		email, err := domain.EmailFrom(row.Email.String)
		if err != nil {
			return domain.Session{}, domain.User{}, fmt.Errorf("invalid email in DB: %w", err)
		}
		emailPtr = &email
	}
	return domain.Session{
			UserID:     pgconv.UUIDFromPgtype(row.UserID),
			TokenHash:  row.TokenHash,
			ExpiresAt:  row.ExpiresAt.Time,
			CreatedAt:  row.CreatedAt.Time,
			LastUsedAt: row.LastUsedAt.Time,
		}, domain.User{
			ID:              pgconv.UUIDFromPgtype(row.UserID),
			Phone:           parsedPhone,
			Role:            role,
			Name:            pgconv.TextToPtrString(row.Name),
			Surname:         pgconv.TextToPtrString(row.Surname),
			Patronymic:      pgconv.TextToPtrString(row.Patronymic),
			Email:           emailPtr,
			EmailVerifiedAt: pgconv.TimestamptzToPtrTime(row.EmailVerifiedAt),
		}, nil
}
