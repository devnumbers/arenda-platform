package tzresolver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
)

// OwnerTimezone resolves owner timezones from the users table with an in-process cache.
type OwnerTimezone struct {
	db    postgres.DBTX
	cache sync.Map // Keyed by owner UUID; values are *time.Location.
}

// NewOwnerTimezone creates an OwnerTimezone resolver backed by the given DB handle.
func NewOwnerTimezone(db postgres.DBTX) *OwnerTimezone {
	return &OwnerTimezone{db: db}
}

func (r *OwnerTimezone) q() *postgres.Queries {
	return postgres.New(r.db)
}

// Resolve returns the owner's timezone as a *time.Location, caching the result
// so repeated lookups for the same owner avoid a round-trip.
func (r *OwnerTimezone) Resolve(ctx context.Context, ownerID uuid.UUID) (*time.Location, error) {
	if cached, ok := r.cache.Load(ownerID); ok {
		loc, ok := cached.(*time.Location)
		if !ok {
			return nil, fmt.Errorf("cached timezone has unexpected type %T", cached)
		}
		return loc, nil
	}
	tzStr, err := r.q().GetUserTimezone(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, fmt.Errorf("get user timezone: %w", err)
	}
	loc, err := time.LoadLocation(tzStr)
	if err != nil {
		return nil, fmt.Errorf("load location %q: %w", tzStr, err)
	}
	r.cache.Store(ownerID, loc)
	return loc, nil
}

// Invalidate removes the cached timezone for the given owner. Call this after a
// user changes their timezone so subsequent resolves pick up the new value.
func (r *OwnerTimezone) Invalidate(ownerID uuid.UUID) {
	r.cache.Delete(ownerID)
}

// Compile-time check that OwnerTimezone implements OwnerTimezoneResolver.
var _ sharedtz.OwnerTimezoneResolver = (*OwnerTimezone)(nil)
