// Package postgres holds the identity persistence adapters: user, session, login-code and attempt-window
// repositories.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	pgen "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/pgerr"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// UserRepository persists users.
type UserRepository struct {
	repoBase
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db pgen.DBTX) *UserRepository {
	return &UserRepository{repoBase{db: db}}
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *UserRepository) WithTx(tx transaction.Tx) (application.UserRepository, error) {
	dbtx, err := assertTxDB(tx)
	if err != nil {
		return nil, fmt.Errorf("identity.UserRepository.WithTx: %w", err)
	}
	return NewUserRepository(dbtx), nil
}

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.SharedPropertyChecker = (*UserRepository)(nil)

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.q().GetUserByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return mapUser(userSourceFromUser(row).toUserRow())
}

func (r *UserRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.q().GetUserByIDForUpdate(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by id for update: %w", err)
	}
	return mapUser(userSourceFromUser(row).toUserRow())
}

// ShareReadableProperty answers whether the two users are connected through
// at least one property both sides can read (ADR 0028 derived read,
// symmetric) — the gate of the foreign profile-photo reads (ADR 0065,
// решение владельца #1286). Implements the application's
// SharedPropertyChecker port; the verdict is one SQL query, no user rows
// leave the database.
func (r *UserRepository) ShareReadableProperty(ctx context.Context, viewerID, userID uuid.UUID) (bool, error) {
	shared, err := r.q().UsersShareReadableProperty(ctx, pgen.UsersShareReadablePropertyParams{
		ViewerID: pgconv.UUIDToPgtype(viewerID),
		UserID:   pgconv.UUIDToPgtype(userID),
	})
	if err != nil {
		return false, fmt.Errorf("check shared readable property: %w", err)
	}
	return shared, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email domain.Email) (domain.User, error) {
	row, err := r.q().GetUserByEmail(ctx, email.String())
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return mapUser(userSourceFromUser(row).toUserRow())
}

func (r *UserRepository) GetByEmailForUpdate(ctx context.Context, email domain.Email) (domain.User, error) {
	row, err := r.q().GetUserByEmailForUpdate(ctx, email.String())
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by email for update: %w", err)
	}
	return mapUser(userSourceFromUser(row).toUserRow())
}

func (r *UserRepository) Create(ctx context.Context, user domain.User) (domain.User, error) {
	row, err := r.q().CreateUser(ctx, pgen.CreateUserParams{
		ID:              pgconv.UUIDToPgtype(user.ID),
		Role:            user.Role.String(),
		Email:           emailToText(user.Email),
		EmailVerifiedAt: pgconv.TimePtrToPgtype(user.EmailVerifiedAt),
	})
	if err != nil {
		// ON CONFLICT returns no rows when an existing user was matched.
		if notFound(err) {
			return r.resolveConflictingUser(ctx, user)
		}
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return mapUser(userSourceFromCreateUser(row).toUserRow())
}

func (r *UserRepository) resolveConflictingUser(ctx context.Context, user domain.User) (domain.User, error) {
	if user.Email == nil {
		return domain.User{}, fmt.Errorf("resolve conflicting user: no email to look up: %w", application.ErrNotFound)
	}
	existing, err := r.GetByEmail(ctx, *user.Email)
	if err != nil {
		if notFound(err) {
			return domain.User{}, fmt.Errorf("resolve conflicting user: conflict resolved without finding user: %w", application.ErrNotFound)
		}
		return domain.User{}, fmt.Errorf("resolve conflicting user by email: %w", err)
	}
	return existing, nil
}

func (r *UserRepository) Update(ctx context.Context, user domain.User) (domain.User, error) {
	row, err := r.q().UpdateUser(ctx, pgen.UpdateUserParams{
		ID:              pgconv.UUIDToPgtype(user.ID),
		Name:            pgconv.StringPtrToPgtype(user.Name),
		Surname:         pgconv.StringPtrToPgtype(user.Surname),
		Patronymic:      pgconv.StringPtrToPgtype(user.Patronymic),
		Email:           emailToText(user.Email),
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
	return mapUser(userSourceFromUser(row).toUserRow())
}

func (r *UserRepository) UpdateEmailVerified(
	ctx context.Context,
	id uuid.UUID,
	email *domain.Email,
	verifiedAt *time.Time,
) (domain.User, error) {
	row, err := r.q().UpdateUserEmailVerified(ctx, pgen.UpdateUserEmailVerifiedParams{
		ID:              pgconv.UUIDToPgtype(id),
		Email:           emailToText(email),
		EmailVerifiedAt: pgconv.TimePtrToPgtype(verifiedAt),
	})
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("update user email verified: %w", err)
	}
	return mapUser(userSourceFromUser(row).toUserRow())
}

// SetPhoto writes the profile photo columns (ADR 0065): the key, the
// sniffed content type and the blur frame's base64 (решение #1594, #1603)
// move together, nils clear the photo.
func (r *UserRepository) SetPhoto(ctx context.Context, id uuid.UUID, key, contentType, blur *string) (domain.User, error) {
	row, err := r.q().SetUserPhoto(ctx, pgen.SetUserPhotoParams{
		ID:               pgconv.UUIDToPgtype(id),
		PhotoKey:         pgconv.StringPtrToPgtype(key),
		PhotoContentType: pgconv.StringPtrToPgtype(contentType),
		PhotoBlur:        pgconv.StringPtrToPgtype(blur),
	})
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("set user photo: %w", err)
	}
	return mapUser(userSourceFromUser(row).toUserRow())
}

// ClearPhotoIfKey repairs a dangling photo (ADR 0065): the columns are
// cleared only while they still advertise the given key — a concurrent
// re-upload (a fresh key under the row lock) wins. The affected-row count
// is the race verdict: 0 rows means the key was already replaced.
func (r *UserRepository) ClearPhotoIfKey(ctx context.Context, id uuid.UUID, key string) (int64, error) {
	rows, err := r.q().ClearUserPhotoIfKey(ctx, pgen.ClearUserPhotoIfKeyParams{
		ID:       pgconv.UUIDToPgtype(id),
		PhotoKey: pgconv.StringPtrToPgtype(&key),
	})
	if err != nil {
		return 0, fmt.Errorf("clear user photo: %w", err)
	}
	return rows, nil
}

// SetPhotoBlurIfKey lands the lazily materialized blur frame (ADR 0065
// аменд, #1603): the conditional UPDATE keys on the still-advertised photo,
// a concurrent re-upload wins. The affected-row count is the race verdict.
func (r *UserRepository) SetPhotoBlurIfKey(ctx context.Context, id uuid.UUID, key, blur string) (int64, error) {
	rows, err := r.q().SetUserPhotoBlurIfKey(ctx, pgen.SetUserPhotoBlurIfKeyParams{
		ID:        pgconv.UUIDToPgtype(id),
		PhotoKey:  pgconv.StringPtrToPgtype(&key),
		PhotoBlur: pgconv.StringPtrToPgtype(&blur),
	})
	if err != nil {
		return 0, fmt.Errorf("set user photo blur: %w", err)
	}
	return rows, nil
}

func (r *UserRepository) MarkEmailVerified(
	ctx context.Context,
	id uuid.UUID,
	verifiedAt time.Time,
) (domain.User, error) {
	row, err := r.q().UpdateUserEmailVerifiedAt(ctx, pgen.UpdateUserEmailVerifiedAtParams{
		ID:              pgconv.UUIDToPgtype(id),
		EmailVerifiedAt: pgconv.TimePtrToPgtype(&verifiedAt),
	})
	if err != nil {
		if notFound(err) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("mark user email verified: %w", err)
	}
	return mapUser(userSourceFromUser(row).toUserRow())
}

func mapUser(row userRow) (domain.User, error) {
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
		ID:               pgconv.UUIDFromPgtype(row.ID),
		Role:             role,
		Name:             pgconv.TextToPtrString(row.Name),
		Surname:          pgconv.TextToPtrString(row.Surname),
		Patronymic:       pgconv.TextToPtrString(row.Patronymic),
		Email:            email,
		EmailVerifiedAt:  pgconv.TimestamptzToPtrTime(row.EmailVerifiedAt),
		Timezone:         tz,
		PhotoKey:         pgconv.TextToPtrString(row.PhotoKey),
		PhotoContentType: pgconv.TextToPtrString(row.PhotoContentType),
		PhotoBlur:        pgconv.TextToPtrString(row.PhotoBlur),
	}, nil
}
