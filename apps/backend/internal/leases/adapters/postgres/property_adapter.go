package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PropertyOperationArchiver adapts the leases operation repository to the
// properties application OperationArchiver port.
type PropertyOperationArchiver struct {
	repo *OperationRepository
}

// NewPropertyOperationArchiver creates a new property-scoped operation archiver.
func NewPropertyOperationArchiver(repo *OperationRepository) *PropertyOperationArchiver {
	return &PropertyOperationArchiver{repo: repo}
}

// WithTx returns an instance bound to the provided transaction.
func (r *PropertyOperationArchiver) WithTx(tx transaction.Tx) propertiesapp.OperationArchiver {
	return NewPropertyOperationArchiver(r.repo.WithTx(tx).(*OperationRepository))
}

// DeleteFutureUneditedOperationsByProperty removes future unedited operations
// for the given property.
func (r *PropertyOperationArchiver) DeleteFutureUneditedOperationsByProperty(ctx context.Context, propertyID uuid.UUID, fromDate time.Time) error {
	return r.repo.DeleteFutureUneditedOperationsByProperty(ctx, propertyID, fromDate)
}

// PropertyRecurringOperationStatusUpdater adapts the leases recurring operation
// repository to the properties application RecurringOperationStatusUpdater port.
type PropertyRecurringOperationStatusUpdater struct {
	repo *RecurringOperationRepository
}

// NewPropertyRecurringOperationStatusUpdater creates a new property-scoped
// recurring operation status updater.
func NewPropertyRecurringOperationStatusUpdater(repo *RecurringOperationRepository) *PropertyRecurringOperationStatusUpdater {
	return &PropertyRecurringOperationStatusUpdater{repo: repo}
}

// WithTx returns an instance bound to the provided transaction.
func (r *PropertyRecurringOperationStatusUpdater) WithTx(tx transaction.Tx) propertiesapp.RecurringOperationStatusUpdater {
	return NewPropertyRecurringOperationStatusUpdater(r.repo.WithTx(tx).(*RecurringOperationRepository))
}

// ListByProperty returns recurring operations for a property.
func (r *PropertyRecurringOperationStatusUpdater) ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]propertiesapp.RecurringOperation, error) {
	recs, err := r.repo.ListByProperty(ctx, ownerID, propertyID)
	if err != nil {
		return nil, err
	}

	result := make([]propertiesapp.RecurringOperation, len(recs))
	for i, rec := range recs {
		result[i] = propertiesapp.RecurringOperation{
			ID:            rec.ID,
			OwnerID:       rec.OwnerID,
			PropertyID:    rec.PropertyID,
			LeaseID:       rec.LeaseID,
			Type:          string(rec.Type),
			Category:      string(rec.Category),
			AmountKopecks: rec.AmountKopecks,
			StartDate:     rec.StartDate,
			PaymentDay:    rec.PaymentDay,
			EndDate:       rec.EndDate,
			Status:        string(rec.Status),
			Comment:       rec.Comment,
		}
	}
	return result, nil
}

// UpdateStatus updates the status of a recurring operation by ID.
func (r *PropertyRecurringOperationStatusUpdater) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.repo.UpdateStatusByID(ctx, id, status)
	return err
}
