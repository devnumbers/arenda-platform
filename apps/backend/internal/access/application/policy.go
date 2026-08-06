package application

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// MembershipPolicy is the T3 authorization policy. It resolves an actor's role
// relative to a data owner (owner-wide data) or a specific property
// (property-scoped data, issue #156).
//
// Owner-wide data (operation categories, tenant contacts and other account-level
// entities) is resolved as follows: the owner (actor == scope) gets RoleOwner;
// for any other actor the role is derived from the strongest membership the
// actor holds across all of the scope's properties — full access to at least
// one property grants RoleFullAccess over the scope's owner-wide data, view-only
// access grants RoleViewer, and no membership grants RoleNone (issue #157).
//
// Property-scoped data is resolved via the property's owner and any membership
// the actor holds: owner → RoleOwner, member → RoleFullAccess/RoleViewer,
// anyone else → RoleNone.
type MembershipPolicy struct {
	owners  PropertyOwnerResolver
	members MembershipRepository
}

// NewMembershipPolicy creates a MembershipPolicy.
func NewMembershipPolicy(owners PropertyOwnerResolver, members MembershipRepository) *MembershipPolicy {
	return &MembershipPolicy{owners: owners, members: members}
}

// Role returns the actor's role over owner-wide data (operation categories,
// tenant contacts and other account-level entities). For the owner's own data
// (actor == scope) it is RoleOwner. For any other actor the role is derived
// from the strongest membership the actor holds across all of the scope's
// properties: full access to at least one property grants RoleFullAccess over
// the scope's owner-wide data, view-only access grants RoleViewer, and no
// membership grants RoleNone (issue #157).
func (p *MembershipPolicy) Role(ctx context.Context, actor, scope uuid.UUID) (sharedpolicy.Role, error) {
	if actor == scope {
		return sharedpolicy.RoleOwner, nil
	}
	role, err := p.members.MaxRoleByOwner(ctx, actor, scope)
	if err != nil {
		return sharedpolicy.RoleNone, err
	}
	switch role {
	case domain.RoleFullAccess:
		return sharedpolicy.RoleFullAccess, nil
	case domain.RoleViewer:
		return sharedpolicy.RoleViewer, nil
	default:
		return sharedpolicy.RoleNone, nil
	}
}

// RoleForProperty returns the actor's role over a specific property. The
// property's existence is never revealed: a missing property, a missing
// membership, and lack of access all resolve to RoleNone, so callers can map
// RoleNone to a "not found" outcome and preserve object privacy.
//
// A suspended membership is treated as no access (RoleNone): the membership's
// role row is filtered out at the SQL level (status = 'active' in
// GetPropertyMemberRole), so a suspended recipient is indistinguishable from a
// non-member for authorization purposes. The UI distinguishes suspended for a
// dedicated screen (T9); here, privacy-preserving RoleNone is returned.
func (p *MembershipPolicy) RoleForProperty(ctx context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	owner, err := p.owners.GetOwnerID(ctx, propertyID)
	if err != nil {
		// A missing property maps to no access so the existence of an object is
		// never revealed (callers turn RoleNone into a "not found" outcome).
		if errors.Is(err, domain.ErrMemberNotFound) {
			return sharedpolicy.RoleNone, nil
		}
		return sharedpolicy.RoleNone, err
	}
	if actor == owner {
		return sharedpolicy.RoleOwner, nil
	}
	role, err := p.members.GetRole(ctx, propertyID, actor)
	if err != nil {
		if errors.Is(err, domain.ErrMemberNotFound) {
			return sharedpolicy.RoleNone, nil
		}
		return sharedpolicy.RoleNone, err
	}
	switch role {
	case domain.RoleFullAccess:
		return sharedpolicy.RoleFullAccess, nil
	case domain.RoleViewer:
		return sharedpolicy.RoleViewer, nil
	default:
		return sharedpolicy.RoleNone, nil
	}
}

// Compile-time check that MembershipPolicy implements sharedpolicy.Policy.
var _ sharedpolicy.Policy = (*MembershipPolicy)(nil)
