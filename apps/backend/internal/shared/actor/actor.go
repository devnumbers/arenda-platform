// Package actor holds the shared identity kernel: the roles that thread
// through every bounded context for authorization and audit.
//
// Role is the canonical home for owner/admin role values (ADR 0034). The string
// values are identical to the policy Role defined in shared/policy (ADR 0028),
// so the two can be mapped by value where a caller bridges the account model
// and authorization. Identity keeps its own ubiquitous-language type as a
// re-export (type alias) of actor.Role; audit and platform layers consume
// actor.Role directly.
package actor

import (
	"errors"
	"fmt"
)

// ErrInvalidRole is returned by NewRole when the raw string is not a known role.
var ErrInvalidRole = errors.New("invalid role")

// Role is a user's role within the system. It is the shared value that every
// context's authorization and audit needs; the full identity aggregate
// (User, Email, Phone, Session) stays in identity/domain.
type Role string

const (
	// RoleOwner is the role of a user who owns properties and manages rental data.
	RoleOwner Role = "owner"
	// RoleAdmin is the role of an internal support user.
	RoleAdmin Role = "admin"
)

// NewRole validates and creates a Role from a raw string. It returns
// [ErrInvalidRole] (wrapping the offending value) when the string is not a
// known role.
func NewRole(raw string) (Role, error) {
	switch Role(raw) {
	case RoleOwner, RoleAdmin:
		return Role(raw), nil
	default:
		return "", fmt.Errorf("invalid role %q: %w", raw, ErrInvalidRole)
	}
}

// String returns the string representation of the role.
func (r Role) String() string {
	return string(r)
}
