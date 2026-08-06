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

// SharedPropertyIDs returns the ids of properties shared with a user via
// property membership (issue #156, T3). Implemented by the access bounded
// context and injected optionally: when nil, only the owner's own properties
// are listed (the pre-T3 behaviour).
type SharedPropertyIDs interface {
	SharedWith(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}

// SuspendedSharedCounter reports how many shared memberships of a recipient
// are currently suspended (hidden from the recipient's property list due to a
// tariff slot shortage). Implemented by the access bounded context and injected
// optionally: when nil, the properties list reports zero hidden shared.
// See issue #158 (T4).
type SuspendedSharedCounter interface {
	CountSuspendedByUser(ctx context.Context, userID uuid.UUID) (int, error)
}

// RecipientSlotPolicy enforces the recipient tariff slot invariant for the
// shared-access memberships of a single property. It is implemented by the
// access bounded context's SlotCoordinator and injected optionally: when nil,
// no slot policy runs (pre-T4 behaviour). RecoverSuspendedForProperty is called
// when the owner archives a shared object (a slot freed for each recipient);
// EnforceOnUnarchiveForProperty is called when the owner unarchives a shared
// object (the object re-enters the recipients' pool); RecoverAfterPropertyDelete
// is called before the owner deletes a shared object (the memberships are
// dropped and the freed slots recovered FIFO). See issue #158 (T4).
type RecipientSlotPolicy interface {
	RecoverSuspendedForProperty(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID) error
	EnforceOnUnarchiveForProperty(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID) error
	RecoverAfterPropertyDelete(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID) error
}

type SubscriptionLimiter interface {
	ActivePropertyLimit(ctx context.Context, userID uuid.UUID) (int, error)
	WithTx(tx transaction.Tx) (SubscriptionLimiter, error)
}

// OccupancyProvider reports which properties have an open lease.
type OccupancyProvider interface {
	IsOccupied(ctx context.Context, scope, propertyID uuid.UUID) (bool, error)
	OccupiedPropertyIDs(ctx context.Context, scope uuid.UUID) (map[uuid.UUID]bool, error)
	WithTx(tx transaction.Tx) OccupancyProvider
}

// PropertyBillingLifecycle manages the billing side effects of archiving and
// unarchiving a property.
type PropertyBillingLifecycle interface {
	Suspend(ctx context.Context, propertyID, scope uuid.UUID, asOf time.Time) error
	Resume(ctx context.Context, propertyID uuid.UUID, scope uuid.UUID, asOf time.Time) error
	// CompleteOpenLeases force-completes all open leases of the property,
	// applying the same side effects as a user-initiated lease completion.
	CompleteOpenLeases(ctx context.Context, scope, propertyID uuid.UUID, asOf time.Time) error
	WithTx(tx transaction.Tx) PropertyBillingLifecycle
}

type PropertyRepository interface {
	Create(ctx context.Context, scope uuid.UUID, property domain.Property) (domain.Property, error)
	GetByIDAndOwner(ctx context.Context, id, scope uuid.UUID) (domain.Property, error)
	GetByIDAndOwnerForUpdate(ctx context.Context, id, scope uuid.UUID) (domain.Property, error)
	// GetByID returns a property by id without owner scoping. Used by the
	// policy/access layer (T3, issue #156) to resolve the data owner for
	// authorization before applying a scope. Callers must not leak existence to
	// actors without a view capability.
	GetByID(ctx context.Context, id uuid.UUID) (domain.Property, error)
	// GetByIDForUpdate is the pessimistic-lock variant of GetByID.
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Property, error)
	ListActiveByOwner(ctx context.Context, scope uuid.UUID) ([]domain.Property, error)
	ListArchivedByOwner(ctx context.Context, scope uuid.UUID) ([]domain.Property, error)
	Update(ctx context.Context, scope uuid.UUID, property domain.Property) (domain.Property, error)
	Archive(ctx context.Context, id, scope uuid.UUID) error
	Unarchive(ctx context.Context, id, scope uuid.UUID) error
	CountActiveByOwner(ctx context.Context, scope uuid.UUID) (int, error)
	Delete(ctx context.Context, id, scope uuid.UUID) error
	DeleteOperationsByProperty(ctx context.Context, scope, propertyID uuid.UUID) error
	DeleteRecurringOperationsByProperty(ctx context.Context, scope, propertyID uuid.UUID) error
	DeleteLeasesByProperty(ctx context.Context, scope, propertyID uuid.UUID) error
	WithTx(tx transaction.Tx) PropertyRepository
}

// LeaseRepository provides lease data needed by the properties bounded context.
type LeaseRepository interface {
	ListByProperty(ctx context.Context, scope, propertyID uuid.UUID) ([]leasesdomain.Lease, error)
	GetOpenLeaseByProperty(ctx context.Context, scope, propertyID uuid.UUID) (leasesdomain.Lease, error)
}

// PhotoStorage persists uploaded property photos and returns their public URL.
type PhotoStorage interface {
	Upload(ctx context.Context, key string, contentType string, size int64, data io.Reader) (string, error)
	Delete(ctx context.Context, key string) error
	HeadBucket(ctx context.Context) error
}

// PropertyPhotoRepository persists photo metadata for properties.
type PropertyPhotoRepository interface {
	Create(ctx context.Context, photoID, propertyID uuid.UUID, url string) (domain.Photo, error)
	GetByID(ctx context.Context, photoID uuid.UUID) (domain.Photo, error)
	GetByIDAndPropertyID(ctx context.Context, photoID, propertyID uuid.UUID) (domain.Photo, error)
	GetByPropertyID(ctx context.Context, propertyID uuid.UUID) ([]domain.Photo, error)
	GetByPropertyIDs(ctx context.Context, propertyIDs []uuid.UUID) (map[uuid.UUID][]domain.Photo, error)
	CountByPropertyID(ctx context.Context, propertyID uuid.UUID) (int, error)
	Delete(ctx context.Context, photoID uuid.UUID) error
	WithTx(tx transaction.Tx) PropertyPhotoRepository
}

// PropertyContactRepository persists property contact records.
type PropertyContactRepository interface {
	Create(ctx context.Context, contact domain.PropertyContact) (domain.PropertyContact, error)
	ListByProperty(ctx context.Context, propertyID, scope uuid.UUID) ([]domain.PropertyContact, error)
	GetByIDAndOwner(ctx context.Context, contactID, scope uuid.UUID) (domain.PropertyContact, error)
	Update(ctx context.Context, scope uuid.UUID, contact domain.PropertyContact) (domain.PropertyContact, error)
	Delete(ctx context.Context, contactID, scope uuid.UUID) error
	WithTx(tx transaction.Tx) PropertyContactRepository
}
