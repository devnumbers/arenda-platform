package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
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
		OwnerID:     pgconv.UUIDToPgtype(ownerID),
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
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Property{}, application.ErrNotFound
		}
		return domain.Property{}, err
	}
	return propertyFromOverdueRow(postgres.Property{
		ID:          row.ID,
		OwnerID:     row.OwnerID,
		Name:        row.Name,
		Type:        row.Type,
		Address:     row.Address,
		Description: row.Description,
		Status:      row.Status,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, row.OverdueRentCount), nil
}

func (r *PropertyRepository) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.Property, error) {
	row, err := r.q().GetPropertyByIDAndOwnerForUpdate(ctx, postgres.GetPropertyByIDAndOwnerForUpdateParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
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
	rows, err := r.q().ListActivePropertiesByOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, err
	}
	properties := make([]domain.Property, 0, len(rows))
	for _, row := range rows {
		properties = append(properties, propertyFromOverdueRow(postgres.Property{
			ID:          row.ID,
			OwnerID:     row.OwnerID,
			Name:        row.Name,
			Type:        row.Type,
			Address:     row.Address,
			Description: row.Description,
			Status:      row.Status,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		}, row.OverdueRentCount))
	}
	return properties, nil
}

func (r *PropertyRepository) ListArchivedByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Property, error) {
	rows, err := r.q().ListArchivedPropertiesByOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, err
	}
	properties := make([]domain.Property, 0, len(rows))
	for _, row := range rows {
		properties = append(properties, propertyFromOverdueRow(postgres.Property{
			ID:          row.ID,
			OwnerID:     row.OwnerID,
			Name:        row.Name,
			Type:        row.Type,
			Address:     row.Address,
			Description: row.Description,
			Status:      row.Status,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		}, row.OverdueRentCount))
	}
	return properties, nil
}

func (r *PropertyRepository) Update(ctx context.Context, ownerID uuid.UUID, property domain.Property) (domain.Property, error) {
	row, err := r.q().UpdateProperty(ctx, postgres.UpdatePropertyParams{
		ID:          pgconv.UUIDToPgtype(property.ID),
		OwnerID:     pgconv.UUIDToPgtype(ownerID),
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
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrNotFound
	}
	return err
}

func (r *PropertyRepository) Unarchive(ctx context.Context, id, ownerID uuid.UUID) error {
	_, err := r.q().UnarchiveProperty(ctx, postgres.UnarchivePropertyParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrNotFound
	}
	return err
}

func (r *PropertyRepository) CountActiveByOwner(ctx context.Context, ownerID uuid.UUID) (int, error) {
	count, err := r.q().CountActivePropertiesByOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func propertyFromRow(row postgres.Property) domain.Property {
	return domain.Property{
		ID:          pgconv.UUIDFromPgtype(row.ID),
		OwnerID:     pgconv.UUIDFromPgtype(row.OwnerID),
		Name:        row.Name,
		Type:        domain.PropertyType(row.Type),
		Address:     row.Address,
		Description: pgconv.TextToString(row.Description),
		Status:      domain.PropertyStatus(row.Status),
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}

func propertyFromOverdueRow(row postgres.Property, overdueRentCount int64) domain.Property {
	p := propertyFromRow(row)
	p.OverdueRentCount = int(overdueRentCount)
	return p
}
