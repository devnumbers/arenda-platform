package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	pgen "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/pgerr"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// LoginCodeRepository persists login codes.
type LoginCodeRepository struct {
	repoBase
}

// NewLoginCodeRepository creates a new login code repository.
func NewLoginCodeRepository(db pgen.DBTX) *LoginCodeRepository {
	return &LoginCodeRepository{repoBase{db: db}}
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *LoginCodeRepository) WithTx(tx transaction.Tx) (application.LoginCodeRepository, error) {
	dbtx, err := assertTxDB(tx)
	if err != nil {
		return nil, fmt.Errorf("identity.LoginCodeRepository.WithTx: %w", err)
	}
	return NewLoginCodeRepository(dbtx), nil
}

func (r *LoginCodeRepository) Save(ctx context.Context, code domain.LoginCode) error {
	if err := r.q().CreateLoginCode(ctx, pgen.CreateLoginCodeParams{
		ID:        pgconv.UUIDToPgtype(code.ID),
		Email:     code.Email.String(),
		CodeHash:  code.CodeHash,
		ExpiresAt: pgtype.Timestamptz{Time: code.ExpiresAt, Valid: true},
		UserID:    pgconv.UUIDToPgtypePtr(code.UserID),
		Purpose:   code.Purpose.String(),
	}); err != nil {
		if pgerr.IsUniqueViolation(err) {
			return application.ErrCodeSentTooRecently
		}
		return fmt.Errorf("create login code: %w", err)
	}
	return nil
}

func (r *LoginCodeRepository) GetLatestByEmailAndPurpose(
	ctx context.Context,
	email domain.Email,
	purpose domain.LoginCodePurpose,
	now time.Time,
) (domain.LoginCode, error) {
	row, err := r.q().GetLatestLoginCodeByEmailAndPurpose(ctx, pgen.GetLatestLoginCodeByEmailAndPurposeParams{
		Email:     email.String(),
		Purpose:   purpose.String(),
		ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if notFound(err) {
			return domain.LoginCode{}, application.ErrNotFound
		}
		return domain.LoginCode{}, fmt.Errorf("get latest login code: %w", err)
	}
	return mapLoginCode(loginCodeRow{
		id:        row.ID,
		userID:    row.UserID,
		email:     pgtype.Text{String: row.Email, Valid: true},
		codeHash:  row.CodeHash,
		expiresAt: row.ExpiresAt,
		used:      row.Used,
		createdAt: row.CreatedAt,
		purpose:   row.Purpose,
	})
}

func (r *LoginCodeRepository) DeleteExpiredByEmailAndPurpose(
	ctx context.Context,
	email domain.Email,
	purpose domain.LoginCodePurpose,
	before time.Time,
) error {
	if err := r.q().DeleteExpiredLoginCodesByEmailAndPurpose(ctx, pgen.DeleteExpiredLoginCodesByEmailAndPurposeParams{
		Email:     email.String(),
		Purpose:   purpose.String(),
		ExpiresAt: pgtype.Timestamptz{Time: before, Valid: true},
	}); err != nil {
		return fmt.Errorf("delete expired login codes by email: %w", err)
	}
	return nil
}

func (r *LoginCodeRepository) DeleteUnusedByEmailAndPurpose(
	ctx context.Context,
	email domain.Email,
	purpose domain.LoginCodePurpose,
) error {
	if err := r.q().DeleteUnusedLoginCodesByEmailAndPurpose(ctx, pgen.DeleteUnusedLoginCodesByEmailAndPurposeParams{
		Email:   email.String(),
		Purpose: purpose.String(),
	}); err != nil {
		return fmt.Errorf("delete unused login codes by email: %w", err)
	}
	return nil
}

func (r *LoginCodeRepository) MarkUsedByID(ctx context.Context, id uuid.UUID) error {
	if err := r.q().MarkLoginCodeUsed(ctx, pgconv.UUIDToPgtype(id)); err != nil {
		return fmt.Errorf("mark login code used: %w", err)
	}
	return nil
}

func (r *LoginCodeRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	if err := r.q().DeleteLoginCodeByID(ctx, pgconv.UUIDToPgtype(id)); err != nil {
		return fmt.Errorf("delete login code by id: %w", err)
	}
	return nil
}

func (r *LoginCodeRepository) DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error) {
	total, err := deleteBatched(ctx, before, func(ctx context.Context, before time.Time, limit int32) (int64, error) {
		n, err := r.q().DeleteExpiredLoginCodesBatch(ctx, pgen.DeleteExpiredLoginCodesBatchParams{
			ExpiresAt: pgtype.Timestamptz{Time: before, Valid: true},
			Limit:     limit,
		})
		if err != nil {
			return 0, fmt.Errorf("delete expired login codes batch: %w", err)
		}
		return n, nil
	})
	if err != nil {
		return 0, fmt.Errorf("delete expired login codes: %w", err)
	}
	return total, nil
}

type loginCodeRow struct {
	id        pgtype.UUID
	userID    pgtype.UUID
	email     pgtype.Text
	codeHash  string
	expiresAt pgtype.Timestamptz
	used      bool
	createdAt pgtype.Timestamptz
	purpose   string
}

func mapLoginCode(row loginCodeRow) (domain.LoginCode, error) {
	email, _, err := parseEmailField(row.email)
	if err != nil {
		return domain.LoginCode{}, err
	}
	purpose, err := domain.NewLoginCodePurpose(row.purpose)
	if err != nil {
		return domain.LoginCode{}, fmt.Errorf("invalid login code purpose from DB: %w", err)
	}
	return domain.LoginCode{
		ID:        pgconv.UUIDFromPgtype(row.id),
		UserID:    pgconv.UUIDFromPgtypePtr(row.userID),
		Email:     email,
		Purpose:   purpose,
		CodeHash:  row.codeHash,
		ExpiresAt: row.expiresAt.Time,
		Used:      row.used,
		CreatedAt: row.createdAt.Time,
	}, nil
}
