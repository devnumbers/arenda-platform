package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PropertyRepository persists properties.
type PropertyRepository struct {
	db postgres.DBTX
}

// NewPropertyRepository creates a new property repository.
func NewPropertyRepository(db postgres.DBTX) *PropertyRepository {
	return &PropertyRepository{db: db}
}

func (r *PropertyRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *PropertyRepository) WithTx(tx transaction.Tx) application.PropertyRepository {
	return NewPropertyRepository(tx.(postgres.DBTX))
}

func (r *PropertyRepository) Create(ctx context.Context, ownerID uuid.UUID, property domain.Property) (domain.Property, error) {
	row, err := r.q().CreateProperty(ctx, postgres.CreatePropertyParams{
		OwnerID:     pgtype.UUID{Bytes: ownerID, Valid: true},
		Name:        property.Name,
		Type:        string(property.Type),
		Address:     property.Address,
		Description: pgtype.Text{String: property.Description, Valid: true},
		Status:      string(property.Status),
	})
	if err != nil {
		return domain.Property{}, err
	}
	return propertyFromRow(row), nil
}

func (r *PropertyRepository) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.Property, error) {
	row, err := r.q().GetPropertyByIDAndOwner(ctx, postgres.GetPropertyByIDAndOwnerParams{
		ID:      pgtype.UUID{Bytes: id, Valid: true},
		OwnerID: pgtype.UUID{Bytes: ownerID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Property{}, application.ErrNotFound
		}
		return domain.Property{}, err
	}
	return propertyFromRow(row), nil
}

func (r *PropertyRepository) ListActiveByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Property, error) {
	rows, err := r.q().ListActivePropertiesByOwner(ctx, pgtype.UUID{Bytes: ownerID, Valid: true})
	if err != nil {
		return nil, err
	}
	properties := make([]domain.Property, 0, len(rows))
	for _, row := range rows {
		properties = append(properties, propertyFromRow(row))
	}
	return properties, nil
}

func (r *PropertyRepository) Update(ctx context.Context, ownerID uuid.UUID, property domain.Property) (domain.Property, error) {
	row, err := r.q().UpdateProperty(ctx, postgres.UpdatePropertyParams{
		ID:          pgtype.UUID{Bytes: property.ID, Valid: true},
		OwnerID:     pgtype.UUID{Bytes: ownerID, Valid: true},
		Name:        property.Name,
		Type:        string(property.Type),
		Address:     property.Address,
		Description: pgtype.Text{String: property.Description, Valid: true},
		Status:      string(property.Status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Property{}, application.ErrNotFound
		}
		return domain.Property{}, err
	}
	return propertyFromRow(row), nil
}

func (r *PropertyRepository) Archive(ctx context.Context, id, ownerID uuid.UUID) error {
	_, err := r.q().ArchiveProperty(ctx, postgres.ArchivePropertyParams{
		ID:      pgtype.UUID{Bytes: id, Valid: true},
		OwnerID: pgtype.UUID{Bytes: ownerID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrNotFound
	}
	return err
}

func (r *PropertyRepository) Unarchive(ctx context.Context, id, ownerID uuid.UUID) error {
	_, err := r.q().UnarchiveProperty(ctx, postgres.UnarchivePropertyParams{
		ID:      pgtype.UUID{Bytes: id, Valid: true},
		OwnerID: pgtype.UUID{Bytes: ownerID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrNotFound
	}
	return err
}

func (r *PropertyRepository) CountActiveByOwner(ctx context.Context, ownerID uuid.UUID) (int, error) {
	count, err := r.q().CountActivePropertiesByOwner(ctx, pgtype.UUID{Bytes: ownerID, Valid: true})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func propertyFromRow(row postgres.Property) domain.Property {
	return domain.Property{
		ID:          uuidFromPgtype(row.ID),
		OwnerID:     uuidFromPgtype(row.OwnerID),
		Name:        row.Name,
		Type:        domain.PropertyType(row.Type),
		Address:     row.Address,
		Description: pgtypeTextToString(row.Description),
		Status:      domain.PropertyStatus(row.Status),
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}

func uuidFromPgtype(u pgtype.UUID) uuid.UUID {
	if !u.Valid {
		return uuid.UUID{}
	}
	return uuid.UUID(u.Bytes)
}

func pgtypeTextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}
