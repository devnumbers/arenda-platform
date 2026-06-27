package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type PropertyRepository interface {
	ExistsActiveByOwner(ctx context.Context, id, ownerID uuid.UUID) (bool, error)
	ExistsByOwner(ctx context.Context, id, ownerID uuid.UUID) (bool, error)
	HasOpenLease(ctx context.Context, id uuid.UUID) (bool, error)
	WithTx(tx transaction.Tx) PropertyRepository
}

type TenantContactRepository interface {
	Create(ctx context.Context, ownerID uuid.UUID, contact domain.TenantContact) (domain.TenantContact, error)
	GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.TenantContact, error)
	ListByIDs(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) ([]domain.TenantContact, error)
	Update(ctx context.Context, ownerID uuid.UUID, contact domain.TenantContact) (domain.TenantContact, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContact, error)
	ListWithLeaseStatus(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContactWithLeases, error)
	WithTx(tx transaction.Tx) TenantContactRepository
}

type LeaseRepository interface {
	Create(ctx context.Context, ownerID uuid.UUID, lease domain.Lease) (domain.Lease, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Lease, error)
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Lease, error)
	GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error)
	GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Lease, error)
	Update(ctx context.Context, ownerID uuid.UUID, lease domain.Lease) (domain.Lease, error)
	Complete(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error)
	CountOpenLeasesByProperty(ctx context.Context, propertyID uuid.UUID) (int, error)
	GetOpenLeaseByProperty(ctx context.Context, propertyID uuid.UUID) (domain.Lease, error)
	ListOpenLeasesWithPastEndDate(ctx context.Context, asOf time.Time, limit int) ([]domain.Lease, error)
	ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.Lease, error)
	WithTx(tx transaction.Tx) LeaseRepository
}

type RecurringOperationRepository interface {
	Create(ctx context.Context, op domain.RecurringOperation) (domain.RecurringOperation, error)
	GetByLeaseID(ctx context.Context, ownerID, leaseID uuid.UUID) (domain.RecurringOperation, error)
	GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.RecurringOperation, error)
	GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.RecurringOperation, error)
	ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.RecurringOperation, error)
	ListByPropertyID(ctx context.Context, propertyID uuid.UUID) ([]domain.RecurringOperation, error)
	Update(ctx context.Context, op domain.RecurringOperation) (domain.RecurringOperation, error)
	UpdateStatus(ctx context.Context, id, ownerID uuid.UUID, status string) (domain.RecurringOperation, error)
	UpdateStatusByLeaseID(ctx context.Context, leaseID, ownerID uuid.UUID, status string) error
	SetReminderOffset(ctx context.Context, ownerID, recID uuid.UUID, offsetDays int) error
	DeleteByLease(ctx context.Context, leaseID uuid.UUID) error
	WithTx(tx transaction.Tx) RecurringOperationRepository
}

// ReminderScheduler is the port used by leases services to schedule/cancel reminders.
type ReminderScheduler = notificationsapp.ReminderScheduler

type OperationFilter struct {
	Types                []domain.OperationType
	Statuses             []domain.OperationStatus
	Categories           []domain.OperationCategory
	PropertyID           uuid.UUID
	FromDate             *time.Time
	ToDate               *time.Time
	RecurringOperationID uuid.UUID
	Limit                int
	Offset               int
}

type FinanceReportTotals struct {
	IncomeKopecks  int64
	ExpenseKopecks int64
}

type FinanceReportPropertyRow struct {
	PropertyID     uuid.UUID
	IncomeKopecks  int64
	ExpenseKopecks int64
}

type FinanceReportCategoryRow struct {
	Type         domain.OperationType
	Category     domain.OperationCategory
	TotalKopecks int64
}

type FinanceReportMonthRow struct {
	Month          time.Time
	IncomeKopecks  int64
	ExpenseKopecks int64
}

type OperationRepository interface {
	Create(ctx context.Context, op domain.Operation) (domain.Operation, error)
	BulkCreate(ctx context.Context, ops []domain.Operation) error
	ListByOwner(ctx context.Context, ownerID uuid.UUID, filter OperationFilter) ([]domain.Operation, error)
	ListByLease(ctx context.Context, leaseID uuid.UUID) ([]domain.Operation, error)
	ListByRecurringOperation(ctx context.Context, recurringOperationID uuid.UUID) ([]domain.Operation, error)
	ListOperationDatesByLease(ctx context.Context, leaseID uuid.UUID) ([]time.Time, error)
	ListOperationDatesByRecurringOperation(ctx context.Context, recurringOperationID uuid.UUID) ([]time.Time, error)
	ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.Operation, error)
	ListByPropertyWithStatuses(ctx context.Context, ownerID, propertyID uuid.UUID, statuses []domain.OperationStatus) ([]domain.Operation, error)
	GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.Operation, error)
	GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.Operation, error)
	Update(ctx context.Context, op domain.Operation) (domain.Operation, error)
	MarkOverdue(ctx context.Context, ownerID, id uuid.UUID, asOf time.Time) (domain.Operation, bool, error)
	SoftDeleteOperation(ctx context.Context, id, ownerID uuid.UUID) error
	DeleteUneditedFutureOperationsByLease(ctx context.Context, leaseID uuid.UUID, after time.Time) error
	DeleteUneditedFutureOperationsByRecurringOperation(ctx context.Context, recurringOperationID uuid.UUID, after time.Time) error
	DeleteOperationsOutsideLeaseRange(ctx context.Context, leaseID uuid.UUID, start time.Time, end *time.Time) error
	DeleteUneditedOperationsByLease(ctx context.Context, leaseID uuid.UUID, from time.Time) error
	ListPendingOperationsWithPastDate(ctx context.Context, ownerID uuid.UUID, asOf time.Time, limit int) ([]domain.Operation, error)
	ListAllPendingOperationsWithPastDate(ctx context.Context, asOf time.Time, limit int) ([]domain.Operation, error)
	GetPropertyOperationsSummary(ctx context.Context, ownerID, propertyID uuid.UUID, asOf time.Time) (OperationsSummary, error)
	HasDepositReturnForLease(ctx context.Context, leaseID uuid.UUID) (bool, error)
	GetFinanceReportTotals(ctx context.Context, ownerID uuid.UUID, from, to time.Time) (FinanceReportTotals, error)
	GetFinanceReportByProperty(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]FinanceReportPropertyRow, error)
	GetFinanceReportByCategory(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]FinanceReportCategoryRow, error)
	GetFinanceReportByMonth(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]FinanceReportMonthRow, error)
	WithTx(tx transaction.Tx) OperationRepository
}
