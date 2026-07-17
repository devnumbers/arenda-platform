package application

import (
	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
)

// paymentAuditActor maps the actor id passed to admin-triggered payment
// operations to the audit actor fields: uuid.Nil marks a system-initiated
// call (reconciliation workers), any other id is an admin.
func paymentAuditActor(actorID uuid.UUID) (*uuid.UUID, auditdomain.ActorRole) {
	if actorID == uuid.Nil {
		return nil, auditdomain.ActorRoleSystem
	}
	return &actorID, auditdomain.ActorRoleAdmin
}
