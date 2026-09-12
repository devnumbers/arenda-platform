package application

// The book listing's continuation cursors (ticket #600): an opaque base64url
// blob the client echoes back, decoding into the keyset key — the (sort
// key, id) tuple the listing's SQL resumes strictly after. The sort key is
// composite: the contact's display name, and for the property sort the
// unbound-flag with the bound property's name ahead of it — the exact order
// the store's ORDER BY walks. The encoding keeps the wire opaque (the client
// never assembles the key); the decode is total — anything malformed is
// ErrInvalidInput, the contract's 400.

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// contactCursorPayload is the cursor's decoded form: the page's last row in
// the listing's own order — the display name, the unbound-flag with the
// bound property's display name (the property sort's leading keys, "" when
// unbound) and the card id (the tie-off).
type contactCursorPayload struct {
	Name         string    `json:"name"`
	PropertyName string    `json:"propertyName,omitempty"`
	Unbound      bool      `json:"unbound,omitempty"`
	ID           uuid.UUID `json:"id"`
}

// ContactCursorKey is the decoded keyset key of a book page (ticket #600):
// the tuple the listing's SQL resumes strictly after. PropertyName is the
// bound property's display name ("" when unbound); Name is the contact's
// display sort name.
type ContactCursorKey struct {
	Unbound      bool
	PropertyName string
	Name         string
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
		ID:           c.Contact.ID,
	}
}

// EncodeContactCursor turns the page's last row into the next page's cursor.
func EncodeContactCursor(key ContactCursorKey) string {
	return encodeContactCursorPayload(contactCursorPayload{
		Name:         key.Name,
		PropertyName: key.PropertyName,
		Unbound:      key.Unbound,
		ID:           key.ID,
	})
}

// DecodeContactCursor parses a client-echoed cursor; anything malformed is
// ErrInvalidInput.
func DecodeContactCursor(cursor string) (ContactCursorKey, error) {
	var payload contactCursorPayload
	if err := decodeContactCursorPayload(cursor, &payload); err != nil {
		return ContactCursorKey{}, err
	}
	return ContactCursorKey{
		Unbound:      payload.Unbound,
		PropertyName: payload.PropertyName,
		Name:         payload.Name,
		ID:           payload.ID,
	}, nil
}

// encodeContactCursorPayload is the shared wire form: JSON in unpadded
// base64url — URL-safe, opaque, and stable across clients.
func encodeContactCursorPayload(payload contactCursorPayload) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		// String and UUID always marshal, and the payload is fixed-shape.
		panic(fmt.Sprintf("encode contact cursor: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// decodeContactCursorPayload is the shared parse; every failure mode folds
// into ErrInvalidInput — the client echoed the blob, the server owns its
// shape.
func decodeContactCursorPayload(cursor string, payload *contactCursorPayload) error {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return fmt.Errorf("cursor is not base64url: %w", ErrInvalidInput)
	}
	if err := json.Unmarshal(raw, payload); err != nil {
		return fmt.Errorf("cursor is not a cursor payload: %w", ErrInvalidInput)
	}
	return nil
}
