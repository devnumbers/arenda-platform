package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// DEMOLISHED VERTICAL — kept only so the old HTTP surface compiles until the
// contacts context replaces it (ADR 0051, ticket #507): migration 000118
// dropped the property_contacts table, so every statement below fails at
// runtime. Nothing here may be «fixed» — the whole vertical (this adapter,
// the service, the handlers, the /properties/{id}/contacts endpoints and the
// openapi paths) is deleted by ticket #507. The admin surface already reads
// the new contacts table.

// PropertyContactRepository persists property contact records.
type PropertyContactRepository struct {
	db postgres.DBTX
}

// NewPropertyContactRepository creates a new property contact repository.
func NewPropertyContactRepository(db postgres.DBTX) *PropertyContactRepository {
	return &PropertyContactRepository{db: db}
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
	row := r.db.QueryRow(ctx, `
		INSERT INTO property_contacts (id, property_id, owner_id, name, phone)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, property_id, owner_id, name, phone, created_at, updated_at`,
		contact.ID, contact.PropertyID, contact.OwnerID, contact.Name, contact.Phone,
	)
	return scanPropertyContact(row)
}

// ListByProperty returns all contacts of a property ordered by created_at ASC.
func (r *PropertyContactRepository) ListByProperty(ctx context.Context, propertyID, scope uuid.UUID) ([]domain.PropertyContact, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, property_id, owner_id, name, phone, created_at, updated_at
		FROM property_contacts
		WHERE property_id = $1 AND owner_id = $2
		ORDER BY created_at ASC`,
		propertyID, scope,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contacts := []domain.PropertyContact{}
	for rows.Next() {
		contact, err := scanPropertyContact(rows)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}
	return contacts, rows.Err()
}

// GetByIDAndOwner returns a single property contact scoped to the owner.
func (r *PropertyContactRepository) GetByIDAndOwner(ctx context.Context, contactID, scope uuid.UUID) (domain.PropertyContact, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, property_id, owner_id, name, phone, created_at, updated_at
		FROM property_contacts
		WHERE id = $1 AND owner_id = $2`,
		contactID, scope,
	)
	contact, err := scanPropertyContact(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PropertyContact{}, application.ErrNotFound
		}
		return domain.PropertyContact{}, err
	}
	return contact, nil
}

// Update modifies an existing property contact scoped to the owner.
func (r *PropertyContactRepository) Update(
	ctx context.Context, scope uuid.UUID, contact domain.PropertyContact,
) (domain.PropertyContact, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE property_contacts SET name = $3, phone = $4
		WHERE id = $1 AND owner_id = $2
		RETURNING id, property_id, owner_id, name, phone, created_at, updated_at`,
		contact.ID, scope, contact.Name, contact.Phone,
	)
	updated, err := scanPropertyContact(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PropertyContact{}, application.ErrNotFound
		}
		return domain.PropertyContact{}, err
	}
	return updated, nil
}

// Delete removes a property contact scoped to the owner. The existence of the
// contact is established by the service inside the same transaction before
// calling Delete, so a zero-rows result here is not treated as NotFound.
func (r *PropertyContactRepository) Delete(ctx context.Context, contactID, scope uuid.UUID) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM property_contacts
		WHERE id = $1 AND owner_id = $2`,
		contactID, scope,
	)
	return err
}

// scanPropertyContact scans one full row (the shared column order above).
type rowScanner interface{ Scan(dest ...any) error }

func scanPropertyContact(row rowScanner) (domain.PropertyContact, error) {
	var c domain.PropertyContact
	if err := row.Scan(&c.ID, &c.PropertyID, &c.OwnerID, &c.Name, &c.Phone, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return domain.PropertyContact{}, err
	}
	return c, nil
}
