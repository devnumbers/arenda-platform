package application

import "errors"

// ErrPhotoNotFound marks a photo request for a card that has no photo — the
// privacy-preserving 404 of the photo serving endpoints (ADR 0065), beside
// the context's own 404 vocabulary.
var ErrPhotoNotFound = errors.New("contacts: photo not found")
