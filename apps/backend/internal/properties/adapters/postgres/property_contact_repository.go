package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PropertyContactRepository persists property contact records.
type PropertyContactRepository struct {
	db postgres.DBTX
}

// NewPropertyContactRepository creates a new property contact repository.
func NewPropertyContactRepository(db postgres.DBTX) *PropertyContactRepository {
	return &PropertyContactRepository{db: db}
}

func (r *PropertyContactRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *PropertyContactRepository) WithTx(tx transaction.Tx) application.PropertyContactRepository {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		panic(fmt.Sprintf("properties.PropertyContactRepository.WithTx: expected postgres.DBTX, got %T", tx))
	}
	return NewPropertyContactRepository(dbtx)
}

// Create inserts a property contact record and returns the created contact.
func (r *PropertyContactRepository) Create(ctx context.Context, contact domain.PropertyContact) (domain.PropertyContact, error) {
	row, err := r.q().CreatePropertyContact(ctx, postgres.CreatePropertyContactParams{
		ID:         pgconv.UUIDToPgtype(contact.ID),
		PropertyID: pgconv.UUIDToPgtype(contact.PropertyID),
		OwnerID:    pgconv.UUIDToPgtype(contact.OwnerID),
		Name:       contact.Name,
		Phone:      contact.Phone,
	})
	if err != nil {
		return domain.PropertyContact{}, err
	}
	return propertyContactFromRow(row), nil
}

// ListByProperty returns all contacts of a property ordered by created_at ASC.
func (r *PropertyContactRepository) ListByProperty(ctx context.Context, propertyID, scope uuid.UUID) ([]domain.PropertyContact, error) {
	rows, err := r.q().ListPropertyContactsByProperty(ctx, postgres.ListPropertyContactsByPropertyParams{
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		OwnerID:    pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		return nil, err
	}
	contacts := make([]domain.PropertyContact, 0, len(rows))
	for _, row := range rows {
		contacts = append(contacts, propertyContactFromRow(row))
	}
	return contacts, nil
}

// GetByIDAndOwner returns a single property contact scoped to the owner.
func (r *PropertyContactRepository) GetByIDAndOwner(ctx context.Context, contactID, scope uuid.UUID) (domain.PropertyContact, error) {
	row, err := r.q().GetPropertyContact(ctx, postgres.GetPropertyContactParams{
		ID:      pgconv.UUIDToPgtype(contactID),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PropertyContact{}, application.ErrNotFound
		}
		return domain.PropertyContact{}, err
	}
	return propertyContactFromRow(row), nil
}

// Update modifies an existing property contact scoped to the owner.
func (r *PropertyContactRepository) Update(
	ctx context.Context, scope uuid.UUID, contact domain.PropertyContact,
) (domain.PropertyContact, error) {
	row, err := r.q().UpdatePropertyContact(ctx, postgres.UpdatePropertyContactParams{
		ID:      pgconv.UUIDToPgtype(contact.ID),
		OwnerID: pgconv.UUIDToPgtype(scope),
		Name:    contact.Name,
		Phone:   contact.Phone,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PropertyContact{}, application.ErrNotFound
		}
		return domain.PropertyContact{}, err
	}
	return propertyContactFromRow(row), nil
}

// Delete removes a property contact scoped to the owner. The existence of the
// contact is established by the service inside the same transaction before
// calling Delete, so a zero-rows result here is not treated as NotFound.
func (r *PropertyContactRepository) Delete(ctx context.Context, contactID, scope uuid.UUID) error {
	return r.q().DeletePropertyContact(ctx, postgres.DeletePropertyContactParams{
		ID:      pgconv.UUIDToPgtype(contactID),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
}

func propertyContactFromRow(row postgres.PropertyContact) domain.PropertyContact {
	return domain.PropertyContact{
		ID:         pgconv.UUIDFromPgtype(row.ID),
		PropertyID: pgconv.UUIDFromPgtype(row.PropertyID),
		OwnerID:    pgconv.UUIDFromPgtype(row.OwnerID),
		Name:       row.Name,
		Phone:      row.Phone,
		CreatedAt:  pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:  pgconv.TimestamptzToTime(row.UpdatedAt),
	}
}
