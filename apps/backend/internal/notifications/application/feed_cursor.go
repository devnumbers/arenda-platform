package application

import (
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/cursor"
)

// The feed page's continuation cursor (#743, канон #597): an opaque base64url
// blob the client echoes back, decoding into the (created_at, id) keyset key
// the SQL resumes strictly after. Same glue the payments search pages use —
// the wire form is the shared package, only the typed payload lives here.
type feedCursorPayload struct {
	CreatedAt time.Time `json:"at"`
	ID        uuid.UUID `json:"id"`
}

// encodeFeedCursor turns a page's last row into the next page's cursor.
func encodeFeedCursor(createdAt time.Time, id uuid.UUID) string {
	return cursor.Encode(feedCursorPayload{CreatedAt: createdAt, ID: id})
}

// decodeFeedCursor parses a client-echoed cursor; anything malformed is
// ErrInvalidInput — the contract's 400.
func decodeFeedCursor(blob string) (time.Time, uuid.UUID, error) {
	var payload feedCursorPayload
	if err := cursor.Decode(blob, &payload); err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidInput
	}
	return payload.CreatedAt, payload.ID, nil
}
