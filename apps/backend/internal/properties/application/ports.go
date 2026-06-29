package application

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// AddressSuggestion is a domain-friendly address hint returned by a suggestion
// provider such as DaData.
type AddressSuggestion struct {
	Value string
	City  string
}

// AddressSuggester returns address suggestions for a partial user query.
type AddressSuggester interface {
	SuggestAddresses(ctx context.Context, query string) ([]AddressSuggestion, error)
}

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
	GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.Property, error)
	ListActiveByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Property, error)
	Update(ctx context.Context, ownerID uuid.UUID, property domain.Property) (domain.Property, error)
	Archive(ctx context.Context, id, ownerID uuid.UUID) error
	Unarchive(ctx context.Context, id, ownerID uuid.UUID) error
	CountActiveByOwner(ctx context.Context, ownerID uuid.UUID) (int, error)
	WithTx(tx transaction.Tx) PropertyRepository
}

// LeaseRepository provides lease data needed by the properties bounded context.
type LeaseRepository interface {
	ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]leasesdomain.Lease, error)
}

// PhotoStorage persists uploaded property photos and returns their public URL.
type PhotoStorage interface {
	Upload(ctx context.Context, key string, contentType string, size int64, data io.Reader) (string, error)
	Delete(ctx context.Context, key string) error
	HeadBucket(ctx context.Context) error
}

// PropertyPhotoRepository persists photo metadata for properties.
type PropertyPhotoRepository interface {
	Create(ctx context.Context, propertyID uuid.UUID, url string) (domain.Photo, error)
	GetByID(ctx context.Context, photoID uuid.UUID) (domain.Photo, error)
	GetByIDAndPropertyID(ctx context.Context, photoID, propertyID uuid.UUID) (domain.Photo, error)
	GetByPropertyID(ctx context.Context, propertyID uuid.UUID) ([]domain.Photo, error)
	GetByPropertyIDs(ctx context.Context, propertyIDs []uuid.UUID) (map[uuid.UUID][]domain.Photo, error)
	CountByPropertyID(ctx context.Context, propertyID uuid.UUID) (int, error)
	Delete(ctx context.Context, photoID uuid.UUID) error
	WithTx(tx transaction.Tx) PropertyPhotoRepository
}
