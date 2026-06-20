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
	WithTx(tx transaction.Tx) SubscriptionLimiter
}

// OccupancyProvider reports which properties have an open lease.
type OccupancyProvider interface {
	IsOccupied(ctx context.Context, ownerID, propertyID uuid.UUID) (bool, error)
	OccupiedPropertyIDs(ctx context.Context, ownerID uuid.UUID) (map[uuid.UUID]bool, error)
	WithTx(tx transaction.Tx) OccupancyProvider
}

// PropertyBillingLifecycle manages the billing side effects of archiving and
// unarchiving a property.
type PropertyBillingLifecycle interface {
	Suspend(ctx context.Context, propertyID uuid.UUID, asOf time.Time) error
	Resume(ctx context.Context, propertyID uuid.UUID, ownerID uuid.UUID, asOf time.Time) error
	WithTx(tx transaction.Tx) PropertyBillingLifecycle
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
