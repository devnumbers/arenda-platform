package application

import (
	"context"

	"github.com/google/uuid"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// fakePolicy maps (actor, scope) -> role for derived-access tests and
// (actor, property) -> role for property-scoped tests. Role defaults to
// RoleOwner when actor == scope (mirroring MembershipPolicy); RoleForProperty
// defaults to RoleOwner so pre-T3 tests keep the "actor is the owner"
// behaviour. Implements sharedpolicy.Policy.
type fakePolicy struct {
	roles         map[[2]uuid.UUID]sharedpolicy.Role
	propertyRoles map[[2]uuid.UUID]sharedpolicy.Role
}

func (f fakePolicy) Role(_ context.Context, actor, scope uuid.UUID) (sharedpolicy.Role, error) {
	if actor == scope {
		return sharedpolicy.RoleOwner, nil
	}
	if r, ok := f.roles[[2]uuid.UUID{actor, scope}]; ok {
		return r, nil
	}
	return sharedpolicy.RoleNone, nil
}

func (f fakePolicy) RoleForProperty(_ context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	if r, ok := f.propertyRoles[[2]uuid.UUID{actor, propertyID}]; ok {
		return r, nil
	}
	return sharedpolicy.RoleOwner, nil
}

// fakeAccessibleScopes maps actor -> accessible owners. Implements
// AccessibleScopes.
type fakeAccessibleScopes map[uuid.UUID][]uuid.UUID

func (f fakeAccessibleScopes) AccessibleOwners(_ context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return f[userID], nil
}

var (
	_ sharedpolicy.Policy = fakePolicy{}
	_ AccessibleScopes    = fakeAccessibleScopes(nil)
)
