package policy

import (
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
)

// HistoryActorRole maps a Role onto the action journal's ActorRole (ADR 0061
// §3): a shared-access member is attributed to their real role, the audit
// pattern (AuditActorRole). Roles that never reach a write gate (none,
// suspended) and any unknown role fall back to the owner attribution.
//
// The adapter lives here, not in the history domain, so history keeps its
// independence from the policy port — the dependency runs this way only.
func HistoryActorRole(r Role) historydomain.ActorRole {
	switch r {
	case RoleFullAccess:
		return historydomain.ActorRoleFullAccess
	case RoleViewer:
		return historydomain.ActorRoleViewer
	default:
		return historydomain.ActorRoleOwner
	}
}
