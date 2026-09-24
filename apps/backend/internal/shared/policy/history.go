package policy

import (
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
)

// HistoryActorRole maps a Role onto the action journal's ActorRole (ADR 0061
// §3): a shared-access member is attributed to their real role, the audit
// pattern (AuditActorRole). RoleNone and any unknown role fall back to the
// owner attribution. RoleSuspended falls back too: a suspended membership
// carries no journal attribution of its own — the write gates reachable in
// that state (e.g. self-exit) attribute from the membership's stored role
// before the mapping, so the fallback sees them only by accident.
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
