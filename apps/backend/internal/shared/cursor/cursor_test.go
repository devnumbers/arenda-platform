package cursor

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testPayload struct {
	Name string    `json:"name"`
	ID   uuid.UUID `json:"id"`
	At   time.Time `json:"at"`
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	payload := testPayload{
		Name: "Аренда за сентябрь",
		ID:   uuid.MustParse("0198d3f6-9c7a-7d12-8f3a-4c6b1e2d5a90"),
		At:   time.Date(2026, 9, 12, 10, 30, 0, 0, time.UTC),
	}

	blob := Encode(payload)

	// The wire form is opaque base64url without padding — URL-safe by shape.
	assert.NotContains(t, blob, "=")
	assert.NotContains(t, blob, "+")
	assert.NotContains(t, blob, "/")

	var decoded testPayload
	require.NoError(t, Decode(blob, &decoded))
	assert.Equal(t, payload, decoded)
}

func TestDecodeIsTotal(t *testing.T) {
	t.Parallel()

	// Anything malformed folds into ErrInvalidInput — the client echoed the
	// blob, the server owns its shape (the contract's 400).
	notJSON := base64.RawURLEncoding.EncodeToString([]byte("not json at all"))
	cases := []struct {
		name string
		blob string
	}{
		{"empty", ""},
		{"not base64url", "не курсор"},
		{"standard alphabet", "a+b/c=="},
		{"valid base64, not json", notJSON},
		{"json of a wrong shape", "WzEsMl0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var payload testPayload
			err := Decode(tc.blob, &payload)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidInput)
		})
	}
}

func TestDecodeErrorCarriesDetail(t *testing.T) {
	t.Parallel()

	var payload testPayload
	err := Decode("не курсор", &payload)

	require.ErrorIs(t, err, ErrInvalidInput)
	assert.ErrorContains(t, err, "base64url")
}

func TestEncodePanicsOnUnmarshallablePayload(t *testing.T) {
	t.Parallel()

	// Payloads are fixed-shape structs; a payload json.Marshal cannot take
	// is a programming error, not a runtime condition.
	assert.Panics(t, func() {
		Encode(map[string]any{"bad": make(chan int)})
	})
}
