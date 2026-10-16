package application

// The history feed's continuation cursor (тикет #708, канон #597): an opaque
// base64url blob the client echoes back, decoding into the (created_at, id)
// keyset key the SQL resumes strictly before/after. Same shared glue the
// notifications feed and the searches use — only the typed payload lives
// here. The feed walks both ways (мессенджер-лента «новые снизу»), so one
// payload serves both directions: before_cursor continues into the past,
// after_cursor asks for rows newer than the visible ones.

import (
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/cursor"
)

// CursorKey is the decoded keyset key: the (created_at, id) pair of the row
// the next page resumes from.
type CursorKey struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type feedCursorPayload struct {
	CreatedAt time.Time `json:"at"`
	ID        uuid.UUID `json:"id"`
}

// encodeFeedCursor turns a row's (created_at, id) into the continuation
// cursor.
func encodeFeedCursor(key CursorKey) string {
	return cursor.Encode(feedCursorPayload(key))
}

// decodeFeedCursor parses a client-echoed cursor; anything malformed is
// ErrInvalidInput — the contract's 400.
func decodeFeedCursor(blob string) (CursorKey, error) {
	var payload feedCursorPayload
	if err := cursor.Decode(blob, &payload); err != nil {
		return CursorKey{}, ErrInvalidInput
	}
	return CursorKey(payload), nil
}
