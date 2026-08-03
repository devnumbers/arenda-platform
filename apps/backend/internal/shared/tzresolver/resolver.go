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

// ReminderRescheduler recalculates pending reminder send times when an owner's
// timezone changes. Wall-clock semantics: the same local date and time-of-day
// are preserved in the new timezone.
type ReminderRescheduler interface {
	RescheduleForTimezoneChange(ctx context.Context, ownerID uuid.UUID, oldTZ, newTZ string) error
}
