package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// ProfileService provides read and update operations for the user's own profile.
type ProfileService struct {
	users UserRepository
	db    transaction.Beginner
}

// NewProfileService creates a ProfileService.
func NewProfileService(users UserRepository, db transaction.Beginner) *ProfileService {
	return &ProfileService{users: users, db: db}
}

// Me returns the user profile.
func (s *ProfileService) Me(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	return s.users.GetByID(ctx, userID)
}

// UpdateProfile updates the user's personal data. When the email changes, the
// new address is marked as unverified until confirmed.
func (s *ProfileService) UpdateProfile(ctx context.Context, userID uuid.UUID, cmd UpdateProfileCommand) (domain.User, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	users, err := s.users.WithTx(tx)
	if err != nil {
		return domain.User{}, fmt.Errorf("bind user repository to tx: %w", err)
	}
	user, err := users.GetByIDForUpdate(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}

	var oldEmail string
	if user.Email != nil {
		oldEmail = user.Email.String()
	}

	if err := user.UpdatePersonalData(cmd.Name, cmd.Surname, cmd.Patronymic, cmd.Email); err != nil {
		return domain.User{}, fmt.Errorf("update personal data: %w", err)
	}

	emailChanged := cmd.Email != nil && user.Email != nil && user.Email.String() != oldEmail
	if emailChanged {
		user.EmailVerifiedAt = nil
	}

	updated, err := users.Update(ctx, user)
	if err != nil {
		return domain.User{}, fmt.Errorf("update user: %w", err)
	}
	user = updated

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("commit tx: %w", err)
	}

	return user, nil
}
