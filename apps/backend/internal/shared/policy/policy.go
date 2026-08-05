package policy

import (
	"context"

	"github.com/google/uuid"
)

// Role is the authorization role of an actor relative to a data owner (scope).
type Role string

const (
	// RoleNone means the actor has no access to the scope's data.
	RoleNone Role = "none"
	// RoleOwner means the actor is the data owner themselves.
	RoleOwner Role = "owner"
	// RoleFullAccess is reserved for property members with full access (T3); not returned by the policy yet.
	RoleFullAccess Role = "full_access"
	// RoleViewer is reserved for property members with view-only access (T3); not returned by the policy yet.
	RoleViewer Role = "viewer"
)

// Policy is the single point of authorization for the application. It maps an
// actor and a scope (the data owner) to a Role, which in turn determines which
// capabilities the actor has over the scope's data.
type Policy interface {
	// Role returns the authorization role of the actor relative to the scope (the
	// data owner). The role determines which capabilities the actor has over the
	// scope's data. For the owner's own data actor == scope and the role is
	// RoleOwner; for anyone else it is RoleNone until property membership is
	// introduced (T3).
	Role(ctx context.Context, actor, scope uuid.UUID) (Role, error)
}

// CanView reports whether the role may read the scope's data.
func CanView(r Role) bool {
	return r == RoleOwner || r == RoleFullAccess || r == RoleViewer
}

// CanEdit reports whether the role may modify the scope's data.
func CanEdit(r Role) bool {
	return r == RoleOwner || r == RoleFullAccess
}

// CanManageMembers reports whether the role may manage property members (invite, change role, revoke).
func CanManageMembers(r Role) bool {
	return r == RoleOwner || r == RoleFullAccess
}

// CanLifecycle reports whether the role may change the property lifecycle (archive, unarchive, delete). Only the owner may do this.
func CanLifecycle(r Role) bool {
	return r == RoleOwner
}
