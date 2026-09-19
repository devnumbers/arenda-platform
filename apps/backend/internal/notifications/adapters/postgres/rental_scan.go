package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// The adapters satisfy the consumer-declared ports (CODING_STANDARDS).
var (
	_ application.ScanZoneDirectory     = (*RentalScanStore)(nil)
	_ application.RentalCompletedSource = (*RentalScanStore)(nil)
)

// RentalScanStore answers the rental-completed scan's questions (#748) over
// the owning tables directly: the sweep targets' zones, the zone's
// needs_attention rentals and the properties' active members. Read-only —
// the scan publishes through the pipeline, it writes nothing here.
type RentalScanStore struct {
	db postgres.DBTX
}

// NewRentalScanStore creates the scan source adapter over the pool.
func NewRentalScanStore(db postgres.DBTX) *RentalScanStore {
	return &RentalScanStore{db: db}
}

// ListScanZones lists the distinct owner timezones having unfinished rentals
// with a planned end — the sweep targets (ADR 0048 p.3).
func (s *RentalScanStore) ListScanZones(ctx context.Context) ([]application.ScanZone, error) {
	zones, err := postgres.New(s.db).ListRentalCompletedScanZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("list rental scan zones: %w", err)
	}
	result := make([]application.ScanZone, 0, len(zones))
	for _, timezone := range zones {
		result = append(result, application.ScanZone{Timezone: timezone})
	}
	return result, nil
}

// ListCompletedTargets lists the zone's rentals in the needs_attention state
// as of the zone's today: not completed, planned end strictly before today
// (решение #737, тип №1 — the day after the planned end).
func (s *RentalScanStore) ListCompletedTargets(
	ctx context.Context, zone string, today time.Time,
) ([]application.RentalCompletedTarget, error) {
	rows, err := postgres.New(s.db).ListRentalCompletedTargets(ctx, postgres.ListRentalCompletedTargetsParams{
		Timezone: zone,
		Column2:  pgconv.DateToPgtype(today),
	})
	if err != nil {
		return nil, fmt.Errorf("list rental completed targets of zone %s: %w", zone, err)
	}
	targets := make([]application.RentalCompletedTarget, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, application.RentalCompletedTarget{
			RentalID:        pgconv.UUIDFromPgtype(row.RentalID),
			PlannedEndDate:  pgconv.DateFromPgtype(row.PlannedEndDate),
			PropertyID:      pgconv.UUIDFromPgtype(row.PropertyID),
			PropertyName:    row.PropertyName,
			PropertyAddress: row.PropertyAddress,
			OwnerID:         pgconv.UUIDFromPgtype(row.OwnerID),
		})
	}
	return targets, nil
}

// ListActiveRecipients lists the property's active members' user ids — the
// event's recipients besides the owner (решение #737: «Просмотр» включён,
// suspended is not an active participant).
func (s *RentalScanStore) ListActiveRecipients(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := postgres.New(s.db).ListPropertyActiveRecipients(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return nil, fmt.Errorf("list property recipients %s: %w", propertyID, err)
	}
	return pgconv.UUIDSliceFromPgtype(ids), nil
}
