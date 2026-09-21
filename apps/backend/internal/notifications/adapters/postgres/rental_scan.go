package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// The adapters satisfy the consumer-declared ports (CODING_STANDARDS).
var (
	_ application.ScanZoneDirectory     = (*RentalScanStore)(nil)
	_ application.RentalCompletedSource = (*RentalScanStore)(nil)
)

// RentalScanStore answers the rental-completed scan's questions (#748,
// #777) over the owning tables directly: the sweep targets' zones, the
// zone's needs_attention rentals, the upcoming boundaries' booking list, the
// boundary job's reload and the properties' active members. Read-only — the
// scan publishes through the pipeline, it writes nothing here.
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

// ListScheduledCompletedTargets lists the unfinished rentals whose boundary
// — 00:00 of the day after the planned end in the owner's timezone — falls
// in the window (from, until]; the completed boundary's booking list (issue
// #777).
func (s *RentalScanStore) ListScheduledCompletedTargets(
	ctx context.Context, from, until time.Time,
) ([]application.RentalScheduleTarget, error) {
	rows, err := postgres.New(s.db).ListRentalScheduledCompletedTargets(ctx, postgres.ListRentalScheduledCompletedTargetsParams{
		Column1: pgconv.TimePtrToPgtype(&from),
		Column2: pgconv.TimePtrToPgtype(&until),
	})
	if err != nil {
		return nil, fmt.Errorf("list scheduled completed rentals: %w", err)
	}
	targets := make([]application.RentalScheduleTarget, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, application.RentalScheduleTarget{
			RentalID:       pgconv.UUIDFromPgtype(row.RentalID),
			PlannedEndDate: pgconv.DateFromPgtype(row.PlannedEndDate),
			FireAt:         pgconv.TimestamptzToTime(row.FireAt),
		})
	}
	return targets, nil
}

// GetScheduledCompletedRental reloads one rental at its completed boundary
// — the boundary job's delivery-time resolution (issue #777). A completed
// rental, an extended one (the planned end moved off the booked date), an
// archived property and a job awake before the boundary are pgx.ErrNoRows
// here and answer live=false: the job finishes without publishing.
func (s *RentalScanStore) GetScheduledCompletedRental(
	ctx context.Context, rentalID uuid.UUID, plannedEnd, now time.Time,
) (application.RentalCompletedTarget, bool, error) {
	row, err := postgres.New(s.db).GetScheduledCompletedRental(ctx, postgres.GetScheduledCompletedRentalParams{
		Column1: pgconv.UUIDToPgtype(rentalID),
		Column2: pgconv.DateToPgtype(plannedEnd),
		Column3: pgconv.TimePtrToPgtype(&now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.RentalCompletedTarget{}, false, nil
		}
		return application.RentalCompletedTarget{}, false, fmt.Errorf("load scheduled completed rental %s: %w", rentalID, err)
	}
	return application.RentalCompletedTarget{
		RentalID:        pgconv.UUIDFromPgtype(row.RentalID),
		PlannedEndDate:  pgconv.DateFromPgtype(row.PlannedEndDate),
		PropertyID:      pgconv.UUIDFromPgtype(row.PropertyID),
		PropertyName:    row.PropertyName,
		PropertyAddress: row.PropertyAddress,
		OwnerID:         pgconv.UUIDFromPgtype(row.OwnerID),
	}, true, nil
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
