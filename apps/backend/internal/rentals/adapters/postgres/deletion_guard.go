package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	pgconv "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance to the properties consumer port.
var _ propertiesapp.RentalDeletionGuard = (*DeletionGuard)(nil)

// DeletionGuard serves the property-delete side of the rentals context
// (issue #632): the unfinished-rental check and the ordered teardown of the
// property's rental rows. It implements the properties application
// RentalDeletionGuard port at the adapter level — the accepted cross-context
// shape (the OccupancyReader precedent): the rentals context owns the
// rentals table and serves the delete over the caller's transaction, and
// the import of the properties application ports creates no cycle.
type DeletionGuard struct{}

// NewDeletionGuard creates the guard. It holds no connection: every call
// binds to the caller's transaction, the same one the property delete runs
// in.
func NewDeletionGuard() *DeletionGuard {
	return &DeletionGuard{}
}

// HasUnfinished reports the property's unfinished rental — the same app
// check the rentals create runs under the property lock, reused through the
// store seam (not copied) so the «незавершённая аренда» rule lives in one
// place. The nil store db is replaced by the caller's transaction in WithTx.
func (DeletionGuard) HasUnfinished(
	ctx context.Context, tx transaction.Tx, scope, propertyID uuid.UUID,
) (bool, error) {
	store, err := NewRentalStore(nil).WithTx(tx)
	if err != nil {
		return false, fmt.Errorf("rentals.DeletionGuard: bind store to tx: %w", err)
	}
	return store.HasUnfinished(ctx, scope, propertyID)
}

// DeleteByProperty removes every rental row of the property inside the
// caller's transaction, before the property row itself: the payment_id
// RESTRICT FK must not race the payments cascade off the property row, so
// the teardown is explicit (ADR 0025 §2).
func (DeletionGuard) DeleteByProperty(
	ctx context.Context, tx transaction.Tx, scope, propertyID uuid.UUID,
) error {
	db, ok := tx.(postgres.DBTX)
	if !ok {
		return fmt.Errorf("rentals.DeletionGuard: %T is not a postgres.DBTX", tx)
	}
	if _, err := postgres.New(db).DeleteRentalsByProperty(ctx, postgres.DeleteRentalsByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	}); err != nil {
		return fmt.Errorf("delete rentals of property %s: %w", propertyID, err)
	}
	return nil
}
