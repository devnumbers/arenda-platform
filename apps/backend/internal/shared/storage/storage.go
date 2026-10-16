// Package storage defines the cross-context object-storage port of the
// private photos (ADR 0065): one image per entity, every byte streamed or
// uploaded through the backend — no method returns or even knows a public
// URL. The port serves three bounded contexts (properties, contacts,
// identity) from one adapter set, the same shared-port shape as the policy
// port (ADR 0028); the concrete adapters live in
// internal/platform/objectstorage.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

// ErrNotFound marks a key that has no object in the storage. The transport
// maps it onto the privacy-preserving 404 — a missing photo is never a 403.
var ErrNotFound = errors.New("object storage: object not found")

// PhotoStorage is the private-photos object storage port (ADR 0065): a
// write-by-key, read-by-stream, delete-by-key contract. The key is the
// caller's (`photos/<uuid>.<ext>`); the storage never builds URLs.
type PhotoStorage interface {
	// Put stores the object under the key, replacing any previous one. The
	// stored content type is the caller's magic-byte-determined value, not
	// a client header.
	Put(ctx context.Context, key string, data io.Reader, contentType string, size int64) error
	// Open returns the object's body, its byte size and the stored content
	// type. A missing key is storage.ErrNotFound.
	Open(ctx context.Context, key string) (body io.ReadCloser, size int64, contentType string, err error)
	// Delete removes the object. Deleting an absent key succeeds — the
	// callers' orphan cleanup is best-effort and must not fail on an
	// already-gone object.
	Delete(ctx context.Context, key string) error
}

// ETagOf returns the strong ETag of an object key (ADR 0065): the serving
// handlers answer 304 from the key hash alone — replacement mints a new
// UUID key, so a new key always means a new ETag and 304 never lies about
// the bytes.
func ETagOf(key string) string {
	sum := sha256.Sum256([]byte(key))
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}
