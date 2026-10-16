package domain

import "errors"

// ErrInvalidNotification rejects a feed row that violates the catalog or the
// snapshot invariants: unknown feed event type, zero ids, empty title/body
// or dedup key.
var ErrInvalidNotification = errors.New("invalid notification")
