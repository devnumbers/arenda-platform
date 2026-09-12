package application

// The search pages' continuation cursors (ticket #597): an opaque base64url
// blob the client echoes back, decoding into the keyset key — the (sort
// key, id) pair the feed's SQL resumes strictly after. The encoding keeps
// the wire opaque (the client never assembles the key); the decode is
// total — anything malformed is ErrInvalidInput, the contract's 400.

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ruleCursorPayload is the payment rules cursor's decoded form: the last
// page's rule creation moment and id — the feed's (created_at, id) order.
type ruleCursorPayload struct {
	CreatedAt time.Time `json:"at"`
	ID        uuid.UUID `json:"id"`
}

// operationCursorPayload is the operations cursor's decoded form: the last
// page's operation date (the domain date, no clock time) and id — the
// feed's (date, id) order.
type operationCursorPayload struct {
	Date string    `json:"date"`
	ID   uuid.UUID `json:"id"`
}

const operationCursorDateFormat = "2006-01-02"

// encodeRuleCursor turns the search page's last rule into the next page's
// cursor.
func encodeRuleCursor(createdAt time.Time, id uuid.UUID) string {
	return encodeCursorPayload(ruleCursorPayload{CreatedAt: createdAt, ID: id})
}

// decodeRuleCursor parses a client-echoed rules cursor; anything malformed
// is ErrInvalidInput.
func decodeRuleCursor(cursor string) (time.Time, uuid.UUID, error) {
	var payload ruleCursorPayload
	if err := decodeCursorPayload(cursor, &payload); err != nil {
		return time.Time{}, uuid.Nil, err
	}
	return payload.CreatedAt, payload.ID, nil
}

// encodeOperationCursor turns the operations page's last row into the next
// page's cursor.
func encodeOperationCursor(date time.Time, id uuid.UUID) string {
	return encodeCursorPayload(operationCursorPayload{
		Date: date.Format(operationCursorDateFormat),
		ID:   id,
	})
}

// decodeOperationCursor parses a client-echoed operations cursor — the
// domain date at UTC midnight; anything malformed is ErrInvalidInput.
func decodeOperationCursor(cursor string) (time.Time, uuid.UUID, error) {
	var payload operationCursorPayload
	if err := decodeCursorPayload(cursor, &payload); err != nil {
		return time.Time{}, uuid.Nil, err
	}
	date, err := time.Parse(operationCursorDateFormat, payload.Date)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("operation cursor date %q: %w", payload.Date, ErrInvalidInput)
	}
	return date, payload.ID, nil
}

// encodeCursorPayload is the shared wire form: JSON in unpadded base64url —
// URL-safe, opaque, and stable across clients.
func encodeCursorPayload(payload any) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		// Time and UUID always marshal, and the payload is fixed-shape.
		panic(fmt.Sprintf("encode cursor payload: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// decodeCursorPayload is the shared parse; every failure mode folds into
// ErrInvalidInput — the client echoed the blob, the server owns its shape.
func decodeCursorPayload(cursor string, payload any) error {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return fmt.Errorf("cursor is not base64url: %w", ErrInvalidInput)
	}
	if err := json.Unmarshal(raw, payload); err != nil {
		return fmt.Errorf("cursor is not a cursor payload: %w", ErrInvalidInput)
	}
	return nil
}
