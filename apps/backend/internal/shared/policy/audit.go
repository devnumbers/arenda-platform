package policy

import (
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
)

// AuditActorRole maps a Role onto the audit ActorRole that records
// property-scoped actions: a shared-access member is attributed to their
// real role instead of being masked as the owner's own (issue #166). Roles
// that never reach a write gate (none, suspended) and any unknown role fall
// back to the historical owner attribution.
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
