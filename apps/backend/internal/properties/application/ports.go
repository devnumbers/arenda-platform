package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type SubscriptionLimiter interface {
	ActivePropertyLimit(ctx context.Context, userID uuid.UUID) (int, error)
}

// OccupancyProvider reports which of the given properties have an open lease.
type OccupancyProvider interface {
	OccupiedPropertyIDs(ctx context.Context, ownerID uuid.UUID, propertyIDs []uuid.UUID) (map[uuid.UUID]bool, error)
}

// RecurringOperation holds the fields of a recurring operation template needed
// by the property service to manage archive/unarchive side effects.
type RecurringOperation struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	PropertyID    uuid.UUID
	LeaseID       uuid.UUID
	Type          string
	Category      string
	AmountKopecks int64
	StartDate     time.Time
	PaymentDay    int
	EndDate       *time.Time
	Status        string
	Comment       string
}

// OperationArchiver removes future unedited operations for a property.
type OperationArchiver interface {
	DeleteFutureUneditedOperationsByProperty(ctx context.Context, propertyID uuid.UUID, fromDate time.Time) error
	WithTx(tx transaction.Tx) OperationArchiver
}

// RecurringOperationStatusUpdater lists and updates recurring operations for a property.
type RecurringOperationStatusUpdater interface {
	ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]RecurringOperation, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	WithTx(tx transaction.Tx) RecurringOperationStatusUpdater
}

// RecurringOperationScheduler generates operation instances for a recurring
// operation template from a given date, skipping existing dates.
type RecurringOperationScheduler interface {
	GenerateOperations(ctx context.Context, tx transaction.Tx, rec RecurringOperation, fromDate time.Time) error
	WithTx(tx transaction.Tx) RecurringOperationScheduler
}

type PropertyRepository interface {
	Create(ctx context.Context, ownerID uuid.UUID, property domain.Property) (domain.Property, error)
	GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.Property, error)
	ListActiveByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Property, error)
	Update(ctx context.Context, ownerID uuid.UUID, property domain.Property) (domain.Property, error)
	Archive(ctx context.Context, id, ownerID uuid.UUID) error
	Unarchive(ctx context.Context, id, ownerID uuid.UUID) error
	CountActiveByOwner(ctx context.Context, ownerID uuid.UUID) (int, error)
	WithTx(tx transaction.Tx) PropertyRepository
}
