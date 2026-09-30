package application

// The search pages' continuation cursors (ticket #597): an opaque base64url
// blob the client echoes back, decoding into the keyset key — the (sort
// key, id) pair the feed's SQL resumes strictly after. The encoding keeps
// the wire opaque (the client never assembles the key); the decode is
// total — anything malformed is ErrInvalidInput, the contract's 400. The
// wire form is the shared glue (internal/shared/cursor); only the typed
// payloads live here.

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/cursor"
)

// ruleCursorPayload is the payment rules cursor's decoded form: the last
// page's rule creation moment and id — the feed's (created_at, id) order.
type ruleCursorPayload struct {
	CreatedAt time.Time `json:"at"`
	ID        uuid.UUID `json:"id"`
}

// operationCursorPayload is the operations cursor's decoded form: the last
// page's operation date (the domain date, no clock time), the sort key the
// cursor was issued under (” = the planned-date default — the pre-#992
// cursors) and id — the feed's (sort key, id) order.
type operationCursorPayload struct {
	Date string    `json:"date"`
	ID   uuid.UUID `json:"id"`
	Key  string    `json:"key,omitempty"`
}

const operationCursorDateFormat = "2006-01-02"

// encodeRuleCursor turns the search page's last rule into the next page's
// cursor.
func encodeRuleCursor(createdAt time.Time, id uuid.UUID) string {
	return cursor.Encode(ruleCursorPayload{CreatedAt: createdAt, ID: id})
}

// decodeRuleCursor parses a client-echoed rules cursor; anything malformed
// is ErrInvalidInput.
func decodeRuleCursor(blob string) (time.Time, uuid.UUID, error) {
	var payload ruleCursorPayload
	if err := cursor.Decode(blob, &payload); err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("rule cursor: %w: %w", ErrInvalidInput, err)
	}
	return payload.CreatedAt, payload.ID, nil
}

// encodeOperationCursor turns the operations page's last row into the next
// page's cursor — under the sort the page was read with (ticket #992): the
// key travels so the walk resumes on the same field it sorted by.
func encodeOperationCursor(key OperationSortKey, date time.Time, id uuid.UUID) string {
	return cursor.Encode(operationCursorPayload{
		Date: date.Format(operationCursorDateFormat),
		ID:   id,
		Key:  string(key.Normalized()),
	})
}

// decodedOperationCursor is the operations cursor's decoded form: the sort
// key the cursor was issued under, the domain date at UTC midnight and the
// row id — the (sort key, date, id) key the page resumes strictly after.
type decodedOperationCursor struct {
	Key  OperationSortKey
	Date time.Time
	ID   uuid.UUID
}

// decodeOperationCursor parses a client-echoed operations cursor; anything
// malformed — including an unknown sort key — is ErrInvalidInput: a cursor
// this service never issued must not silently degrade onto the default walk.
func decodeOperationCursor(blob string) (decodedOperationCursor, error) {
	var payload operationCursorPayload
	if err := cursor.Decode(blob, &payload); err != nil {
		return decodedOperationCursor{}, fmt.Errorf("operation cursor: %w: %w", ErrInvalidInput, err)
	}
	date, err := time.Parse(operationCursorDateFormat, payload.Date)
	if err != nil {
		return decodedOperationCursor{},
			fmt.Errorf("operation cursor date %q: %w", payload.Date, ErrInvalidInput)
	}
	switch payload.Key {
	case "", string(SortByDate):
		return decodedOperationCursor{Key: SortByDate, Date: date, ID: payload.ID}, nil
	case string(SortByPaidDate):
		return decodedOperationCursor{Key: SortByPaidDate, Date: date, ID: payload.ID}, nil
	default:
		return decodedOperationCursor{},
			fmt.Errorf("operation cursor key %q: %w", payload.Key, ErrInvalidInput)
	}
}
