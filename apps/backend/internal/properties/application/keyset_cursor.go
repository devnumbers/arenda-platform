package application

// The properties search's continuation cursor (ticket #601): an opaque
// base64url blob the client echoes back, decoding into the keyset key —
// the (sort key, id) pair the search's SQL resumes strictly after. The
// encoding keeps the wire opaque (the client never assembles the key);
// the decode is total — anything malformed is ErrInvalidInput, the
// contract's 400. The pattern is the searches' canon (ticket #597).

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// propertySearchCursorPayload is the cursor's decoded form: the last page's
// property display name and id — the search's (name, id) order. The search
// walks one fixed order, so the blob carries no sort binding (unlike the
// contacts book, whose walk the client chooses).
type propertySearchCursorPayload struct {
	Name string    `json:"name"`
	ID   uuid.UUID `json:"id"`
}

// EncodePropertySearchCursor turns the search page's last property into the
// next page's cursor.
func EncodePropertySearchCursor(name string, id uuid.UUID) string {
	return encodeCursorBlob(propertySearchCursorPayload{Name: name, ID: id})
}

// DecodePropertySearchCursor parses a client-echoed cursor; anything
// malformed is ErrInvalidInput.
func DecodePropertySearchCursor(cursor string) (string, uuid.UUID, error) {
	var payload propertySearchCursorPayload
	if err := decodeCursorBlob(cursor, &payload); err != nil {
		return "", uuid.Nil, err
	}
	return payload.Name, payload.ID, nil
}

// encodeCursorBlob is the shared wire form: JSON in unpadded base64url —
// URL-safe, opaque, and stable across clients.
func encodeCursorBlob(payload propertySearchCursorPayload) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		// String and UUID always marshal, and the payload is fixed-shape.
		panic(fmt.Sprintf("encode property search cursor: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// decodeCursorBlob is the shared parse; every failure mode folds into
// ErrInvalidInput — the client echoed the blob, the server owns its shape.
func decodeCursorBlob(cursor string, payload *propertySearchCursorPayload) error {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return fmt.Errorf("cursor is not base64url: %w", ErrInvalidInput)
	}
	if err := json.Unmarshal(raw, payload); err != nil {
		return fmt.Errorf("cursor is not a cursor payload: %w", ErrInvalidInput)
	}
	return nil
}
