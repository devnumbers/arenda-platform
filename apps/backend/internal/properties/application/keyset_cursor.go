package application

// The properties search's continuation cursor (ticket #601): an opaque
// base64url blob the client echoes back, decoding into the keyset key —
// the (sort key, id) pair the search's SQL resumes strictly after. The
// encoding keeps the wire opaque (the client never assembles the key);
// the decode is total — anything malformed is ErrInvalidInput, the
// contract's 400. The pattern is the searches' canon (ticket #597); the
// wire form is the shared glue (internal/shared/cursor), only the typed
// payload lives here.

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/cursor"
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
	return cursor.Encode(propertySearchCursorPayload{Name: name, ID: id})
}

// DecodePropertySearchCursor parses a client-echoed cursor; anything
// malformed is ErrInvalidInput.
func DecodePropertySearchCursor(blob string) (string, uuid.UUID, error) {
	var payload propertySearchCursorPayload
	if err := cursor.Decode(blob, &payload); err != nil {
		return "", uuid.Nil, fmt.Errorf("property search cursor: %w: %w", ErrInvalidInput, err)
	}
	return payload.Name, payload.ID, nil
}
