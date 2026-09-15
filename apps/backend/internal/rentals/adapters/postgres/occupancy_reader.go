package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	pgconv "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	propertiesdomain "github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	rentalsdomain "github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
)

// Compile-time conformance to the properties consumer port.
var _ propertiesapp.RentalOccupancyReader = (*OccupancyReader)(nil)

// OccupancyReader serves the properties list occupancy projection (ticket
// #585, резолюция #584): one batched read of the unfinished rentals of the
// listed properties, each status computed by the pure rentals-domain
// StatusOf against the data owner's today (ADR 0048). It implements the
// properties application RentalOccupancyReader port at the adapter level —
// the accepted cross-context shape (the access SharedProperties precedent):
// the rentals context owns the rentals table and exposes a read-only
// adapter, and the import of the properties application ports creates no
// cycle.
type OccupancyReader struct {
	db       postgres.DBTX
	calendar paymentsapp.OwnerCalendar
}

// NewOccupancyReader creates the reader over the given connection or pool
// and the owner calendar (ADR 0048) the status math resolves today with.
func NewOccupancyReader(db postgres.DBTX, calendar paymentsapp.OwnerCalendar) *OccupancyReader {
	return &OccupancyReader{db: db, calendar: calendar}
}

// OccupancyByProperty reports the occupancy of every listed property: the
// one unfinished rental's status and dates, or OccupancyNone for a property
// without one. Rentals of properties outside the owners map are ignored.
func (r *OccupancyReader) OccupancyByProperty(
	ctx context.Context, owners propertiesapp.PropertyOwners,
) (map[uuid.UUID]propertiesdomain.Occupancy, error) {
	out := make(map[uuid.UUID]propertiesdomain.Occupancy, len(owners))
	if len(owners) == 0 {
		return out, nil
	}
	ids := make([]pgtype.UUID, 0, len(owners))
	for propertyID := range owners {
		ids = append(ids, pgconv.UUIDToPgtype(propertyID))
		out[propertyID] = propertiesdomain.Occupancy{Status: propertiesdomain.OccupancyNone}
	}

	rows, err := postgres.New(r.db).ListUnfinishedRentalsByPropertyIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list unfinished rentals: %w", err)
	}

	// One today per distinct data owner of the found rentals: the status is
	// a date-boundary fact of the owner, not of the asking actor.
	todays := make(map[uuid.UUID]time.Time, len(rows))
	for _, row := range rows {
		ownerID := pgconv.UUIDFromPgtype(row.OwnerID)
		if _, ok := todays[ownerID]; ok {
			continue
		}
		today, err := r.calendar.Today(ctx, ownerID)
		if err != nil {
			return nil, fmt.Errorf("resolve owner today: %w", err)
		}
		todays[ownerID] = today
	}

	for _, row := range rows {
		rental := rentalsdomain.Rental{
			StartDate:      pgconv.DateFromPgtype(row.StartDate),
			PlannedEndDate: pgconv.DatePtrFromPgtype(row.PlannedEndDate),
		}
		propertyID := pgconv.UUIDFromPgtype(row.PropertyID)
		occupancy, err := occupancyOf(
			rentalsdomain.StatusOf(rental, todays[pgconv.UUIDFromPgtype(row.OwnerID)]),
			&rental.StartDate,
			rental.PlannedEndDate,
		)
		if err != nil {
			return nil, fmt.Errorf("occupancy of property %s: %w", propertyID, err)
		}
		out[propertyID] = occupancy
	}
	return out, nil
}

// occupancyOf maps the rentals status onto the properties occupancy
// vocabulary exhaustively: the two enums are deliberately separate (the
// contexts do not share domain types), so the mapping is spelled out and a
// status the query can never return (completed — completed_date IS NULL) or
// an unknown one fails loudly instead of travelling as a silent string cast.
func occupancyOf(
	status rentalsdomain.Status, start, plannedEnd *time.Time,
) (propertiesdomain.Occupancy, error) {
	occ := propertiesdomain.Occupancy{StartDate: start, PlannedEndDate: plannedEnd}
	switch status {
	case rentalsdomain.StatusUpcoming:
		occ.Status = propertiesdomain.OccupancyUpcoming
	case rentalsdomain.StatusActive:
		occ.Status = propertiesdomain.OccupancyActive
	case rentalsdomain.StatusNeedsAttention:
		occ.Status = propertiesdomain.OccupancyNeedsAttention
	default:
		return propertiesdomain.Occupancy{}, fmt.Errorf("unexpected rental status %q", status)
	}
	return occ, nil
}
