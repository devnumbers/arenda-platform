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

// MemberStatus is the lifecycle state of a membership. An active membership
// occupies a tariff slot in the recipient's limit and grants access; a
// suspended membership is hidden from the recipient (no slot, no access) when
// the recipient's tariff limit is exceeded, and is recovered FIFO when a slot
// frees up. See issue #158 (T4).
type MemberStatus string

const (
	// MemberStatusActive occupies a tariff slot and grants access to the object.
	MemberStatusActive MemberStatus = "active"
	// MemberStatusSuspended is hidden from the recipient (no slot, no access)
	// until a slot frees up and the membership is recovered FIFO.
	MemberStatusSuspended MemberStatus = "suspended"
)

// ParseMemberStatus parses a membership status string.
func ParseMemberStatus(s string) (MemberStatus, error) {
	switch MemberStatus(s) {
	case MemberStatusActive, MemberStatusSuspended:
		return MemberStatus(s), nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidMemberStatus, s)
	}
}

// String returns the string representation of the member status.
func (ms MemberStatus) String() string { return string(ms) }

// Membership is a single shared-access grant on a property.
type Membership struct {
	ID          uuid.UUID
	PropertyID  uuid.UUID
	UserID      uuid.UUID
	Role        Role
	GrantedBy   uuid.UUID
	Status      MemberStatus
	SuspendedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsSuspended reports whether the membership is suspended (hidden from the
// recipient due to a tariff slot shortage).
func (m Membership) IsSuspended() bool { return m.Status == MemberStatusSuspended }

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
	// ErrInvalidMemberStatus is returned when a membership status string is not
	// a recognized member status value.
	ErrInvalidMemberStatus = errors.New("invalid membership status")
	// ErrCannotLeaveSuspended is returned when a recipient tries to self-exit a
	// suspended membership. Self-exit is not available for suspended access
	// because the object is already hidden from the recipient.
	ErrCannotLeaveSuspended = errors.New("cannot leave a suspended membership")
	// ErrPropertyArchived is returned when a new member or invitation is added
	// to an archived property. Existing members keep working: revoking and role
	// changes stay available on archived objects (issue #163).
	ErrPropertyArchived = errors.New("cannot add members to an archived property")
)
