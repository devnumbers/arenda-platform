package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// UserRepository persists users.
type UserRepository struct {
	db  postgres.DBTX
	enc encryption.Encryptor
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db postgres.DBTX, enc encryption.Encryptor) *UserRepository {
	return &UserRepository{db: db, enc: enc}
}

func (r *UserRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *UserRepository) WithTx(tx transaction.Tx) (application.UserRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("identity.UserRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewUserRepository(dbtx, r.enc), nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.q().GetUserByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.q().GetUserByIDForUpdate(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) GetPhoneByID(ctx context.Context, id uuid.UUID) (domain.Phone, error) {
	row, err := r.q().GetUserPhoneByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Phone{}, application.ErrNotFound
		}
		return domain.Phone{}, err
	}
	phone, err := decryptPhone(ctx, r.enc, row.Phone, row.PhoneEncrypted)
	if err != nil {
		return domain.Phone{}, err
	}
	parsed, err := domain.NewPhone(phone)
	if err != nil {
		return domain.Phone{}, err
	}
	return parsed, nil
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone domain.Phone) (domain.User, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.User{}, err
	}
	row, err := r.q().GetUserByPhone(ctx, encryptedPhone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) GetByPhoneForUpdate(ctx context.Context, phone domain.Phone) (domain.User, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.User{}, err
	}
	row, err := r.q().GetUserByPhoneForUpdate(ctx, encryptedPhone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email domain.Email) (domain.User, error) {
	row, err := r.q().GetUserByEmail(ctx, email.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) Create(ctx context.Context, user domain.User) (domain.User, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, user.Phone.String())
	if err != nil {
		return domain.User{}, err
	}
	row, err := r.q().CreateUser(ctx, postgres.CreateUserParams{
		ID:              pgconv.UUIDToPgtype(user.ID),
		Phone:           encryptedPhone,
		Role:            user.Role.String(),
		PhoneEncrypted:  !r.enc.IsNoop(),
		Email:           emailPtrToPgtype(user.Email),
		EmailVerifiedAt: pgconv.TimePtrToPgtype(user.EmailVerifiedAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return r.GetByPhone(ctx, user.Phone)
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, postgres.User{
		ID:              row.ID,
		Phone:           row.Phone,
		Role:            row.Role,
		Name:            row.Name,
		Surname:         row.Surname,
		Patronymic:      row.Patronymic,
		Email:           row.Email,
		EmailVerifiedAt: row.EmailVerifiedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		PhoneEncrypted:  row.PhoneEncrypted,
	})
}

func (r *UserRepository) Update(ctx context.Context, user domain.User) (domain.User, error) {
	row, err := r.q().UpdateUser(ctx, postgres.UpdateUserParams{
		ID:         pgconv.UUIDToPgtype(user.ID),
		Name:       pgconv.StringPtrToPgtype(user.Name),
		Surname:    pgconv.StringPtrToPgtype(user.Surname),
		Patronymic: pgconv.StringPtrToPgtype(user.Patronymic),
		Email:      emailPtrToPgtype(user.Email),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return domain.User{}, application.ErrEmailAlreadyTaken
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) UpdatePhone(ctx context.Context, id uuid.UUID, phone domain.Phone) (domain.User, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.User{}, err
	}
	row, err := r.q().UpdateUserPhone(ctx, postgres.UpdateUserPhoneParams{
		ID:             pgconv.UUIDToPgtype(id),
		Phone:          encryptedPhone,
		PhoneEncrypted: !r.enc.IsNoop(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return domain.User{}, application.ErrPhoneAlreadyTaken
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) UpdateEmailVerified(ctx context.Context, id uuid.UUID, email *domain.Email, verifiedAt *time.Time) (domain.User, error) {
	row, err := r.q().UpdateUserEmailVerified(ctx, postgres.UpdateUserEmailVerifiedParams{
		ID:              pgconv.UUIDToPgtype(id),
		Email:           emailPtrToPgtype(email),
		EmailVerifiedAt: pgconv.TimePtrToPgtype(verifiedAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, postgres.User{
		ID:              row.ID,
		Phone:           row.Phone,
		Role:            row.Role,
		Name:            row.Name,
		Surname:         row.Surname,
		Patronymic:      row.Patronymic,
		Email:           row.Email,
		EmailVerifiedAt: row.EmailVerifiedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		PhoneEncrypted:  row.PhoneEncrypted,
	})
}

func (r *UserRepository) mapUser(ctx context.Context, row postgres.User) (domain.User, error) {
	phone, err := decryptPhone(ctx, r.enc, row.Phone, row.PhoneEncrypted)
	if err != nil {
		return domain.User{}, err
	}
	parsedPhone, err := domain.NewPhone(phone)
	if err != nil {
		return domain.User{}, err
	}
	role, err := domain.NewRole(row.Role)
	if err != nil {
		return domain.User{}, err
	}
	var email *domain.Email
	if row.Email.Valid && row.Email.String != "" {
		e, err := domain.EmailFrom(row.Email.String)
		if err != nil {
			return domain.User{}, fmt.Errorf("invalid email in DB: %w", err)
		}
		email = &e
	}
	return domain.User{
		ID:              pgconv.UUIDFromPgtype(row.ID),
		Phone:           parsedPhone,
		Role:            role,
		Name:            pgconv.TextToPtrString(row.Name),
		Surname:         pgconv.TextToPtrString(row.Surname),
		Patronymic:      pgconv.TextToPtrString(row.Patronymic),
		Email:           email,
		EmailVerifiedAt: pgconv.TimestamptzToPtrTime(row.EmailVerifiedAt),
	}, nil
}

func emailPtrToPgtype(e *domain.Email) pgtype.Text {
	if e == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: e.String(), Valid: true}
}
