// Package cursor provides the keyset continuation cursors' shared wire
// form: an opaque base64url blob the client echoes back, decoding into a
// payload struct — the keyset key the listing's SQL resumes strictly
// after. The encoding keeps the wire opaque (the client never assembles
// the key); the decode is total — anything malformed is ErrInvalidInput,
// the contract's 400. Contexts keep the typed payload structs and fold
// this package's sentinel into their own ErrInvalidInput.
package cursor

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrInvalidInput is the malformed-cursor sentinel: the client echoed a
// blob the server cannot read back into its payload.
var ErrInvalidInput = errors.New("invalid cursor")

// Encode is the shared wire form: JSON in unpadded base64url — URL-safe,
// opaque, and stable across clients. Payloads are fixed-shape structs, so
// marshalling cannot fail in operation; a failure is a programming error
// and panics.
func Encode(payload any) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(fmt.Sprintf("encode cursor payload: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// Decode is the shared parse; every failure mode folds into
// ErrInvalidInput — the client echoed the blob, the server owns its shape.
func Decode(blob string, payload any) error {
	raw, err := base64.RawURLEncoding.DecodeString(blob)
	if err != nil {
		return fmt.Errorf("cursor is not base64url: %w", ErrInvalidInput)
	}
	if err := json.Unmarshal(raw, payload); err != nil {
		return fmt.Errorf("cursor is not a cursor payload: %w", ErrInvalidInput)
	}
	return nil
}
