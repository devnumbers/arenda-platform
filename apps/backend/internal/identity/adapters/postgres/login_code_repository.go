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
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/pgerr"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// LoginCodeRepository persists login codes.
type LoginCodeRepository struct {
	db  pgen.DBTX
	enc encryption.Encryptor
}

// NewLoginCodeRepository creates a new login code repository.
func NewLoginCodeRepository(db pgen.DBTX, enc encryption.Encryptor) *LoginCodeRepository {
	return &LoginCodeRepository{db: db, enc: enc}
}

func (r *LoginCodeRepository) q() *pgen.Queries {
	return pgen.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *LoginCodeRepository) WithTx(tx transaction.Tx) (application.LoginCodeRepository, error) {
	dbtx, ok := tx.(pgen.DBTX)
	if !ok {
		return nil, fmt.Errorf("identity.LoginCodeRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewLoginCodeRepository(dbtx, r.enc), nil
}

func (r *LoginCodeRepository) Save(ctx context.Context, code domain.LoginCode) error {
	encryptedPhone, err := encryptPhone(ctx, r.enc, code.Phone.String())
	if err != nil {
		return err
	}
	if err := r.q().CreateLoginCode(ctx, pgen.CreateLoginCodeParams{
		ID:             pgconv.UUIDToPgtype(code.ID),
		Phone:          pgtype.Text{String: encryptedPhone, Valid: true},
		Email:          pgtype.Text{String: code.Email.String(), Valid: true},
		CodeHash:       code.CodeHash,
		ExpiresAt:      pgtype.Timestamptz{Time: code.ExpiresAt, Valid: true},
		UserID:         pgconv.UUIDToPgtypePtr(code.UserID),
		Purpose:        code.Purpose.String(),
		PhoneEncrypted: !r.enc.IsNoop(),
	}); err != nil {
		if pgerr.IsUniqueViolation(err) {
			return application.ErrCodeSentTooRecently
		}
		return fmt.Errorf("create login code: %w", err)
	}
	return nil
}

func (r *LoginCodeRepository) GetLatestByPhoneAndEmail(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose, now time.Time) (domain.LoginCode, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.LoginCode{}, err
	}
	row, err := r.q().GetLatestLoginCodeByPhoneAndEmailAndPurpose(ctx, pgen.GetLatestLoginCodeByPhoneAndEmailAndPurposeParams{
		Phone:     pgtype.Text{String: encryptedPhone, Valid: true},
		Email:     pgtype.Text{String: email.String(), Valid: true},
		Purpose:   purpose.String(),
		ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.LoginCode{}, application.ErrNotFound
		}
		return domain.LoginCode{}, fmt.Errorf("get latest login code: %w", err)
	}
	return r.mapLoginCode(ctx, toLoginCodeRowPhoneEmail(row))
}

func (r *LoginCodeRepository) DeleteExpiredByPhoneAndEmail(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose, before time.Time) error {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return err
	}
	if err := r.q().DeleteExpiredLoginCodesByPhoneAndEmail(ctx, pgen.DeleteExpiredLoginCodesByPhoneAndEmailParams{
		Phone:     pgtype.Text{String: encryptedPhone, Valid: true},
		Email:     pgtype.Text{String: email.String(), Valid: true},
		Purpose:   purpose.String(),
		ExpiresAt: pgtype.Timestamptz{Time: before, Valid: true},
	}); err != nil {
		return fmt.Errorf("delete expired login codes by phone and email: %w", err)
	}
	return nil
}

func (r *LoginCodeRepository) DeleteUnusedByPhoneAndEmail(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose) error {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return err
	}
	if err := r.q().DeleteUnusedLoginCodesByPhoneAndEmail(ctx, pgen.DeleteUnusedLoginCodesByPhoneAndEmailParams{
		Phone:   pgtype.Text{String: encryptedPhone, Valid: true},
		Email:   pgtype.Text{String: email.String(), Valid: true},
		Purpose: purpose.String(),
	}); err != nil {
		return fmt.Errorf("delete unused login codes by phone and email: %w", err)
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

func (r *LoginCodeRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	if err := r.q().DeleteLoginCodesByUserID(ctx, pgconv.UUIDToPgtype(userID)); err != nil {
		return fmt.Errorf("delete login codes by user id: %w", err)
	}
	return nil
}

func (r *LoginCodeRepository) DeleteExpiredBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error) {
	n, err := r.q().DeleteExpiredLoginCodesBatch(ctx, pgen.DeleteExpiredLoginCodesBatchParams{
		ExpiresAt: pgtype.Timestamptz{Time: before, Valid: true},
		Limit:     batchSize,
	})
	if err != nil {
		return 0, fmt.Errorf("delete expired login codes batch: %w", err)
	}
	return n, nil
}

type loginCodeRow struct {
	id             pgtype.UUID
	userID         pgtype.UUID
	phone          pgtype.Text
	email          pgtype.Text
	codeHash       string
	expiresAt      pgtype.Timestamptz
	used           bool
	createdAt      pgtype.Timestamptz
	purpose        string
	phoneEncrypted bool
}

func toLoginCodeRowPhoneEmail(row pgen.GetLatestLoginCodeByPhoneAndEmailAndPurposeRow) loginCodeRow {
	return loginCodeRow{
		id:             row.ID,
		userID:         row.UserID,
		phone:          row.Phone,
		email:          row.Email,
		codeHash:       row.CodeHash,
		expiresAt:      row.ExpiresAt,
		used:           row.Used,
		createdAt:      row.CreatedAt,
		purpose:        row.Purpose,
		phoneEncrypted: row.PhoneEncrypted,
	}
}

func (r *LoginCodeRepository) mapLoginCode(ctx context.Context, row loginCodeRow) (domain.LoginCode, error) {
	phone, err := decryptPhone(ctx, r.enc, row.phone.String, row.phoneEncrypted)
	if err != nil {
		return domain.LoginCode{}, err
	}
	parsedPhone, err := domain.NewPhone(phone)
	if err != nil {
		return domain.LoginCode{}, err
	}
	var email domain.Email
	if row.email.Valid && row.email.String != "" {
		e, err := domain.EmailFrom(row.email.String)
		if err != nil {
			return domain.LoginCode{}, fmt.Errorf("invalid email in DB: %w", err)
		}
		email = e
	}
	purpose, err := domain.NewLoginCodePurpose(row.purpose)
	if err != nil {
		return domain.LoginCode{}, err
	}
	return domain.LoginCode{
		ID:        pgconv.UUIDFromPgtype(row.id),
		UserID:    pgconv.UUIDFromPgtypePtr(row.userID),
		Phone:     parsedPhone,
		Email:     email,
		Purpose:   purpose,
		CodeHash:  row.codeHash,
		ExpiresAt: row.expiresAt.Time,
		Used:      row.used,
		CreatedAt: row.createdAt.Time,
	}, nil
}
