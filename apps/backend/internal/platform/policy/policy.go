// Package policy provides the platform-level authorization policy adapters.
//
// The membership-aware policy (MembershipPolicy) lives in the access bounded
// context (internal/access/application) and is the production policy wired in
// cmd/api/wire. This package keeps OwnerOnlyPolicy as a dependency-free fallback
// used by tests that exercise owner-wide authorization without a database.
package policy

import (
	"context"
	"errors"

	"github.com/google/uuid"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// ownerNotFound is returned by the fallback owner resolver to signal that the
// property does not exist; it is mapped to RoleNone so existence is not leaked.
var errOwnerNotFound = errors.New("owner not found")

// OwnerResolver resolves the owner of a property. The fallback OwnerOnlyPolicy
// uses it only for RoleForProperty; nil leaves RoleForProperty owner-only.
type OwnerResolver func(ctx context.Context, propertyID uuid.UUID) (uuid.UUID, error)

// OwnerOnlyPolicy is a dependency-free policy: an actor has access only to
// their own data (actor == scope). Property membership is not consulted. It is
// the T2 policy kept as a fallback for tests; production wires the
// membership-aware policy from the access context.
type OwnerOnlyPolicy struct {
	// The owner resolver is optional. When nil, RoleForProperty cannot resolve
	// an owner and returns RoleNone for any actor (preserving privacy for
	// unknown objects).
	owner OwnerResolver
}

// NewOwnerOnlyPolicy creates an OwnerOnlyPolicy without a property owner
// resolver; RoleForProperty always returns RoleNone.
func NewOwnerOnlyPolicy() *OwnerOnlyPolicy {
	return &OwnerOnlyPolicy{}
}

// NewOwnerOnlyPolicyWithResolver creates an OwnerOnlyPolicy that resolves
// property owners through the given resolver, used by RoleForProperty.
func NewOwnerOnlyPolicyWithResolver(owner OwnerResolver) *OwnerOnlyPolicy {
	return &OwnerOnlyPolicy{owner: owner}
}

// Role returns RoleOwner when the actor is the data owner (actor == scope),
// and RoleNone otherwise.
func (p *OwnerOnlyPolicy) Role(_ context.Context, actor, scope uuid.UUID) (sharedpolicy.Role, error) {
	if actor == scope {
		return sharedpolicy.RoleOwner, nil
	}
	return sharedpolicy.RoleNone, nil
}

// RoleForProperty returns RoleOwner when the actor owns the property, and
// RoleNone otherwise. Without an owner resolver, every actor is treated as a
// non-owner (RoleNone), preserving object privacy.
func (p *OwnerOnlyPolicy) RoleForProperty(ctx context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	if p.owner == nil {
		return sharedpolicy.RoleNone, nil
	}
	owner, err := p.owner(ctx, propertyID)
	if err != nil {
		if errors.Is(err, errOwnerNotFound) {
			return sharedpolicy.RoleNone, nil
		}
		return sharedpolicy.RoleNone, err
	}
	if actor == owner {
		return sharedpolicy.RoleOwner, nil
	}
	return sharedpolicy.RoleNone, nil
}

// Compile-time check that OwnerOnlyPolicy implements sharedpolicy.Policy.
var _ sharedpolicy.Policy = (*OwnerOnlyPolicy)(nil)
