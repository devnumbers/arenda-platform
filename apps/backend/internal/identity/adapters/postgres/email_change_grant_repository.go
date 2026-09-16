package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	pgen "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// EmailChangeGrantRepository persists email-change grants. The token is stored
// only as the hash the application computed; the email is plaintext (same as
// users.email), so unlike the user and login-code repositories this one needs
// no encryptor.
type EmailChangeGrantRepository struct {
	db pgen.DBTX
}

var _ application.EmailChangeGrantRepository = (*EmailChangeGrantRepository)(nil)

// NewEmailChangeGrantRepository creates a new email-change grant repository.
func NewEmailChangeGrantRepository(db pgen.DBTX) *EmailChangeGrantRepository {
	return &EmailChangeGrantRepository{db: db}
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *EmailChangeGrantRepository) WithTx(tx transaction.Tx) (application.EmailChangeGrantRepository, error) {
	dbtx, err := assertTxDB(tx)
	if err != nil {
		return nil, fmt.Errorf("identity.EmailChangeGrantRepository.WithTx: %w", err)
	}
	return NewEmailChangeGrantRepository(dbtx), nil
}

// q exposes the sqlc Queries over the repository's database handle.
func (r *EmailChangeGrantRepository) q() *pgen.Queries { return pgen.New(r.db) }

func (r *EmailChangeGrantRepository) Save(ctx context.Context, grant domain.EmailChangeGrant) error {
	err := r.q().InsertEmailChangeGrant(ctx, pgen.InsertEmailChangeGrantParams{
		ID:        pgconv.UUIDToPgtype(grant.ID),
		UserID:    pgconv.UUIDToPgtype(grant.UserID),
		Email:     grant.Email.String(),
		TokenHash: grant.TokenHash,
		ExpiresAt: pgconv.TimePtrToPgtype(&grant.ExpiresAt),
		CreatedAt: pgconv.TimePtrToPgtype(&grant.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("insert email change grant: %w", err)
	}
	return nil
}

func (r *EmailChangeGrantRepository) GetByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.EmailChangeGrant, error) {
	row, err := r.q().GetEmailChangeGrantByUserIDForUpdate(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		if notFound(err) {
			return domain.EmailChangeGrant{}, application.ErrNotFound
		}
		return domain.EmailChangeGrant{}, fmt.Errorf("get email change grant by user: %w", err)
	}
	email, err := domain.EmailFrom(row.Email)
	if err != nil {
		return domain.EmailChangeGrant{}, fmt.Errorf("invalid grant email from DB: %w", err)
	}
	return domain.EmailChangeGrant{
		ID:        pgconv.UUIDFromPgtype(row.ID),
		UserID:    pgconv.UUIDFromPgtype(row.UserID),
		Email:     email,
		TokenHash: row.TokenHash,
		ExpiresAt: pgconv.TimestamptzToTime(row.ExpiresAt),
		CreatedAt: pgconv.TimestamptzToTime(row.CreatedAt),
	}, nil
}

func (r *EmailChangeGrantRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	if err := r.q().DeleteEmailChangeGrantByID(ctx, pgconv.UUIDToPgtype(id)); err != nil {
		return fmt.Errorf("delete email change grant: %w", err)
	}
	return nil
}

func (r *EmailChangeGrantRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	if err := r.q().DeleteEmailChangeGrantsByUserID(ctx, pgconv.UUIDToPgtype(userID)); err != nil {
		return fmt.Errorf("delete email change grants by user: %w", err)
	}
	return nil
}
