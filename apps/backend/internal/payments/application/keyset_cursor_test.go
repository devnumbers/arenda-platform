package application

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The search pages' continuation cursors (ticket #597): an opaque wire blob
// that decodes back into the keyset key — the (sort key, id) pair the feed
// resumes strictly after. Anything malformed is ErrInvalidInput, the
// contract's 400.

func TestDecodeRuleCursorRoundTrip(t *testing.T) {
	t.Parallel()
	createdAt := time.Date(2026, 9, 11, 10, 30, 0, 123456000, time.UTC)
	id := uuid.Must(uuid.NewV7())

	decodedAt, decodedID, err := decodeRuleCursor(encodeRuleCursor(createdAt, id))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !decodedAt.Equal(createdAt) {
		t.Errorf("created_at = %v, want %v", decodedAt, createdAt)
	}
	if decodedID != id {
		t.Errorf("id = %s, want %s", decodedID, id)
	}
}

func TestDecodeOperationCursorRoundTrip(t *testing.T) {
	t.Parallel()
	// The operations cursor carries the sort key and the domain date only —
	// the wire form of the (sort key, date, id) key; the hour components
	// never survive a round trip (ticket #992: the key travels so a cursor
	// echoed under another sort is a 400, not a silently wrong walk).
	date := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	id := uuid.Must(uuid.NewV7())

	decoded, err := decodeOperationCursor(encodeOperationCursor(SortByPaidDate, date, id))
	if err != nil {
		t.Fatalf("decode paid: %v", err)
	}
	if decoded.Key != SortByPaidDate {
		t.Errorf("key = %q, want paid_date", decoded.Key)
	}
	if !decoded.Date.Equal(date) {
		t.Errorf("date = %v, want %v", decoded.Date, date)
	}
	if decoded.ID != id {
		t.Errorf("id = %s, want %s", decoded.ID, id)
	}

	decoded, err = decodeOperationCursor(encodeOperationCursor(SortByDate, date, id))
	if err != nil {
		t.Fatalf("decode default: %v", err)
	}
	if decoded.Key != SortByDate {
		t.Errorf("default key = %q, want date", decoded.Key)
	}

	if _, err = decodeOperationCursor(encodeOperationCursor(SortByDate, date.Add(15*time.Hour), id)); err != nil {
		t.Fatalf("decode same-day instant: %v", err)
	}
}

func TestDecodeCursorsRejectMalformed(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())
	garbage := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	badJSON := base64.RawURLEncoding.EncodeToString([]byte(`{"at":42}`))
	badUUID := base64.RawURLEncoding.EncodeToString([]byte(`{"at":"2026-09-11T10:30:00Z","id":"nope"}`))
	badDate := base64.RawURLEncoding.EncodeToString([]byte(`{"date":"yesterday","id":"` + id.String() + `"}`))
	badKey := base64.RawURLEncoding.EncodeToString([]byte(
		`{"date":"2026-09-11","id":"` + id.String() + `","key":"created"}`))

	cases := []struct {
		name   string
		decode func() error
	}{
		{"empty rules cursor", func() error { _, _, err := decodeRuleCursor(""); return err }},
		{"empty operation cursor", func() error { _, err := decodeOperationCursor(""); return err }},
		{"not base64", func() error { _, _, err := decodeRuleCursor("!!!"); return err }},
		{"garbage plaintext", func() error { _, _, err := decodeRuleCursor(garbage); return err }},
		{"broken json", func() error { _, _, err := decodeRuleCursor(badJSON); return err }},
		{"broken uuid", func() error { _, _, err := decodeRuleCursor(badUUID); return err }},
		{"broken date", func() error { _, err := decodeOperationCursor(badDate); return err }},
		{"unknown sort key", func() error { _, err := decodeOperationCursor(badKey); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.decode(); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("decode error = %v, want ErrInvalidInput", err)
			}
		})
	}
}
