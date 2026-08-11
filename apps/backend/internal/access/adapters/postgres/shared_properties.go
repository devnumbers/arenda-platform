package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// SharedProperties resolves the property ids shared with a user via property
// memberships (issue #156, T3). It implements the properties application
// SharedMemberships port without creating a circular import: the access
// context owns the membership table and exposes a read-only adapter. The
// cross-context import of the properties application ports is accepted at the
// adapter level (issue T11).
type SharedProperties struct {
	db postgres.DBTX
}

// NewSharedProperties creates a SharedProperties adapter.
func NewSharedProperties(db postgres.DBTX) *SharedProperties {
	return &SharedProperties{db: db}
}

// SharedWith returns the distinct property ids where the user is a member.
// Kept for the leases context (operation reports, issue #157 T3); the
// properties context consumes MembershipsWith instead.
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

// MembershipsWith returns the active shared-access memberships of the user
// (property id + recipient role), implementing the properties application
// SharedMemberships port (issue T11).
func (s *SharedProperties) MembershipsWith(ctx context.Context, userID uuid.UUID) ([]propertiesapp.SharedMembership, error) {
	rows, err := postgres.New(s.db).ListPropertyMembersByUser(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, err
	}
	out := make([]propertiesapp.SharedMembership, 0, len(rows))
	for _, row := range rows {
		out = append(out, propertiesapp.SharedMembership{
			PropertyID: pgconv.UUIDFromPgtype(row.PropertyID),
			Role:       toPolicyRole(row.Role),
		})
	}
	return out, nil
}

// toPolicyRole maps the membership role column to a policy role; an
// unrecognized value degrades to RoleNone.
func toPolicyRole(role string) sharedpolicy.Role {
	switch role {
	case string(sharedpolicy.RoleFullAccess):
		return sharedpolicy.RoleFullAccess
	case string(sharedpolicy.RoleViewer):
		return sharedpolicy.RoleViewer
	default:
		return sharedpolicy.RoleNone
	}
}
