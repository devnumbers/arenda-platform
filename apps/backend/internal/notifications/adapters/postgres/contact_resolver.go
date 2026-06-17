package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// ContactResolver resolves the owner's phone number from a UserContactProvider port.
type ContactResolver struct {
	provider application.UserContactProvider
}

// NewContactResolver creates a new contact resolver.
func NewContactResolver(provider application.UserContactProvider) *ContactResolver {
	return &ContactResolver{provider: provider}
}

// Resolve returns the owner's SMS contact.
func (r *ContactResolver) Resolve(ctx context.Context, ownerID uuid.UUID) (application.Contact, error) {
	phone, err := r.provider.PhoneByID(ctx, ownerID)
	if err != nil {
		return application.Contact{}, fmt.Errorf("get user phone: %w", err)
	}
	return application.Contact{
		Channel: application.ChannelSMS,
		Address: phone,
	}, nil
}
