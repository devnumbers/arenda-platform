package application

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
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

// SharedMembership is the active shared-access membership of a user on a
// property (issue T11): the property id and the recipient's role on it.
type SharedMembership struct {
	PropertyID uuid.UUID
	Role       sharedpolicy.Role
}

// SharedMemberships returns the active shared-access memberships of a user
// (property id + recipient role). Implemented by the access bounded context
// (issue T11). Optional: when nil, only the owner's own properties are listed.
type SharedMemberships interface {
	MembershipsWith(ctx context.Context, userID uuid.UUID) ([]SharedMembership, error)
}

// OwnerDisplayNameResolver resolves the public display name of a property
// owner for the sharing banner (issue T11). Optional.
type OwnerDisplayNameResolver interface {
	DisplayName(ctx context.Context, userID uuid.UUID) (string, error)
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
// dropped and the freed slots recovered FIFO). RecoverSuspended is called when
// an owner frees one of their OWN tariff slots (archive/delete of an own object,
// or the billing auto-archive): the owner is never a member row of their own
// object, so the per-property entries above do not visit them as a recipient —
// this per-recipient entry reactivates the owner's own suspended shared queue
// FIFO. See issue #158 (T4).
type RecipientSlotPolicy interface {
	RecoverSuspendedForProperty(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID) error
	EnforceOnUnarchiveForProperty(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID) error
	RecoverAfterPropertyDelete(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID) error
	RecoverSuspended(ctx context.Context, tx transaction.Tx, recipientID uuid.UUID) error
}

// SharedMembersDeleteMailer bridges the property deletion flow to the access
// sharing lifecycle emails (issue #162, T6). Implemented by the access bounded
// context and injected optionally: when nil, deleting a property sends no
// emails to former shared members. CollectFormerMemberEmails runs inside the
// delete transaction, before the slot policy drops the memberships (only
// active+suspended members are collected; pending invitations are not
// memberships and receive nothing). SendPropertyDeleted runs after the commit;
// a send failure is logged by the caller and never rolls anything back.
type SharedMembersDeleteMailer interface {
	CollectFormerMemberEmails(ctx context.Context, tx transaction.Tx, propertyID uuid.UUID) ([]string, error)
	SendPropertyDeleted(ctx context.Context, to, propertyTitle string) error
}

type SubscriptionLimiter interface {
	ActivePropertyLimit(ctx context.Context, userID uuid.UUID) (int, error)
	WithTx(tx transaction.Tx) (SubscriptionLimiter, error)
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
	// SetPin writes the global pin in one atomic UPDATE (ticket #577, the
	// PUT favorite's canon): nil clears it, a moment pins the property since
	// then. The scope is the property's owner — the edit capability (Owner,
	// Full Access) is resolved by the service's policy before the call.
	SetPin(ctx context.Context, id, scope uuid.UUID, pinnedAt *time.Time) (domain.Property, error)
	Archive(ctx context.Context, id, scope uuid.UUID) error
	Unarchive(ctx context.Context, id, scope uuid.UUID) error
	CountActiveByOwner(ctx context.Context, scope uuid.UUID) (int, error)
	Delete(ctx context.Context, id, scope uuid.UUID) error
	WithTx(tx transaction.Tx) PropertyRepository
}

// PhotoStorage persists uploaded property photos and returns their public URL.
type PhotoStorage interface {
	Upload(ctx context.Context, key, contentType string, size int64, data io.Reader) (string, error)
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
