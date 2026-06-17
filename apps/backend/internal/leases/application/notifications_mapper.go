package application

import (
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// ToOperationInfo maps a lease domain operation to the notification scheduling info.
func ToOperationInfo(op domain.Operation) notificationsapp.OperationInfo {
	return notificationsapp.OperationInfo{
		ID:                   op.ID,
		OwnerID:              op.OwnerID,
		PropertyID:           op.PropertyID,
		LeaseID:              domain.LeaseIDPtr(op.LeaseID),
		RecurringOperationID: domain.LeaseIDPtr(op.RecurringOperationID),
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
