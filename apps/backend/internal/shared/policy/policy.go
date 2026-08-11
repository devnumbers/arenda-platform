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
	// RoleFullAccess is the role of a property member with full access (issue
	// #156, T3): all data operations and equal member management, but no
	// lifecycle control.
	RoleFullAccess Role = "full_access"
	// RoleViewer is the role of a property member with view-only access (issue
	// #156, T3): read-only of all object data, no exceptions.
	RoleViewer Role = "viewer"
	// RoleSuspended is the role of an actor whose property membership is
	// suspended because their tariff's active-property limit is exceeded
	// (issue #158, T4). It grants no capabilities, but unlike RoleNone it is
	// distinguishable: the property page entry point maps it to a dedicated
	// "access suspended" outcome so the UI can show an honest screen (T9).
	RoleSuspended Role = "suspended"
)

// Policy is the single point of authorization for the application. It maps an
// actor and a data owner to a Role, which in turn determines which capabilities
// the actor has over the owner's data.
type Policy interface {
	// Role returns the authorization role of the actor relative to the scope
	// (the data owner). The role determines which capabilities the actor has
	// over the scope's owner-wide data (operation categories, tenant contacts
	// and other account-level entities). For the owner's own data actor ==
	// scope and the role is RoleOwner; for an actor with property memberships the
	// role is derived as the strongest role across the scope's properties;
	// otherwise RoleNone.
	Role(ctx context.Context, actor, scope uuid.UUID) (Role, error)

	// RoleForProperty returns the authorization role of the actor relative to a
	// specific property (issue #156, T3). It resolves the property's owner and
	// any membership the actor holds on that property: the owner gets
	// RoleOwner, a property member gets RoleFullAccess or RoleViewer, and
	// anyone else gets RoleNone. A suspended membership gets RoleSuspended
	// (T9): no capabilities, but distinguishable from RoleNone. Callers that
	// render object privacy must map RoleNone to a "not found" outcome so the
	// existence of an object is never revealed.
	RoleForProperty(ctx context.Context, actor, propertyID uuid.UUID) (Role, error)
}

// CanView reports whether the role may read the scope's data.
func CanView(r Role) bool {
	return r == RoleOwner || r == RoleFullAccess || r == RoleViewer
}

// CanEdit reports whether the role may modify the scope's data.
func CanEdit(r Role) bool {
	return r == RoleOwner || r == RoleFullAccess
}

// CanManageMembers reports whether the role may manage property members (invite,
// change role, revoke).
func CanManageMembers(r Role) bool {
	return r == RoleOwner || r == RoleFullAccess
}

// CanLifecycle reports whether the role may change the property lifecycle
// (archive, unarchive, delete). Only the owner may do this.
func CanLifecycle(r Role) bool {
	return r == RoleOwner
}
