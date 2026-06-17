package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type PropertyRepository interface {
	ExistsActiveByOwner(ctx context.Context, id, ownerID uuid.UUID) (bool, error)
	ExistsByOwner(ctx context.Context, id, ownerID uuid.UUID) (bool, error)
	HasOpenLease(ctx context.Context, id uuid.UUID) (bool, error)
}

type TenantContactRepository interface {
	Create(ctx context.Context, ownerID uuid.UUID, contact domain.TenantContact) (domain.TenantContact, error)
	GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.TenantContact, error)
	Update(ctx context.Context, ownerID uuid.UUID, contact domain.TenantContact) (domain.TenantContact, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContact, error)
}

type LeaseRepository interface {
	Create(ctx context.Context, ownerID uuid.UUID, lease domain.Lease) (domain.Lease, error)
	GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Lease, error)
	Update(ctx context.Context, ownerID uuid.UUID, lease domain.Lease) (domain.Lease, error)
	Complete(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error)
	CountOpenLeasesByProperty(ctx context.Context, propertyID uuid.UUID) (int, error)
	GetOpenLeaseByProperty(ctx context.Context, propertyID uuid.UUID) (domain.Lease, error)
	ListOpenLeasesWithPastEndDate(ctx context.Context, asOf time.Time) ([]domain.Lease, error)
	WithTx(tx transaction.Tx) LeaseRepository
}

type RecurringOperationRepository interface {
	Create(ctx context.Context, op domain.RecurringOperation) (domain.RecurringOperation, error)
	GetByLease(ctx context.Context, leaseID uuid.UUID) (domain.RecurringOperation, error)
	GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.RecurringOperation, error)
	ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.RecurringOperation, error)
	Update(ctx context.Context, op domain.RecurringOperation) (domain.RecurringOperation, error)
	UpdateStatus(ctx context.Context, id, ownerID uuid.UUID, status string) (domain.RecurringOperation, error)
	DeleteByLease(ctx context.Context, leaseID uuid.UUID) error
	WithTx(tx transaction.Tx) RecurringOperationRepository
}

type OperationRepository interface {
	Create(ctx context.Context, op domain.Operation) (domain.Operation, error)
	BulkCreate(ctx context.Context, ops []domain.Operation) error
	ListByLease(ctx context.Context, leaseID uuid.UUID) ([]domain.Operation, error)
	ListOperationDatesByLease(ctx context.Context, leaseID uuid.UUID) ([]time.Time, error)
	ListOperationDatesByRecurringOperation(ctx context.Context, recurringOperationID uuid.UUID) ([]time.Time, error)
	ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.Operation, error)
	GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.Operation, error)
	Update(ctx context.Context, op domain.Operation) (domain.Operation, error)
	SoftDeleteOperation(ctx context.Context, id, ownerID uuid.UUID) error
	DeleteUneditedFutureOperationsByLease(ctx context.Context, leaseID uuid.UUID, after time.Time) error
	DeleteUneditedFutureOperationsByRecurringOperation(ctx context.Context, recurringOperationID uuid.UUID, after time.Time) error
	DeleteOperationsOutsideLeaseRange(ctx context.Context, leaseID uuid.UUID, start time.Time, end *time.Time) error
	DeleteUneditedOperationsByLease(ctx context.Context, leaseID uuid.UUID) error
	WithTx(tx transaction.Tx) OperationRepository
}
