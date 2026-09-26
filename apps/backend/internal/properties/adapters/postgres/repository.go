// Package postgres holds the properties persistence adapters: property, photo and contact repositories with
// occupancy queries.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
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
		PinnedAt:    row.PinnedAt,
	}, row.MembersCount), nil
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
		PinnedAt:    row.PinnedAt,
	}, row.MembersCount), nil
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
			PinnedAt:    row.PinnedAt,
		}, row.MembersCount))
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
			PinnedAt:    row.PinnedAt,
		}, row.MembersCount))
	}
	return properties, nil
}

// SearchVisible walks the actor's visible non-archived properties matching
// the search — the search endpoint's keyset window (ticket #601). The store
// escapes the ILIKE metacharacters (ESCAPE '\') and carries the raw keyset
// key: each row's own (name, id) resumes the walk strictly after itself.
// access_role arrives resolved per row ('owner' or the membership role).
func (r *PropertyRepository) SearchVisible(
	ctx context.Context, actor uuid.UUID, q application.PropertySearchQuery,
) ([]domain.Property, error) {
	var afterName pgtype.Text
	var afterID pgtype.UUID
	if q.AfterName != nil {
		afterName = pgtype.Text{String: *q.AfterName, Valid: true}
	}
	if q.AfterID != nil {
		afterID = pgconv.UUIDToPgtype(*q.AfterID)
	}
	rows, err := r.q().SearchVisibleProperties(ctx, postgres.SearchVisiblePropertiesParams{
		Actor:     pgconv.UUIDToPgtype(actor),
		Search:    pgconv.EscapeLikePattern(q.Search),
		AfterName: afterName,
		AfterID:   afterID,
		PageLimit: q.Limit,
	})
	if err != nil {
		return nil, err
	}
	properties := make([]domain.Property, 0, len(rows))
	for _, row := range rows {
		p := propertyFromCountsRow(postgres.Property{
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
			PinnedAt:    row.PinnedAt,
		}, row.MembersCount)
		p.AccessRole = sharedpolicy.Role(row.AccessRole)
		properties = append(properties, p)
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

// SetPin writes the global pin in one atomic UPDATE (ticket #577); the value
// — a moment or NULL — arrives resolved from the application layer, which has
// proven existence and the edit capability under the row lock.
func (r *PropertyRepository) SetPin(ctx context.Context, id, scope uuid.UUID, pinnedAt *time.Time) (domain.Property, error) {
	row, err := r.q().SetPropertyPin(ctx, postgres.SetPropertyPinParams{
		PinnedAt: pgconv.TimePtrToPgtype(pinnedAt),
		ID:       pgconv.UUIDToPgtype(id),
		OwnerID:  pgconv.UUIDToPgtype(scope),
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
		PinnedAt:    pgconv.TimestamptzToPtrTime(row.PinnedAt),
	}
}

// propertyFromCountsRow maps a property row plus its shared-access members
// count projection to the domain model.
func propertyFromCountsRow(row postgres.Property, membersCount int64) domain.Property {
	p := propertyFromRow(row)
	p.MembersCount = int(membersCount)
	return p
}
