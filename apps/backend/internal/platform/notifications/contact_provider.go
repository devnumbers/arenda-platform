package notifications

import (
	"context"

	"github.com/google/uuid"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// ContactProvider wraps the identity application UserRepository to implement the notifications UserContactProvider port.
type ContactProvider struct {
	repo identityapp.UserRepository
}

// NewContactProvider creates a new ContactProvider adapter.
func NewContactProvider(repo identityapp.UserRepository) *ContactProvider {
	return &ContactProvider{repo: repo}
}

// PhoneByID returns the phone number for the user with the given ID.
func (p *ContactProvider) PhoneByID(ctx context.Context, userID uuid.UUID) (string, error) {
	return p.repo.GetPhoneByID(ctx, userID)
}

var _ notificationsapp.UserContactProvider = (*ContactProvider)(nil)
