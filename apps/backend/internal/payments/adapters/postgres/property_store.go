package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the adapter to the consumer-declared port.
var _ application.PropertyStore = (*PropertyStore)(nil)

// PropertyStore is the postgres adapter of the payments property port: it
// resolves the data owner of a property and takes the mutation's
// serialization lock (ADR 0049 §3). The properties context owns the entity;
// this adapter reads only the columns payments needs, over the payments
// context's own query.
type PropertyStore struct {
	db postgres.DBTX
}

// NewPropertyStore creates a property store over the given connection or pool.
func NewPropertyStore(db postgres.DBTX) *PropertyStore {
	return &PropertyStore{db: db}
}

func (s *PropertyStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *PropertyStore) WithTx(tx transaction.Tx) (application.PropertyStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("payments.PropertyStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewPropertyStore(dbtx), nil
}

// Get loads the property reference without locking.
func (s *PropertyStore) Get(ctx context.Context, propertyID uuid.UUID) (application.PropertyRef, error) {
	row, err := s.q().GetPropertyForPayment(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.PropertyRef{}, application.ErrNotFound
		}
		return application.PropertyRef{}, fmt.Errorf("get property %s: %w", propertyID, err)
	}
	return propertyRefFromRow(row.OwnerID, row.Status), nil
}

// GetForUpdate loads the property reference with the row locked — the
// payments mutation's serialization point (ADR 0049 §3).
func (s *PropertyStore) GetForUpdate(ctx context.Context, propertyID uuid.UUID) (application.PropertyRef, error) {
	row, err := s.q().GetPropertyForPaymentMutation(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.PropertyRef{}, application.ErrNotFound
		}
		return application.PropertyRef{}, fmt.Errorf("lock property %s: %w", propertyID, err)
	}
	return propertyRefFromRow(row.OwnerID, row.Status), nil
}

// propertyRefFromRow maps a property row onto the payments reference. The
// archived status is the financial read-only state (ticket #446); every other
// status serves payments alike.
func propertyRefFromRow(ownerID pgtype.UUID, status string) application.PropertyRef {
	return application.PropertyRef{
		OwnerID:  pgconv.UUIDFromPgtype(ownerID),
		Archived: status == "archived",
	}
}
