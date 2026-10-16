package policy

import (
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
)

// AuditActorRole maps a Role onto the audit ActorRole that records
// property-scoped actions: a shared-access member is attributed to their
// real role instead of being masked as the owner's own (issue #166). RoleNone
// and any unknown role fall back to the historical owner attribution.
// RoleSuspended falls back too: a suspended membership carries no attribution
// of its own — the write gate reachable in that state (self-exit) attributes
// from the membership's stored role before the mapping, so the fallback sees
// it only by accident (issue #859).
//
// The adapter lives here, not in the audit domain, so audit keeps its
// independence from the policy port — the dependency runs this way only.
func AuditActorRole(r Role) auditdomain.ActorRole {
	switch r {
	case RoleFullAccess:
		return auditdomain.ActorRoleFullAccess
	case RoleViewer:
		return auditdomain.ActorRoleViewer
	default:
		return auditdomain.ActorRoleOwner
	}
}
