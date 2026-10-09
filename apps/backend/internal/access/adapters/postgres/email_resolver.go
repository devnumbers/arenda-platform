package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
)

// UserEmailResolverAdapter adapts an identity UserReader to the access
// UserEmailResolver port (issue #162, T6): the narrow path that exposes the
// user's email (PII) for the transactional sharing lifecycle emails.
type UserEmailResolverAdapter struct {
	users UserReader
}

// NewUserEmailResolver creates a UserEmailResolverAdapter.
func NewUserEmailResolver(users UserReader) *UserEmailResolverAdapter {
	return &UserEmailResolverAdapter{users: users}
}

// GetEmail returns the user's email address. A missing user or a user without
// an email reads as "" — the caller skips the send.
func (a *UserEmailResolverAdapter) GetEmail(ctx context.Context, id uuid.UUID) (string, error) {
	u, err := a.users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, identityapp.ErrNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("get user by id: %w", err)
	}
	if u.Email == nil {
		return "", nil
	}
	return u.Email.String(), nil
}

// GetPhotoURL returns the user's profile photo streaming path (ADR 0065,
// решение #1286): "" when the user is gone or has no photo. Serves the
// properties context's OwnerPhotoResolver (the owner avatar on the detail
// and the shared list rows).
func (a *UserEmailResolverAdapter) GetPhotoURL(ctx context.Context, id uuid.UUID) (string, error) {
	u, err := a.users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, identityapp.ErrNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("get user by id: %w", err)
	}
	return photo.UserPhotoPath(id, u.PhotoKey), nil
}

var _ application.UserEmailResolver = (*UserEmailResolverAdapter)(nil)
