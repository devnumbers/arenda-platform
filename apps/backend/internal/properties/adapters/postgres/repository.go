package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

// attributesToJSON marshals domain attributes to JSONB bytes. An empty or nil
// set serializes as '{}' (the column default is NOT NULL DEFAULT '{}').
func attributesToJSON(attrs domain.Attributes) ([]byte, error) {
	if attrs == nil {
		attrs = domain.Attributes{}
	}
	b, err := json.Marshal(map[string]any(attrs))
	if err != nil {
		return nil, fmt.Errorf("marshal attributes: %w", err)
	}
	return b, nil
}

// attributesFromJSON unmarshals JSONB bytes into domain attributes. A null or
// empty byte slice yields an empty (non-nil) attributes set, matching the
// column's NOT NULL DEFAULT '{}' contract.
func attributesFromJSON(raw []byte) (domain.Attributes, error) {
	attrs := domain.Attributes{}
	if len(raw) == 0 {
		return attrs, nil
	}
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return nil, fmt.Errorf("decode attributes: %w", err)
	}
	if attrs == nil {
		attrs = domain.Attributes{}
	}
	return attrs, nil
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *PropertyRepository) WithTx(tx transaction.Tx) application.PropertyRepository {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		panic(fmt.Sprintf("properties.PropertyRepository.WithTx: expected postgres.DBTX, got %T", tx))
	}
	return NewPropertyRepository(dbtx)
}

func (r *PropertyRepository) Create(ctx context.Context, scope uuid.UUID, property domain.Property) (domain.Property, error) {
	attrsJSON, err := attributesToJSON(property.Attributes)
	if err != nil {
		return domain.Property{}, err
	}
	row, err := r.q().CreateProperty(ctx, postgres.CreatePropertyParams{
		ID:          pgconv.UUIDToPgtype(property.ID),
		OwnerID:     pgconv.UUIDToPgtype(scope),
		Name:        property.Name,
		Type:        string(property.Type),
		Address:     property.Address,
		Description: pgtype.Text{String: property.Description, Valid: true},
		Attributes:  attrsJSON,
		Status:      string(property.Status),
	})
	if err != nil {
		return domain.Property{}, err
	}
	return propertyFromRow(row), nil
}

func (r *PropertyRepository) GetByIDAndOwner(ctx context.Context, id, scope uuid.UUID) (domain.Property, error) {
	row, err := r.q().GetPropertyByIDAndOwner(ctx, postgres.GetPropertyByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Property{}, application.ErrNotFound
		}
		return domain.Property{}, err
	}
	return propertyFromCountsRow(postgres.Property{
		ID:          row.ID,
		OwnerID:     row.OwnerID,
		Name:        row.Name,
		Type:        row.Type,
		Address:     row.Address,
		Description: row.Description,
		Attributes:  row.Attributes,
		Status:      row.Status,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, row.OverdueRentCount, row.MembersCount), nil
}

func (r *PropertyRepository) GetByIDAndOwnerForUpdate(ctx context.Context, id, scope uuid.UUID) (domain.Property, error) {
	row, err := r.q().GetPropertyByIDAndOwnerForUpdate(ctx, postgres.GetPropertyByIDAndOwnerForUpdateParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Property{}, application.ErrNotFound
		}
		return domain.Property{}, err
	}
	return propertyFromRow(row), nil
}

// GetByID returns a property by id without owner scoping (T3, issue #156). Used
// by the policy/access layer to resolve the data owner before authorization.
func (r *PropertyRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Property, error) {
	row, err := r.q().GetPropertyByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Property{}, application.ErrNotFound
		}
		return domain.Property{}, err
	}
	return propertyFromCountsRow(postgres.Property{
		ID:          row.ID,
		OwnerID:     row.OwnerID,
		Name:        row.Name,
		Type:        row.Type,
		Address:     row.Address,
		Description: row.Description,
		Attributes:  row.Attributes,
		Status:      row.Status,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, 0, row.MembersCount), nil
}

// GetByIDForUpdate is the pessimistic-lock variant of GetByID (T3, issue #156).
func (r *PropertyRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Property, error) {
	row, err := r.q().GetPropertyByIDForUpdate(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Property{}, application.ErrNotFound
		}
		return domain.Property{}, err
	}
	return propertyFromRow(row), nil
}

func (r *PropertyRepository) ListActiveByOwner(ctx context.Context, scope uuid.UUID) ([]domain.Property, error) {
	rows, err := r.q().ListActivePropertiesByOwner(ctx, pgconv.UUIDToPgtype(scope))
	if err != nil {
		return nil, err
	}
	properties := make([]domain.Property, 0, len(rows))
	for _, row := range rows {
		properties = append(properties, propertyFromCountsRow(postgres.Property{
			ID:          row.ID,
			OwnerID:     row.OwnerID,
			Name:        row.Name,
			Type:        row.Type,
			Address:     row.Address,
			Description: row.Description,
			Attributes:  row.Attributes,
			Status:      row.Status,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		}, row.OverdueRentCount, row.MembersCount))
	}
	return properties, nil
}

func (r *PropertyRepository) ListArchivedByOwner(ctx context.Context, scope uuid.UUID) ([]domain.Property, error) {
	rows, err := r.q().ListArchivedPropertiesByOwner(ctx, pgconv.UUIDToPgtype(scope))
	if err != nil {
		return nil, err
	}
	properties := make([]domain.Property, 0, len(rows))
	for _, row := range rows {
		properties = append(properties, propertyFromCountsRow(postgres.Property{
			ID:          row.ID,
			OwnerID:     row.OwnerID,
			Name:        row.Name,
			Type:        row.Type,
			Address:     row.Address,
			Description: row.Description,
			Attributes:  row.Attributes,
			Status:      row.Status,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		}, row.OverdueRentCount, row.MembersCount))
	}
	return properties, nil
}

func (r *PropertyRepository) Update(ctx context.Context, scope uuid.UUID, property domain.Property) (domain.Property, error) {
	attrsJSON, err := attributesToJSON(property.Attributes)
	if err != nil {
		return domain.Property{}, err
	}
	row, err := r.q().UpdateProperty(ctx, postgres.UpdatePropertyParams{
		ID:          pgconv.UUIDToPgtype(property.ID),
		OwnerID:     pgconv.UUIDToPgtype(scope),
		Name:        property.Name,
		Type:        string(property.Type),
		Address:     property.Address,
		Description: pgtype.Text{String: property.Description, Valid: true},
		Attributes:  attrsJSON,
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

func (r *PropertyRepository) Archive(ctx context.Context, id, scope uuid.UUID) error {
	_, err := r.q().ArchiveProperty(ctx, postgres.ArchivePropertyParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrNotFound
	}
	return err
}

func (r *PropertyRepository) Unarchive(ctx context.Context, id, scope uuid.UUID) error {
	_, err := r.q().UnarchiveProperty(ctx, postgres.UnarchivePropertyParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrNotFound
	}
	return err
}

func (r *PropertyRepository) CountActiveByOwner(ctx context.Context, scope uuid.UUID) (int, error) {
	count, err := r.q().CountActivePropertiesByOwner(ctx, pgconv.UUIDToPgtype(scope))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *PropertyRepository) Delete(ctx context.Context, id, scope uuid.UUID) error {
	return r.q().DeleteProperty(ctx, postgres.DeletePropertyParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	})
}

func (r *PropertyRepository) DeleteOperationsByProperty(ctx context.Context, scope, propertyID uuid.UUID) error {
	return r.q().DeleteOperationsByProperty(ctx, postgres.DeleteOperationsByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
}

func (r *PropertyRepository) DeleteRecurringOperationsByProperty(ctx context.Context, scope, propertyID uuid.UUID) error {
	return r.q().DeleteRecurringOperationsByProperty(ctx, postgres.DeleteRecurringOperationsByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
}

func (r *PropertyRepository) DeleteLeasesByProperty(ctx context.Context, scope, propertyID uuid.UUID) error {
	return r.q().DeleteLeasesByProperty(ctx, postgres.DeleteLeasesByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
}

func propertyFromRow(row postgres.Property) domain.Property {
	attrs, err := attributesFromJSON(row.Attributes)
	if err != nil {
		// Corrupted JSONB is a data-integrity anomaly, not a user error: the
		// column is NOT NULL DEFAULT '{}' and is only ever written through
		// json.Marshal. Degrade gracefully (empty attributes) so the property
		// stays readable; the row id is available to the caller for diagnosis.
		attrs = domain.Attributes{}
	}
	return domain.Property{
		ID:          pgconv.UUIDFromPgtype(row.ID),
		OwnerID:     pgconv.UUIDFromPgtype(row.OwnerID),
		Name:        row.Name,
		Type:        domain.PropertyType(row.Type),
		Address:     row.Address,
		Description: pgconv.TextToString(row.Description),
		Attributes:  attrs,
		Status:      domain.PropertyStatus(row.Status),
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}

// propertyFromCountsRow maps a property row plus its scalar projections
// (overdue rent count, shared-access members count) to the domain model.
func propertyFromCountsRow(row postgres.Property, overdueRentCount, membersCount int64) domain.Property {
	p := propertyFromRow(row)
	p.OverdueRentCount = int(overdueRentCount)
	p.MembersCount = int(membersCount)
	return p
}
