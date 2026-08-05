package application

import (
	"context"

	"github.com/google/uuid"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// fakePolicy maps (actor, scope) -> role for derived-access tests. When actor
// == scope it returns RoleOwner (mirroring MembershipPolicy). Implements
// sharedpolicy.Policy.
type fakePolicy struct {
	roles map[[2]uuid.UUID]sharedpolicy.Role
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

func (fakePolicy) RoleForProperty(_ context.Context, _, _ uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleNone, nil
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
