// Package domain defines the access bounded context domain model: property
// memberships that grant shared access to an object (issue #156, T3).
package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Role is the membership role a user holds on a property. It is distinct from
// sharedpolicy.Role, which additionally encodes the owner and none cases
// derived from properties.owner_id and the absence of a membership row.
type Role string

const (
	// RoleFullAccess grants all data operations and equal member management.
	RoleFullAccess Role = "full_access"
	// RoleViewer grants read-only access to all object data.
	RoleViewer Role = "viewer"
)

// ParseRole parses a membership role string. Only full_access and viewer are
// valid — the owner role is never stored as a membership.
func ParseRole(s string) (Role, error) {
	switch Role(s) {
	case RoleFullAccess, RoleViewer:
		return Role(s), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidRole, s)
	}
}

// String returns the string representation of the role.
func (r Role) String() string { return string(r) }

// Membership is a single shared-access grant on a property.
type Membership struct {
	ID         uuid.UUID
	PropertyID uuid.UUID
	UserID     uuid.UUID
	Role       Role
	GrantedBy  uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Sentinel errors for the access domain.
var (
	ErrInvalidRole         = errors.New("invalid membership role")
	ErrMemberNotFound      = errors.New("membership not found")
	ErrMemberAlreadyExists = errors.New("membership already exists")
	// ErrCannotAddOwner is returned when an attempt is made to add the object
	// owner as a member. The owner is the single source of truth in
	// properties.owner_id and cannot be a member.
	ErrCannotAddOwner = errors.New("owner cannot be a member")
	// ErrCannotAddSelf is returned when an actor tries to grant themselves
	// access via the member API. Ownership is the only self-granted access.
	ErrCannotAddSelf = errors.New("cannot add yourself as a member")
	// ErrCannotRevokeOwner is returned when an attempt is made to revoke the
	// owner's access.
	ErrCannotRevokeOwner = errors.New("owner access cannot be revoked")
	// ErrCannotLeaveOwnProperty is returned when the owner attempts to leave
	// their own object via self-exit.
	ErrCannotLeaveOwnProperty = errors.New("owner cannot leave their own property")
)
