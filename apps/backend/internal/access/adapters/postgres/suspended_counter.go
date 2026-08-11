package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// SuspendedCounter reports how many of a recipient's shared memberships are
// suspended (hidden from the recipient's property list due to a tariff slot
// shortage). It implements the properties application SuspendedSharedCounter
// port without creating a circular import: the access context owns the
// membership table and exposes a read-only adapter. See issue #158 (T4).
type SuspendedCounter struct {
	db postgres.DBTX
}

// NewSuspendedCounter creates a SuspendedCounter adapter.
func NewSuspendedCounter(db postgres.DBTX) *SuspendedCounter {
	return &SuspendedCounter{db: db}
}

// CountSuspendedByUser returns the number of the user's suspended memberships.
func (c *SuspendedCounter) CountSuspendedByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	count, err := postgres.New(c.db).CountSuspendedMembersByUser(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return 0, fmt.Errorf("count suspended memberships: %w", err)
	}
	return int(count), nil
}
