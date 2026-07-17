package application

import (
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// AuditActorRole maps an identity role to the audit actor role, defaulting to
// owner for unknown roles.
func AuditActorRole(role domain.Role) auditdomain.ActorRole {
	if role == domain.RoleAdmin {
		return auditdomain.ActorRoleAdmin
	}
	return auditdomain.ActorRoleOwner
}
