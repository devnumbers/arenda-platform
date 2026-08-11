package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// AccessibleScopes resolves the owners whose owner-wide data an actor may read
// (the owners of properties the actor is a member of). Implements
// leases/application.AccessibleScopes without creating a circular import: the
// access context owns the membership table and exposes a read-only adapter.
type AccessibleScopes struct {
	db postgres.DBTX
}

// NewAccessibleScopes creates an AccessibleScopes adapter.
func NewAccessibleScopes(db postgres.DBTX) *AccessibleScopes {
	return &AccessibleScopes{db: db}
}

// AccessibleOwners returns the distinct owner ids of the properties the user is
// a member of (issue #157).
func (a *AccessibleScopes) AccessibleOwners(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := postgres.New(a.db).ListAccessibleOwners(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		out = append(out, pgconv.UUIDFromPgtype(row))
	}
	return out, nil
}
