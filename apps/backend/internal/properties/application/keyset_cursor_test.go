package application

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// The search cursor round-trips its keyset key (ticket #597's pattern): the
// page's last (name, id) survives the blob, and the decode is total — any
// malformed input is ErrInvalidInput, the contract's 400.
func TestPropertySearchCursorRoundTrip(t *testing.T) {
	t.Parallel()

	name := "Моя двухкомнатная квартира"
	id := uuid.MustParse("01933e2f-7b1a-7000-8000-000000000001")

	cursor := EncodePropertySearchCursor(name, id)
	gotName, gotID, err := DecodePropertySearchCursor(cursor)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if gotName != name {
		t.Errorf("expected name %q, got %q", name, gotName)
	}
	if gotID != id {
		t.Errorf("expected id %s, got %s", id, gotID)
	}
}

// The wire form is opaque base64url of a minimal JSON payload — the client
// echoes the blob, it never assembles the key.
func TestPropertySearchCursorWireForm(t *testing.T) {
	t.Parallel()

	const wireName = "Квартира"
	cursor := EncodePropertySearchCursor(wireName, uuid.Must(uuid.NewV7()))

	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatalf("cursor is not unpadded base64url: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("cursor payload is not JSON: %v", err)
	}
	if payload["name"] != wireName {
		t.Errorf("expected payload name %v, got %v", wireName, payload["name"])
	}
	if _, ok := payload["id"]; !ok {
		t.Error("expected payload id key")
	}
	if strings.ContainsRune(cursor, '=') {
		t.Error("expected unpadded base64url")
	}
}

func TestPropertySearchCursorRejectsMalformed(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"not base64":         "%%%not-base64%%%",
		"base64 of garbage":  base64.RawURLEncoding.EncodeToString([]byte("garbage")),
		"json of wrong kind": base64.RawURLEncoding.EncodeToString([]byte(`["x",1]`)),
	}
	for name, cursor := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, _, err := DecodePropertySearchCursor(cursor); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}
