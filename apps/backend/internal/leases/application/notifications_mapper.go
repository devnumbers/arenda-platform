package application

import (
	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// ToOperationInfo maps a lease domain operation to the notification scheduling info.
func ToOperationInfo(op domain.Operation) notificationsapp.OperationInfo {
	var leaseID, recurringOperationID *uuid.UUID
	if op.LeaseID != uuid.Nil {
		leaseID = &op.LeaseID
	}
	if op.RecurringOperationID != uuid.Nil {
		recurringOperationID = &op.RecurringOperationID
	}
	return notificationsapp.OperationInfo{
		ID:                   op.ID,
		OwnerID:              op.OwnerID,
		PropertyID:           op.PropertyID,
		LeaseID:              leaseID,
		RecurringOperationID: recurringOperationID,
		OperationDate:        op.OperationDate,
		Type:                 string(op.Type),
		Category:             string(op.Category),
		AmountKopecks:        op.AmountKopecks,
	}
}

// ToOperationInfoSlice maps a slice of domain operations to notification scheduling infos.
func ToOperationInfoSlice(ops []domain.Operation) []notificationsapp.OperationInfo {
	out := make([]notificationsapp.OperationInfo, len(ops))
	for i, op := range ops {
		out[i] = ToOperationInfo(op)
	}
	return out
}
