package application

// The book listing's continuation cursors (ticket #600): an opaque base64url
// blob the client echoes back, decoding into the keyset key — the (sort
// key, id) tuple the listing's SQL resumes strictly after. The sort key is
// composite: the contact's display name, for the property sort the
// unbound-flag with the bound property's name ahead of it, for the created
// sort the creation moment (ticket #847) — the exact order the store's
// ORDER BY walks. The encoding keeps the wire opaque (the client
// never assembles the key); the decode is total — anything malformed is
// ErrInvalidInput, the contract's 400. The wire form is the shared glue
// (internal/shared/cursor); only the typed payload lives here.

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/cursor"
)

// contactCursorPayload is the cursor's decoded form: the page's last row in
// the listing's own order — the display name, the unbound-flag with the
// bound property's display name (the property sort's leading keys, "" when
// unbound), the creation moment (the created sort's leading key — carried
// on every walk, read only by the created sort's predicate), the card id
// (the tie-off) and the sort/order the
// page was walked with. The sort vocabulary rides in the blob: echoing the
// cursor under a different sort would silently misread every key, so the
// mismatch is the contract's 400.
type contactCursorPayload struct {
	Name         string    `json:"name"`
	PropertyName string    `json:"propertyName,omitempty"`
	Unbound      bool      `json:"unbound,omitempty"`
	CreatedAt    time.Time `json:"at"`
	ID           uuid.UUID `json:"id"`
	Sort         ListSort  `json:"sort"`
	Order        ListOrder `json:"order"`
}

// ContactCursorKey is the decoded keyset key of a book page (ticket #600):
// the tuple the listing's SQL resumes strictly after. PropertyName is the
// bound property's display name ("" when unbound); Name is the contact's
// display sort name; CreatedAt is the card's creation moment — the created
// sort's leading key (carried on every walk; read only by the created
// sort's predicate).
type ContactCursorKey struct {
	Unbound      bool
	PropertyName string
	Name         string
	CreatedAt    time.Time
	ID           uuid.UUID
}

// CursorKey extracts the keyset key of a listed row — the continuation the
// service encodes into the page's nextCursor when the page came back full.
// The sort name replicates the listing SQL's concat_ws over the name fields:
// optional fields store as NULL and the SQL concatenation skips them, so the
// Go join of the non-empty parts is byte-identical.
func (c ListedContact) CursorKey() ContactCursorKey {
	parts := make([]string, 0, 3)
	for _, name := range []string{
		c.Contact.FirstName, c.Contact.LastName, c.Contact.Patronymic,
	} {
		if name != "" {
			parts = append(parts, name)
		}
	}
	return ContactCursorKey{
		Unbound:      c.Contact.PropertyID == nil,
		PropertyName: c.PropertyName,
		Name:         strings.Join(parts, " "),
		CreatedAt:    c.Contact.CreatedAt,
		ID:           c.Contact.ID,
	}
}

// EncodeContactCursor turns the page's last row into the next page's cursor,
// binding the blob to the sort/order the page was walked with.
func EncodeContactCursor(key ContactCursorKey, sort ListSort, order ListOrder) string {
	return cursor.Encode(contactCursorPayload{
		Name:         key.Name,
		PropertyName: key.PropertyName,
		Unbound:      key.Unbound,
		CreatedAt:    key.CreatedAt,
		ID:           key.ID,
		Sort:         sort,
		Order:        order,
	})
}

// DecodeContactCursor parses a client-echoed cursor and returns its keyset
// key with the bound sort/order; anything malformed is ErrInvalidInput.
func DecodeContactCursor(blob string) (ContactCursorKey, ListSort, ListOrder, error) {
	var payload contactCursorPayload
	if err := cursor.Decode(blob, &payload); err != nil {
		return ContactCursorKey{}, "", "", fmt.Errorf("contact cursor: %w: %w", ErrInvalidInput, err)
	}
	return ContactCursorKey{
		Unbound:      payload.Unbound,
		PropertyName: payload.PropertyName,
		Name:         payload.Name,
		CreatedAt:    payload.CreatedAt,
		ID:           payload.ID,
	}, payload.Sort, payload.Order, nil
}
