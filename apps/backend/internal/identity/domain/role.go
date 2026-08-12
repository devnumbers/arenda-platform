package domain

import (
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// Role is a user's role within the system. It is a re-export of the canonical
// shared-kernel type [actor.Role] (ADR 0034); identity keeps it as part of its
// ubiquitous language while the canonical home for role values is shared/actor.
//
// The type alias means identity/domain consumers (AuditActorRole, the User
// aggregate, httpsupport role checks) continue to work unchanged.
type Role = actor.Role

// RoleOwner and RoleAdmin are re-exports of the canonical shared-kernel role
// constants from [actor], preserved here for identity's ubiquitous language.
const (
	RoleOwner = actor.RoleOwner
	RoleAdmin = actor.RoleAdmin
)

// NewRole validates and creates a Role from a raw string. It delegates to
// [actor.NewRole] so the canonical validation lives in one place; the error is
// re-wrapped with [ErrInvalidRole] so identity callers can keep matching on the
// domain sentinel.
func NewRole(raw string) (Role, error) {
	r, err := actor.NewRole(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %q", ErrInvalidRole, raw)
	}
	return r, nil
}

// String returns the string representation of the role. It is inherited from
// the underlying [actor.Role] via the type alias.
