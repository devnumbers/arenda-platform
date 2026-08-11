package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// ContactResolver resolves owner contact information.
type ContactResolver struct {
	queries *postgres.Queries
}

// NewContactResolver creates a resolver.
func NewContactResolver(queries *postgres.Queries) *ContactResolver {
	return &ContactResolver{queries: queries}
}

// Resolve returns the owner's contact.
// Currently returns email if verified; otherwise an error.
func (r *ContactResolver) Resolve(ctx context.Context, scope uuid.UUID) (application.Contact, error) {
	email, err := r.queries.GetVerifiedEmailByUserID(ctx, pgconv.UUIDToPgtype(scope))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.Contact{}, application.ErrNoContact
		}
		return application.Contact{}, fmt.Errorf("resolve contact: %w", err)
	}
	if !email.Valid || email.String == "" {
		return application.Contact{}, application.ErrNoContact
	}

	return application.Contact{
		Channel: application.ChannelEmail,
		Email:   email.String,
	}, nil
}
