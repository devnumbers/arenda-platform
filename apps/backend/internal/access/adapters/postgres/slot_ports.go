package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// RecipientLimiterAdapter bridges billing/application.PropertyLimiter to the
// access RecipientLimiter port. It keeps the access application free of any
// billing import: only this adapter layer may depend on both contexts (the
// DDD boundary sits at the application/domain layer).
type RecipientLimiterAdapter struct {
	limiter billingapp.PropertyLimiter
}

// NewRecipientLimiterAdapter creates a RecipientLimiterAdapter over the billing
// property limiter.
func NewRecipientLimiterAdapter(limiter billingapp.PropertyLimiter) *RecipientLimiterAdapter {
	return &RecipientLimiterAdapter{limiter: limiter}
}

// ActivePropertyLimit returns the recipient's active-property tariff limit.
func (a *RecipientLimiterAdapter) ActivePropertyLimit(ctx context.Context, recipientID uuid.UUID) (int, error) {
	return a.limiter.ActivePropertyLimit(ctx, recipientID)
}

// WithTx binds the underlying billing limiter to the transaction (so the
// subscription is read with a row lock, serializing concurrent slot checks)
// and wraps the tx-bound limiter in a fresh adapter.
func (a *RecipientLimiterAdapter) WithTx(tx transaction.Tx) (application.RecipientLimiter, error) {
	txLimiter, err := a.limiter.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind recipient limiter tx: %w", err)
	}
	return &RecipientLimiterAdapter{limiter: txLimiter}, nil
}

// Compile-time check that RecipientLimiterAdapter implements the access port.
var _ application.RecipientLimiter = (*RecipientLimiterAdapter)(nil)

// OccupancyPortAdapter bridges properties/application.OccupancyProvider to the
// access OccupancyPort. Occupancy is a read-only, short-lived signal, so the
// adapter has no WithTx: the coordinator reads it on the main connection,
// consistent with how the properties service reads occupancy outside its
// listing transaction. Reading outside the slot-coordinator tx is acceptable
// because occupancy only gates the eviction/recovery comparator (open lease
// first), never the slot count itself.
type OccupancyPortAdapter struct {
	provider propertiesapp.OccupancyProvider
}

// NewOccupancyPortAdapter creates an OccupancyPortAdapter over the properties
// occupancy provider.
func NewOccupancyPortAdapter(provider propertiesapp.OccupancyProvider) *OccupancyPortAdapter {
	return &OccupancyPortAdapter{provider: provider}
}

// OccupiedPropertyIDs returns the set of properties of the data owner that have
// an open lease.
func (a *OccupancyPortAdapter) OccupiedPropertyIDs(ctx context.Context, ownerID uuid.UUID) (map[uuid.UUID]bool, error) {
	return a.provider.OccupiedPropertyIDs(ctx, ownerID)
}

// Compile-time check that OccupancyPortAdapter implements the access port.
var _ application.OccupancyPort = (*OccupancyPortAdapter)(nil)

// OwnedActivePropertiesAdapter bridges properties/application.PropertyRepository
// to the access OwnedActivePropertiesPort. It exposes the recipient's own
// active+maintenance properties (the own-object half of their tariff pool) as
// id+UpdatedAt pairs.
type OwnedActivePropertiesAdapter struct {
	repo propertiesapp.PropertyRepository
}

// NewOwnedActivePropertiesAdapter creates an OwnedActivePropertiesAdapter over
// the properties repository.
func NewOwnedActivePropertiesAdapter(repo propertiesapp.PropertyRepository) *OwnedActivePropertiesAdapter {
	return &OwnedActivePropertiesAdapter{repo: repo}
}

// ListActiveWithMeta returns the ids and UpdatedAt of the owner's active and
// maintenance properties (the statuses that occupy a tariff slot), mapped from
// the properties repository projection.
func (a *OwnedActivePropertiesAdapter) ListActiveWithMeta(ctx context.Context, ownerID uuid.UUID) ([]application.OwnedPropertyMeta, error) {
	props, err := a.repo.ListActiveByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list active properties by owner: %w", err)
	}
	out := make([]application.OwnedPropertyMeta, 0, len(props))
	for _, p := range props {
		out = append(out, application.OwnedPropertyMeta{
			ID:        p.ID,
			UpdatedAt: p.UpdatedAt,
		})
	}
	return out, nil
}

// Compile-time check that OwnedActivePropertiesAdapter implements the access port.
var _ application.OwnedActivePropertiesPort = (*OwnedActivePropertiesAdapter)(nil)
