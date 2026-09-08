// Package application holds the contacts use cases and ports: the CRUD and
// search over the owner's contact book (ADR 0054, ticket #506).
package application

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The application error vocabulary of the contact use cases: the transport
// maps them onto the wire contract (400/403/404).
var (
	// ErrNotFound covers a missing contact, a foreign one and an actor
	// without the view capability — the privacy-preserving 404.
	ErrNotFound = errors.New("contacts: not found")
	// ErrForbidden marks an actor whose role grants the view capability but
	// not the one the use case needs (a viewer on mutations — the ADR 0028
	// matrix).
	ErrForbidden = errors.New("contacts: forbidden")
	// ErrInvalidInput marks a command that violates the create/update
	// contract (empty or overlong name, malformed phone or email). The same
	// sentinel the domain validator returns, re-exported for the transport.
	ErrInvalidInput = domain.ErrInvalidInput
)

// ListScope selects which slice of the book a listing reads: the flat book
// (the actor's own cards plus the property-bound cards of the properties the
// actor can view — the merged visibility of the global book page), only the
// actor's own unbound contacts («без объекта»), or one property's contacts.
// The property scope is the shared-members surface — the service gates the
// actor's view capability on the property; visibility itself is driven by
// the binding, whatever book the card lives in (the book owner never changes
// on a move).
type ListScope string

const (
	ListScopeAll             ListScope = "all"
	ListScopeWithoutProperty ListScope = "without_property"
	ListScopeProperty        ListScope = "property"
)

// ListQuery is the listing filter. Search is a case-insensitive substring
// match over the name fields, phone, email, messenger username and role
// (” = no filter); the store escapes the LIKE metacharacters. Sort/Order
// order the flat listing; the empty values mean the defaults (name/asc).
type ListQuery struct {
	Scope      ListScope
	PropertyID uuid.UUID // ListScopeProperty only.
	Search     string
	Sort       ListSort
	Order      ListOrder
}

// ListSort selects the sort key of a book listing: the contact's display
// name, or the bound property's name (the unbound cards lead both ways —
// the «Общие контакты» group) with contact-name order inside.
type ListSort string

const (
	ListSortName     ListSort = "name"
	ListSortProperty ListSort = "property"
)

// ListOrder is the sort direction of a book listing.
type ListOrder string

const (
	ListOrderAsc  ListOrder = "asc"
	ListOrderDesc ListOrder = "desc"
)

// ListedContact is the list projection of a card: the card itself plus the
// display name of its bound property ("" when unbound) — the wire response
// carries it so the client labels and groups rows without re-reading
// properties.
type ListedContact struct {
	Contact      domain.Contact
	PropertyName string
}

// ContactStore is the persistence port of the contact book. Every method is
// scoped by the data owner where the SQL contract needs it; the by-id read is
// deliberately unscoped — the service loads the card first and authorizes
// from its own property binding, never leaking the miss as a success.
type ContactStore interface {
	// GetByID loads one card by id alone; ErrNotFound when unknown. The
	// service gates the result before it travels anywhere.
	GetByID(ctx context.Context, id uuid.UUID) (domain.Contact, error)
	// List returns the actor's visible contacts per the query's scope,
	// search and sort (ADR 0054): the flat book scope reads the merged
	// visibility — the actor's own cards plus the cards bound to properties
	// the actor can view; the property scope reads the cards bound to that
	// property whatever book they live in; the unbound scope reads the
	// actor's own cards alone.
	List(ctx context.Context, actorID uuid.UUID, q ListQuery) ([]ListedContact, error)
	// Create inserts a new card (the id and owner are app-side) and returns
	// the stored row with its timestamps.
	Create(ctx context.Context, c domain.Contact) (domain.Contact, error)
	// Update writes the editable fields of the card keyed by (id, owner_id)
	// and returns the stored row; a zero-rows update is ErrNotFound — the
	// use case has gated the actor from a pre-transaction read, so the miss
	// means the card is gone.
	Update(ctx context.Context, c domain.Contact) (domain.Contact, error)
	// Delete removes the card keyed by (id, owner_id); zero rows deleted is
	// ErrNotFound for the same reason.
	Delete(ctx context.Context, id, ownerID uuid.UUID) error
	WithTx(tx transaction.Tx) (ContactStore, error)
}

// PropertyRef is the contacts view of the property a use case targets: just
// the data owner (the SQL scope, ADR 0028). The properties context owns the
// entity; contacts never needs its rest.
type PropertyRef struct {
	OwnerID uuid.UUID
}

// PropertyStore resolves the property a contact use case targets: the owner
// whose book a property-bound card lands in. Reads only — the store never
// participates in a contact transaction, so it has no WithTx.
type PropertyStore interface {
	// Get loads the property reference; ErrNotFound when no such property
	// exists (the policy gate has already hidden it from strangers).
	Get(ctx context.Context, propertyID uuid.UUID) (PropertyRef, error)
}
