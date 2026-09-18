package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	identitydomain "github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// SharedListEnricher serves the access projections of the properties list
// reads (ticket #702): the active-member display names of the owner's own
// properties (the cards' participant row) and the recipient's suspended
// shared memberships as blur-card placeholders, each carrying the owner
// contact for the reason sheet. User data resolves through the identity user
// reader — the access SQL never joins users; the per-user lookups follow the
// ListMembers precedent (issue #693), the volumes are one list read's worth
// of members and owners.
type SharedListEnricher struct {
	db    postgres.DBTX
	users UserReader
}

// NewSharedListEnricher creates a SharedListEnricher.
func NewSharedListEnricher(db postgres.DBTX, users UserReader) *SharedListEnricher {
	return &SharedListEnricher{db: db, users: users}
}

// ownerContact is the reason sheet's contact row of one property owner.
type ownerContact struct {
	name  string
	email string
}

// NamesByOwner resolves the display names of the active members of the
// owner's properties, implementing the properties application
// MemberNamesReader port (ticket #702). The result carries a name list per
// property with at least one active member, in membership order.
func (s *SharedListEnricher) NamesByOwner(ctx context.Context, owner uuid.UUID) (map[uuid.UUID][]string, error) {
	rows, err := postgres.New(s.db).ListActiveMemberUsersByOwner(ctx, pgconv.UUIDToPgtype(owner))
	if err != nil {
		return nil, fmt.Errorf("list member users: %w", err)
	}
	names := make(map[uuid.UUID][]string, len(rows))
	display := make(map[uuid.UUID]string)
	for _, row := range rows {
		userID := pgconv.UUIDFromPgtype(row.UserID)
		name, ok := display[userID]
		if !ok {
			u, err := s.users.GetByID(ctx, userID)
			if err != nil {
				return nil, fmt.Errorf("lookup member user: %w", err)
			}
			name = application.DisplayNameOf(toMemberUser(u))
			display[userID] = name
		}
		propertyID := pgconv.UUIDFromPgtype(row.PropertyID)
		names[propertyID] = append(names[propertyID], name)
	}
	return names, nil
}

// SuspendedWith resolves the recipient's suspended shared memberships in the
// FIFO order (ticket #702), implementing the properties application
// SuspendedSharedMemberships port. Each placeholder carries the owner's
// display name and account email for the reason sheet's contact row; a
// missing owner row fails the read — the FK makes it unreachable, and an
// infra failure must surface as a 500, not as a silently missing card.
func (s *SharedListEnricher) SuspendedWith(ctx context.Context, userID uuid.UUID) ([]propertiesapp.SharedSuspendedMembership, error) {
	rows, err := postgres.New(s.db).ListSuspendedSharedWithOwner(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, fmt.Errorf("list suspended shared: %w", err)
	}
	out := make([]propertiesapp.SharedSuspendedMembership, 0, len(rows))
	owners := make(map[uuid.UUID]ownerContact)
	for _, row := range rows {
		ownerID := pgconv.UUIDFromPgtype(row.OwnerID)
		contact, ok := owners[ownerID]
		if !ok {
			u, err := s.users.GetByID(ctx, ownerID)
			if err != nil {
				return nil, fmt.Errorf("lookup owner user: %w", err)
			}
			contact = ownerContact{
				name:  application.DisplayNameOf(toMemberUser(u)),
				email: ownerEmail(u),
			}
			owners[ownerID] = contact
		}
		out = append(out, propertiesapp.SharedSuspendedMembership{
			PropertyID: pgconv.UUIDFromPgtype(row.PropertyID),
			// The role is DB-constrained to the two shared-access roles
			// (migration 000094); no corrupt-role filtering like the active
			// list's appendSharedProperties needs here.
			Role:       toPolicyRole(row.Role),
			OwnerName:  contact.name,
			OwnerEmail: contact.email,
		})
	}
	return out, nil
}

// ownerEmail extracts the owner's account email for the reason sheet — the
// mockup's deliberate contact exposure for this surface (Figma 2229-100002);
// empty when the owner has none.
func ownerEmail(u identitydomain.User) string {
	if u.Email == nil {
		return ""
	}
	return u.Email.String()
}
