package policy

import (
	"context"

	"github.com/google/uuid"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// OwnerOnlyPolicy is the T2 policy: an actor has access only to their own data (actor == scope). Property membership is not yet supported; it will be added in T3.
type OwnerOnlyPolicy struct{}

// NewOwnerOnlyPolicy creates an OwnerOnlyPolicy.
func NewOwnerOnlyPolicy() *OwnerOnlyPolicy {
	return &OwnerOnlyPolicy{}
}

// Role returns RoleOwner when the actor is the data owner (actor == scope),
// and RoleNone otherwise. This preserves the pre-T2 behavior where every
// user only ever sees their own data.
func (OwnerOnlyPolicy) Role(_ context.Context, actor, scope uuid.UUID) (sharedpolicy.Role, error) {
	if actor == scope {
		return sharedpolicy.RoleOwner, nil
	}
	return sharedpolicy.RoleNone, nil
}

// Compile-time check that OwnerOnlyPolicy implements sharedpolicy.Policy.
var _ sharedpolicy.Policy = (*OwnerOnlyPolicy)(nil)
