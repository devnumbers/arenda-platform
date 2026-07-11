package domain

import (
	"fmt"
)

// Role represents a user's role within the system.
type Role string

const (
	RoleOwner Role = "owner"
	RoleAdmin Role = "admin"
)

// NewRole validates and creates a Role from a raw string.
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
