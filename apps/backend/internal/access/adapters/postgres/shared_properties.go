package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// SharedProperties resolves the property ids shared with a user via property
// memberships (issue #156, T3). It implements the properties application
// SharedPropertyIDs port without creating a circular import: the access context
// owns the membership table and exposes a read-only adapter.
type SharedProperties struct {
	db postgres.DBTX
}

// NewSharedProperties creates a SharedProperties adapter.
func NewSharedProperties(db postgres.DBTX) *SharedProperties {
	return &SharedProperties{db: db}
}

// SharedWith returns the distinct property ids where the user is a member.
func (s *SharedProperties) SharedWith(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := postgres.New(s.db).ListPropertyMembersByUser(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		out = append(out, pgconv.UUIDFromPgtype(row.PropertyID))
	}
	return out, nil
}
