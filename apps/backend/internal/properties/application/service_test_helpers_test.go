package application

import (
	"context"

	"github.com/google/uuid"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// testOwnerPolicy is the policy used by property service tests that pre-date
// the membership-aware policy. It treats the actor as the owner of every
// property (RoleOwner) so tests exercising owner-scoped behaviour keep working
// without wiring a real policy. It is intentionally test-only: production
// always injects the membership-aware policy from the access context.
type testOwnerPolicy struct{}

func (testOwnerPolicy) Role(_ context.Context, actor, scope uuid.UUID) (sharedpolicy.Role, error) {
	if actor == scope {
		return sharedpolicy.RoleOwner, nil
	}
	return sharedpolicy.RoleNone, nil
}

func (testOwnerPolicy) RoleForProperty(_ context.Context, _, _ uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleOwner, nil
}

// staticRolePolicy returns a fixed role for every RoleForProperty lookup. It
// backs privacy/outcome tests of the property service: how a given policy
// outcome (owner, member, none, suspended) maps to service errors (T9).
type staticRolePolicy struct {
	role sharedpolicy.Role
}

func (p staticRolePolicy) Role(_ context.Context, actor, scope uuid.UUID) (sharedpolicy.Role, error) {
	if actor == scope {
		return sharedpolicy.RoleOwner, nil
	}
	return sharedpolicy.RoleNone, nil
}

func (p staticRolePolicy) RoleForProperty(_ context.Context, _, _ uuid.UUID) (sharedpolicy.Role, error) {
	return p.role, nil
}
