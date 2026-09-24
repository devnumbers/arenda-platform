package postgres

// RentalLinkReader is the read-only rentals adapter behind the payments
// mutation gate (ADR 0053, ticket #818): which payment rules a rental row
// references — the rent payment is created, edited and deleted only through
// the rental. It implements the payments application RentalManagedReader
// port at the adapter level — the accepted cross-context shape (the
// OccupancyReader precedent): the rentals context owns the rentals table and
// exposes a read-only adapter, and the import of the payments application
// ports creates no cycle.

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	pgconv "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance to the payments consumer port.
var _ paymentsapp.RentalManagedReader = (*RentalLinkReader)(nil)

// RentalLinkReader is the postgres adapter of the payments
// RentalManagedReader port.
type RentalLinkReader struct {
	db postgres.DBTX
}

// NewRentalLinkReader creates the reader over the given connection or pool.
func NewRentalLinkReader(db postgres.DBTX) *RentalLinkReader {
	return &RentalLinkReader{db: db}
}

// WithTx returns a reader bound to the provided transaction — the gate runs
// inside the conveyor's transaction, under the property lock.
func (r *RentalLinkReader) WithTx(tx transaction.Tx) (paymentsapp.RentalManagedReader, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("rentals.RentalLinkReader.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewRentalLinkReader(dbtx), nil
}

// ManagedPaymentIDs returns the subset of the given rules a rental row
// references — any rental state (a completed rental is final, and its
// payment is final with it). An empty input never reaches the query.
func (r *RentalLinkReader) ManagedPaymentIDs(
	ctx context.Context, scope uuid.UUID, paymentIDs []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	managed := make(map[uuid.UUID]bool, len(paymentIDs))
	if len(paymentIDs) == 0 {
		return managed, nil
	}
	rows, err := postgres.New(r.db).ListRentalManagedPaymentIDs(ctx, postgres.ListRentalManagedPaymentIDsParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PaymentIds: pgconv.UUIDSliceToPgtype(paymentIDs),
	})
	if err != nil {
		return nil, fmt.Errorf("list rental-managed payment ids: %w", err)
	}
	for _, id := range rows {
		managed[pgconv.UUIDFromPgtype(id)] = true
	}
	return managed, nil
}
