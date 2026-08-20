// Package postgres holds the identity persistence adapters: user, session, login-code and attempt-window
// repositories, with deterministic phone encryption at the storage boundary.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	pgen "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/pgerr"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// UserRepository persists users.
type UserRepository struct {
	repoBase
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db pgen.DBTX, enc encryption.Encryptor) *UserRepository {
	return &UserRepository{repoBase{db: db, enc: enc}}
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *UserRepository) WithTx(tx transaction.Tx) (application.UserRepository, error) {
	dbtx, err := assertTxDB(tx)
	if err != nil {
		return nil, fmt.Errorf("identity.UserRepository.WithTx: %w", err)
	}
	return NewUserRepository(dbtx, r.enc), nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.q().GetUserByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return mapUser(ctx, r.enc, userSourceFromUser(row).toUserRow())
}

func (r *UserRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.q().GetUserByIDForUpdate(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by id for update: %w", err)
	}
	return mapUser(ctx, r.enc, userSourceFromUser(row).toUserRow())
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone domain.Phone) (domain.User, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.User{}, err
	}
	row, err := r.q().GetUserByPhone(ctx, encryptedPhone)
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by phone: %w", err)
	}
	return mapUser(ctx, r.enc, userSourceFromUser(row).toUserRow())
}

func (r *UserRepository) GetByPhoneForUpdate(ctx context.Context, phone domain.Phone) (domain.User, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.User{}, err
	}
	row, err := r.q().GetUserByPhoneForUpdate(ctx, encryptedPhone)
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by phone for update: %w", err)
	}
	return mapUser(ctx, r.enc, userSourceFromUser(row).toUserRow())
}

func (r *UserRepository) GetByEmail(ctx context.Context, email domain.Email) (domain.User, error) {
	row, err := r.q().GetUserByEmail(ctx, email.String())
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return mapUser(ctx, r.enc, userSourceFromUser(row).toUserRow())
}

func (r *UserRepository) Create(ctx context.Context, user domain.User) (domain.User, error) {
	encryptedPhone, phoneEncrypted, err := phoneToColumns(ctx, r.enc, user.Phone.String())
	if err != nil {
		return domain.User{}, err
	}
	row, err := r.q().CreateUser(ctx, pgen.CreateUserParams{
		ID:              pgconv.UUIDToPgtype(user.ID),
		Phone:           encryptedPhone,
		Role:            user.Role.String(),
		PhoneEncrypted:  phoneEncrypted,
		Email:           emailPtrToPgtype(user.Email),
		EmailVerifiedAt: pgconv.TimePtrToPgtype(user.EmailVerifiedAt),
	})
	if err != nil {
		// ON CONFLICT returns no rows when an existing user was matched.
		if notFound(err) {
			return r.resolveConflictingUser(ctx, user)
		}
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return mapUser(ctx, r.enc, userSourceFromCreateUser(row).toUserRow())
}

func (r *UserRepository) resolveConflictingUser(ctx context.Context, user domain.User) (domain.User, error) {
	existing, err := r.GetByPhone(ctx, user.Phone)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, application.ErrNotFound) {
		return domain.User{}, fmt.Errorf("resolve conflicting user by phone: %w", err)
	}

	if user.Email == nil {
		return domain.User{}, fmt.Errorf("resolve conflicting user: no user by phone and no email to look up: %w", application.ErrNotFound)
	}

	existing, err = r.GetByEmail(ctx, *user.Email)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			return domain.User{}, fmt.Errorf("resolve conflicting user: conflict resolved without finding user: %w", application.ErrNotFound)
		}
		return domain.User{}, fmt.Errorf("resolve conflicting user by email: %w", err)
	}

	if existing.Phone != user.Phone {
		return domain.User{}, application.ErrEmailAlreadyTaken
	}
	return existing, nil
}

func (r *UserRepository) Update(ctx context.Context, user domain.User) (domain.User, error) {
	row, err := r.q().UpdateUser(ctx, pgen.UpdateUserParams{
		ID:              pgconv.UUIDToPgtype(user.ID),
		Name:            pgconv.StringPtrToPgtype(user.Name),
		Surname:         pgconv.StringPtrToPgtype(user.Surname),
		Patronymic:      pgconv.StringPtrToPgtype(user.Patronymic),
		Email:           emailPtrToPgtype(user.Email),
		EmailVerifiedAt: pgconv.TimePtrToPgtype(user.EmailVerifiedAt),
		Timezone:        user.Timezone.String(),
	})
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		if pgerr.IsUniqueViolation(err) {
			return domain.User{}, application.ErrEmailAlreadyTaken
		}
		return domain.User{}, fmt.Errorf("update user: %w", err)
	}
	return mapUser(ctx, r.enc, userSourceFromUser(row).toUserRow())
}

func (r *UserRepository) UpdatePhone(ctx context.Context, id uuid.UUID, phone domain.Phone) (domain.User, error) {
	encryptedPhone, phoneEncrypted, err := phoneToColumns(ctx, r.enc, phone.String())
	if err != nil {
		return domain.User{}, err
	}
	row, err := r.q().UpdateUserPhone(ctx, pgen.UpdateUserPhoneParams{
		ID:             pgconv.UUIDToPgtype(id),
		Phone:          encryptedPhone,
		PhoneEncrypted: phoneEncrypted,
	})
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		if pgerr.IsUniqueViolation(err) {
			return domain.User{}, application.ErrPhoneAlreadyTaken
		}
		return domain.User{}, fmt.Errorf("update user phone: %w", err)
	}
	return mapUser(ctx, r.enc, userSourceFromUser(row).toUserRow())
}

func (r *UserRepository) UpdateEmailVerified(
	ctx context.Context,
	id uuid.UUID,
	email *domain.Email,
	verifiedAt *time.Time,
) (domain.User, error) {
	row, err := r.q().UpdateUserEmailVerified(ctx, pgen.UpdateUserEmailVerifiedParams{
		ID:              pgconv.UUIDToPgtype(id),
		Email:           emailPtrToPgtype(email),
		EmailVerifiedAt: pgconv.TimePtrToPgtype(verifiedAt),
	})
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("update user email verified: %w", err)
	}
	return mapUser(ctx, r.enc, userSourceFromUpdateEmailVerified(row).toUserRow())
}

func mapUser(ctx context.Context, enc encryption.Encryptor, row userRow) (domain.User, error) {
	phone, err := decryptPhoneField(ctx, enc, row.Phone, row.PhoneEncrypted)
	if err != nil {
		return domain.User{}, err
	}
	role, err := domain.NewRole(row.Role)
	if err != nil {
		return domain.User{}, fmt.Errorf("invalid role from DB: %w", err)
	}
	emailValue, present, err := parseEmailField(row.Email)
	if err != nil {
		return domain.User{}, err
	}
	var email *domain.Email
	if present {
		email = &emailValue
	}
	if row.Timezone == "" {
		return domain.User{}, fmt.Errorf("invalid timezone from DB: %w", domain.ErrInvalidTimezone)
	}
	tz, err := domain.TimezoneFrom(row.Timezone)
	if err != nil {
		return domain.User{}, fmt.Errorf("invalid timezone from DB: %w", err)
	}
	return domain.User{
		ID:              pgconv.UUIDFromPgtype(row.ID),
		Phone:           phone,
		Role:            role,
		Name:            pgconv.TextToPtrString(row.Name),
		Surname:         pgconv.TextToPtrString(row.Surname),
		Patronymic:      pgconv.TextToPtrString(row.Patronymic),
		Email:           email,
		EmailVerifiedAt: pgconv.TimestamptzToPtrTime(row.EmailVerifiedAt),
		Timezone:        tz,
	}, nil
}

func emailPtrToPgtype(e *domain.Email) pgtype.Text {
	if e == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: e.String(), Valid: true}
}
