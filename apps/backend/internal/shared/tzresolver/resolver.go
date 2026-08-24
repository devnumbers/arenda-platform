// Package tzresolver defines the timezone-resolution port shared by the platform contexts.
package tzresolver

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// OwnerTimezoneResolver resolves a user's timezone into a *time.Location.
// Implementations should cache results since a user's timezone changes rarely.
type OwnerTimezoneResolver interface {
	Resolve(ctx context.Context, ownerID uuid.UUID) (*time.Location, error)
}
