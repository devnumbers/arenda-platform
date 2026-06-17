package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
)

// ContactResolver resolves the owner's phone number from Postgres.
type ContactResolver struct {
	db postgres.DBTX
}

// NewContactResolver creates a new contact resolver.
func NewContactResolver(db postgres.DBTX) *ContactResolver {
	return &ContactResolver{db: db}
}

func (r *ContactResolver) q() *postgres.Queries {
	return postgres.New(r.db)
}

// Resolve returns the owner's SMS contact.
func (r *ContactResolver) Resolve(ctx context.Context, ownerID uuid.UUID) (application.Contact, error) {
	phone, err := r.q().GetUserPhoneByID(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return application.Contact{}, fmt.Errorf("get user phone: %w", err)
	}
	return application.Contact{
		Channel: application.ChannelSMS,
		Address: phone,
	}, nil
}
