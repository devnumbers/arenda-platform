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
	HasOpenLease(ctx context.Context, id uuid.UUID) (bool, error)
}

type TenantContactRepository interface {
	Create(ctx context.Context, ownerID uuid.UUID, contact domain.TenantContact) (domain.TenantContact, error)
	GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.TenantContact, error)
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
	WithTx(tx transaction.Tx) LeaseRepository
}

type RecurringOperationRepository interface {
	Create(ctx context.Context, op domain.RecurringOperation) (domain.RecurringOperation, error)
	GetByLease(ctx context.Context, leaseID uuid.UUID) (domain.RecurringOperation, error)
	Update(ctx context.Context, op domain.RecurringOperation) (domain.RecurringOperation, error)
	DeleteByLease(ctx context.Context, leaseID uuid.UUID) error
	WithTx(tx transaction.Tx) RecurringOperationRepository
}

type OperationRepository interface {
	BulkCreate(ctx context.Context, ops []domain.Operation) error
	ListByLease(ctx context.Context, leaseID uuid.UUID) ([]domain.Operation, error)
	DeleteUneditedFutureOperationsByLease(ctx context.Context, leaseID uuid.UUID, after time.Time) error
	WithTx(tx transaction.Tx) OperationRepository
}
