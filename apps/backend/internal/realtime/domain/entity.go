// Package domain holds the realtime stream's vocabulary (карта #714,
// тикет #716; ADR 0062): the entity dictionary and the coarse change pair.
// The package knows nothing about the transport (the platform hub serves the
// frames) and nothing about the publishing contexts — they know the dictionary
// through the realtime port.
package domain

import (
	"github.com/google/uuid"
)

// Entity is one entry of the invalidation dictionary (ADR 0062 §2): the
// stable coarse name of a changed entity category. The names are the contract
// between the backend capture points and the frontend query-key families —
// adding an entry is backward compatible, renaming one is not (a client
// without a mapping ignores the frame, a renamed one invalidates nothing).
type Entity string

// The dictionary of eight entities (ADR 0062 §2). The frontend maps each name
// onto the query-key families it invalidates (one category may cover several
// families — operations covers the payment operations and the global ones).
const (
	EntityPayments   Entity = "payments"
	EntityOperations Entity = "operations"
	EntityTasks      Entity = "tasks"
	EntityContacts   Entity = "contacts"
	EntityRentals    Entity = "rentals"
	EntityProperty   Entity = "property"
	EntityAccess     Entity = "access"
	EntityHistory    Entity = "history"
)

// All returns the dictionary in its canonical order. The capture points and
// the tests enumerate it — never a bare literal — so a rename fails to
// compile here first.
func All() []Entity {
	return []Entity{
		EntityPayments,
		EntityOperations,
		EntityTasks,
		EntityContacts,
		EntityRentals,
		EntityProperty,
		EntityAccess,
		EntityHistory,
	}
}

// Change is one coarse invalidation fact: an entity category changed on an
// object. The frame carries no entity data — clients re-read through the API;
// the stream never becomes a second system of record (ADR 0062 §2).
type Change struct {
	Entity Entity
	// PropertyID is the object the change belongs to. Nil is the owner's
	// property-less book (ADR 0052): the frame's payload carries a null
	// propertyId, and its audience is the actor themself — the book has no
	// members to notify, but the actor's other tabs and devices wait for the
	// invalidation like anyone else's.
	PropertyID *uuid.UUID
}

// On builds the change pair for an entity on an object.
func On(entity Entity, propertyID uuid.UUID) Change {
	id := propertyID
	return Change{Entity: entity, PropertyID: &id}
}

// InOwnerBook builds the change pair for an entity of the owner's
// property-less book (ADR 0052) — the payload's propertyId is null.
func InOwnerBook(entity Entity) Change {
	return Change{Entity: entity}
}

// HistoryOn builds the history pair of a property — the piggyback the
// mutation conveyors append when their transaction recorded action journal
// rows (ADR 0061): a written journal row is a history change for the object's
// feed. The journal anchors every row to a property, so the owner book has no
// history pair.
func HistoryOn(propertyID uuid.UUID) Change {
	return On(EntityHistory, propertyID)
}
