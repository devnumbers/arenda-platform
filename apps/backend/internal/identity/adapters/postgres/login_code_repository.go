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

// LoginCodeRepository persists login codes.
type LoginCodeRepository struct {
	db  postgres.DBTX
	enc encryption.Encryptor
}

// NewLoginCodeRepository creates a new login code repository.
func NewLoginCodeRepository(db postgres.DBTX, enc encryption.Encryptor) *LoginCodeRepository {
	return &LoginCodeRepository{db: db, enc: enc}
}

func (r *LoginCodeRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *LoginCodeRepository) WithTx(tx transaction.Tx) (application.LoginCodeRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
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
	return r.q().CreateLoginCode(ctx, postgres.CreateLoginCodeParams{
		ID:             pgconv.UUIDToPgtype(code.ID),
		Phone:          pgtype.Text{String: encryptedPhone, Valid: true},
		Email:          pgtype.Text{String: code.Email.String(), Valid: code.Email.String() != ""},
		CodeHash:       code.CodeHash,
		ExpiresAt:      pgtype.Timestamptz{Time: code.ExpiresAt, Valid: true},
		UserID:         pgconv.UUIDToPgtypePtr(code.UserID),
		Purpose:        code.Purpose.String(),
		PhoneEncrypted: !r.enc.IsNoop(),
	})
}

func (r *LoginCodeRepository) GetLatestByPhoneAndEmail(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose, now time.Time) (domain.LoginCode, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.LoginCode{}, err
	}
	row, err := r.q().GetLatestLoginCodeByPhoneAndEmailAndPurpose(ctx, postgres.GetLatestLoginCodeByPhoneAndEmailAndPurposeParams{
		Phone:     pgtype.Text{String: encryptedPhone, Valid: true},
		Email:     pgtype.Text{String: email.String(), Valid: true},
		Purpose:   purpose.String(),
		ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.LoginCode{}, application.ErrNotFound
		}
		return domain.LoginCode{}, err
	}
	return r.mapLoginCode(ctx, toLoginCodeRowPhoneEmail(row))
}

func (r *LoginCodeRepository) DeleteExpiredByPhoneAndEmail(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose, before time.Time) error {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return err
	}
	return r.q().DeleteExpiredLoginCodesByPhoneAndEmail(ctx, postgres.DeleteExpiredLoginCodesByPhoneAndEmailParams{
		Phone:     pgtype.Text{String: encryptedPhone, Valid: true},
		Email:     pgtype.Text{String: email.String(), Valid: true},
		Purpose:   purpose.String(),
		ExpiresAt: pgtype.Timestamptz{Time: before, Valid: true},
	})
}

func (r *LoginCodeRepository) DeleteUnusedByPhoneAndEmail(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose) error {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return err
	}
	return r.q().DeleteUnusedLoginCodesByPhoneAndEmail(ctx, postgres.DeleteUnusedLoginCodesByPhoneAndEmailParams{
		Phone:   pgtype.Text{String: encryptedPhone, Valid: true},
		Email:   pgtype.Text{String: email.String(), Valid: true},
		Purpose: purpose.String(),
	})
}

func (r *LoginCodeRepository) MarkUsedByID(ctx context.Context, id uuid.UUID) error {
	return r.q().MarkLoginCodeUsed(ctx, pgconv.UUIDToPgtype(id))
}

func (r *LoginCodeRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return r.q().DeleteLoginCodeByID(ctx, pgconv.UUIDToPgtype(id))
}

func (r *LoginCodeRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.q().DeleteLoginCodesByUserID(ctx, pgconv.UUIDToPgtype(userID))
}

func (r *LoginCodeRepository) DeleteExpiredBefore(ctx context.Context, before time.Time) error {
	return r.q().DeleteExpiredLoginCodes(ctx, pgtype.Timestamptz{Time: before, Valid: true})
}

func (r *LoginCodeRepository) DeleteExpiredBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error) {
	return r.q().DeleteExpiredLoginCodesBatch(ctx, postgres.DeleteExpiredLoginCodesBatchParams{
		ExpiresAt: pgtype.Timestamptz{Time: before, Valid: true},
		Limit:     batchSize,
	})
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

func toLoginCodeRowPhoneEmail(row postgres.GetLatestLoginCodeByPhoneAndEmailAndPurposeRow) loginCodeRow {
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
