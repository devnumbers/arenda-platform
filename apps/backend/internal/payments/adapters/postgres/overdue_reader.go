package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	pgconv "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// Compile-time conformance to the properties consumer port.
var _ propertiesapp.OverdueOperationsReader = (*OverdueOperationsReader)(nil)

// OverdueOperationsReader serves the payments half of the properties list
// red dot (ticket #585, резолюция #584): which listed properties hold at
// least one overdue planned operation. It implements the properties
// application OverdueOperationsReader port at the adapter level (the
// accepted cross-context shape, the access SharedProperties precedent): the
// payments context owns the operations table and exposes a read-only
// adapter, and the import of the properties application ports creates no
// cycle.
type OverdueOperationsReader struct {
	db       postgres.DBTX
	calendar paymentsapp.OwnerCalendar
}

// NewOverdueOperationsReader creates the reader over the given connection or
// pool and the owner calendar (ADR 0048) the overdue predicate resolves
// today with — the same truth the payments listings report, never a second
// computation of it.
func NewOverdueOperationsReader(db postgres.DBTX, calendar paymentsapp.OwnerCalendar) *OverdueOperationsReader {
	return &OverdueOperationsReader{db: db, calendar: calendar}
}

// OverdueByProperty reports true for every listed property with at least one
// overdue planned operation; the rest miss from the result. The read batches
// per distinct data owner, each batch against that owner's today.
func (r *OverdueOperationsReader) OverdueByProperty(
	ctx context.Context, owners propertiesapp.PropertyOwners,
) (map[uuid.UUID]bool, error) {
	out := make(map[uuid.UUID]bool, len(owners))
	if len(owners) == 0 {
		return out, nil
	}
	byOwner := make(map[uuid.UUID][]pgtype.UUID, len(owners))
	for propertyID, ownerID := range owners {
		byOwner[ownerID] = append(byOwner[ownerID], pgconv.UUIDToPgtype(propertyID))
	}
	for ownerID, ids := range byOwner {
		today, err := r.calendar.Today(ctx, ownerID)
		if err != nil {
			return nil, fmt.Errorf("resolve owner today: %w", err)
		}
		rows, err := postgres.New(r.db).ListPropertyIDsWithOverdueOperations(
			ctx, postgres.ListPropertyIDsWithOverdueOperationsParams{
				Owner:       pgconv.UUIDToPgtype(ownerID),
				Today:       pgconv.DateToPgtype(today),
				PropertyIds: ids,
			})
		if err != nil {
			return nil, fmt.Errorf("list overdue properties of owner %s: %w", ownerID, err)
		}
		for _, row := range rows {
			out[pgconv.UUIDFromPgtype(row)] = true
		}
	}
	return out, nil
}
