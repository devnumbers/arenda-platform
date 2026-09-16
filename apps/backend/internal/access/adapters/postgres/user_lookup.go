package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	identitydomain "github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// UserReader is the subset of the identity user repository that the access
// context needs. Declared here so this adapter depends on a narrow contract
// rather than the full identity repository.
type UserReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (identitydomain.User, error)
	GetByEmail(ctx context.Context, email identitydomain.Email) (identitydomain.User, error)
}

// UserLookupAdapter adapts an identity UserReader to the access UserLookup port.
type UserLookupAdapter struct {
	users UserReader
}

// NewUserLookup creates a UserLookupAdapter.
func NewUserLookup(users UserReader) *UserLookupAdapter {
	return &UserLookupAdapter{users: users}
}

// GetByID resolves a registered user by id for member display and the
// participant mutations (issue #694). A missing user maps to
// domain.ErrUserNotFound, mirroring GetByEmail and the port contract — real
// failures stay wrapped so callers can abort on them.
func (a *UserLookupAdapter) GetByID(ctx context.Context, id uuid.UUID) (application.MemberUser, error) {
	u, err := a.users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, identityapp.ErrNotFound) {
			return application.MemberUser{}, domain.ErrUserNotFound
		}
		return application.MemberUser{}, fmt.Errorf("get user by id: %w", err)
	}
	return toMemberUser(u), nil
}

// GetByEmail resolves a registered user by email. Used by the member lookup
// and email invitation paths (the email is not stored and never appears in
// audit context). A missing user maps to domain.ErrUserNotFound so the
// invitation flow can distinguish "unregistered email" from real failures.
func (a *UserLookupAdapter) GetByEmail(ctx context.Context, email string) (application.MemberUser, error) {
	parsed, err := identitydomain.NewEmail(email)
	if err != nil {
		return application.MemberUser{}, domain.ErrUserNotFound
	}
	u, err := a.users.GetByEmail(ctx, parsed)
	if err != nil {
		if errors.Is(err, identityapp.ErrNotFound) {
			return application.MemberUser{}, domain.ErrUserNotFound
		}
		return application.MemberUser{}, fmt.Errorf("get user by email: %w", err)
	}
	return toMemberUser(u), nil
}

func toMemberUser(u identitydomain.User) application.MemberUser {
	return application.MemberUser{
		ID:       u.ID,
		Name:     u.Name,
		Surname:  u.Surname,
		Phone:    u.Phone.String(),
		HasEmail: u.Email != nil,
	}
}
