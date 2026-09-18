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

// SharedSuspendedMembership is one suspended shared membership of the reading
// actor (ticket #702): the blur-card shown in the property list while the
// object is temporarily unavailable because the actor's tariff limit is
// exceeded. It carries the object's own card data (title and address — the
// card renders for real under the blur, Figma 2213-99113; photos are not
// wired — every card wears the same house-glyph avatar today) plus the owner
// contact the reason sheet renders (Figma 2229-100002). It replaces the
// hidden-shared count footnote (issues #158 T4, #163 — same predicate and
// FIFO order).
type SharedSuspendedMembership struct {
	PropertyID uuid.UUID
	Role       sharedpolicy.Role
	Name       string
	Address    string
	OwnerName  string
	OwnerEmail string
}

// SuspendedSharedMemberships lists the reading actor's suspended shared
// memberships (ticket #702) in the FIFO order the hidden-shared count used
// (#158 T4, #163): suspended memberships on non-archived properties.
// Implemented by the access bounded context and injected optionally — when
// nil, the list carries no placeholders.
type SuspendedSharedMemberships interface {
	SuspendedWith(ctx context.Context, userID uuid.UUID) ([]SharedSuspendedMembership, error)
}

// MemberNamesReader resolves the display names of the active members of the
// reading actor's own properties (ticket #702): the participant row on the
// owner's list cards («Участники», Figma 2200-97368). Implemented by the
// access bounded context and injected optionally — when nil, the list rows
// carry no names. Shared (non-owner) rows never receive names: the row is
// the owner's surface.
type MemberNamesReader interface {
	NamesByOwner(ctx context.Context, owner uuid.UUID) (map[uuid.UUID][]string, error)
}

// PropertyOwners maps the property ids of one list read onto their data
// owners (the projections resolve «today» and scope per data owner, ADR
// 0048); the rows carry the owner already, so the service passes it instead
// of every adapter re-reading the properties.
type PropertyOwners map[uuid.UUID]uuid.UUID

// OwnerCalendar resolves a user's calendar date (ADR 0048) — the list reads
// report the reading actor's «today» so the client counts the «Осталось
// N месяцев» rental badge against the right day boundary (ticket #586; the
// tasks feed's today rule, #521). Implemented by the calendar adapter shared
// with rentals/payments; optional — when nil, the lists fall back to the
// server clock's UTC date.
type OwnerCalendar interface {
	Today(ctx context.Context, userID uuid.UUID) (time.Time, error)
}

// PropertiesPage is one listing read: the merged rows (own + shared, the
// pinned first), the reading actor's calendar date (ADR 0048) the client
// renders the rental badges against (ticket #586), and the actor's
// suspended shared memberships as blur-card placeholders (ticket #702).
type PropertiesPage struct {
	Items []domain.Property
	Today time.Time
	// SuspendedShared lists the actor's suspended shared memberships in FIFO
	// order (ticket #702); empty when the port is unwired or the actor has
	// none. Only the main list fills it — archived objects hide their
	// suspended legs behind the archive itself (#163).
	SuspendedShared []SharedSuspendedMembership
}

// RentalOccupancyReader reports the per-property occupancy projection
// («Занятость объекта», резолюция #584, ticket #585) computed from each
// property's single unfinished rental against the data owner's today (ADR
// 0048). Implemented by the rentals bounded context; injected optionally —
// when nil, the list reads report no occupancy. Every listed property must
// be present in the result: without an unfinished rental it reports
// OccupancyNone.
type RentalOccupancyReader interface {
	OccupancyByProperty(ctx context.Context, owners PropertyOwners) (map[uuid.UUID]domain.Occupancy, error)
}

// OverdueOperationsReader reports which of the given properties have overdue
// planned operations — stored planned rows dated before the data owner's
// today (ADR 0048), the same predicate the payments listings resolve as the
// overdue view status (резолюция #584: the red dot never duplicates the
// computation). Implemented by the payments bounded context; injected
// optionally — when nil, the list reads report no overdue flags. Properties
// without overdue operations may miss from the result.
type OverdueOperationsReader interface {
	OverdueByProperty(ctx context.Context, owners PropertyOwners) (map[uuid.UUID]bool, error)
}

// RentalDeletionGuard gates the property deletion on the rentals bounded
// context (issue #632). HasUnfinished reports the property's unfinished
// rental — the delete is refused with ErrPropertyOccupied while the
// property is rented out; the owner completes the rental first (#627).
// DeleteByProperty removes the property's rental rows inside the caller's
// transaction BEFORE the property row itself: the explicit order (the ADR
// 0025 §2 precedent) defuses the rentals.payment_id RESTRICT FK — left to
// the FK machinery, the rentals and the payments cascades off the property
// row run in no guaranteed order and the RESTRICT kills the whole delete.
// Both methods take the caller's transaction — the same shape as
// RecipientSlotPolicy. Implemented by the rentals bounded context; injected
// optionally — when nil, deletion skips both steps (a test wiring).
type RentalDeletionGuard interface {
	HasUnfinished(ctx context.Context, tx transaction.Tx, scope, propertyID uuid.UUID) (bool, error)
	DeleteByProperty(ctx context.Context, tx transaction.Tx, scope, propertyID uuid.UUID) error
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

type SubscriptionLimiter interface {
	ActivePropertyLimit(ctx context.Context, userID uuid.UUID) (int, error)
	WithTx(tx transaction.Tx) (SubscriptionLimiter, error)
}

// PropertySearchQuery is the visible-slice search's parameters (ticket
// #601): the raw search substring — the store escapes the ILIKE
// metacharacters — and the keyset window over (name, id). AfterName and
// AfterID travel together; nil (no cursor) reads from the beginning.
type PropertySearchQuery struct {
	Search    string
	AfterName *string
	AfterID   *uuid.UUID
	Limit     int32
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
	// SearchVisible walks the actor's visible non-archived properties (own
	// plus actively shared) matching the search substring, in the (name, id)
	// keyset order — the search endpoint's window (ticket #601). AccessRole
	// is filled per row: RoleOwner for own rows, the membership role for
	// shared ones.
	SearchVisible(ctx context.Context, actor uuid.UUID, q PropertySearchQuery) ([]domain.Property, error)
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
