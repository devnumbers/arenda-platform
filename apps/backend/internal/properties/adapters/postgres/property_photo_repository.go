package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PropertyPhotoRepository persists property photo metadata.
type PropertyPhotoRepository struct {
	db postgres.DBTX
}

// NewPropertyPhotoRepository creates a new property photo repository.
func NewPropertyPhotoRepository(db postgres.DBTX) *PropertyPhotoRepository {
	return &PropertyPhotoRepository{db: db}
}

func (r *PropertyPhotoRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *PropertyPhotoRepository) WithTx(tx transaction.Tx) application.PropertyPhotoRepository {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		panic(fmt.Sprintf("properties.PropertyPhotoRepository.WithTx: expected postgres.DBTX, got %T", tx))
	}
	return NewPropertyPhotoRepository(dbtx)
}

// Create inserts a photo record and returns the created photo.
func (r *PropertyPhotoRepository) Create(ctx context.Context, photoID, propertyID uuid.UUID, url string) (domain.Photo, error) {
	row, err := r.q().CreatePropertyPhoto(ctx, postgres.CreatePropertyPhotoParams{
		ID:         pgconv.UUIDToPgtype(photoID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		Url:        url,
	})
	if err != nil {
		return domain.Photo{}, err
	}
	return photoFromRow(row), nil
}

// GetByPropertyID returns all photos for a single property.
func (r *PropertyPhotoRepository) GetByPropertyID(ctx context.Context, propertyID uuid.UUID) ([]domain.Photo, error) {
	rows, err := r.q().ListPropertyPhotosByPropertyID(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return nil, err
	}
	return photosFromRows(rows), nil
}

// GetByPropertyIDs returns photos grouped by property ID.
func (r *PropertyPhotoRepository) GetByPropertyIDs(ctx context.Context, propertyIDs []uuid.UUID) (map[uuid.UUID][]domain.Photo, error) {
	if len(propertyIDs) == 0 {
		return map[uuid.UUID][]domain.Photo{}, nil
	}
	ids := make([]pgtype.UUID, 0, len(propertyIDs))
	for _, id := range propertyIDs {
		ids = append(ids, pgconv.UUIDToPgtype(id))
	}
	rows, err := r.q().ListPropertyPhotosByPropertyIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[uuid.UUID][]domain.Photo, len(propertyIDs))
	for _, row := range rows {
		propertyID := pgconv.UUIDFromPgtype(row.PropertyID)
		result[propertyID] = append(result[propertyID], photoFromRow(row))
	}
	return result, nil
}

// CountByPropertyID returns the number of photos for a property.
func (r *PropertyPhotoRepository) CountByPropertyID(ctx context.Context, propertyID uuid.UUID) (int, error) {
	count, err := r.q().CountPropertyPhotosByPropertyID(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetByID returns a single photo by its ID.
func (r *PropertyPhotoRepository) GetByID(ctx context.Context, photoID uuid.UUID) (domain.Photo, error) {
	row, err := r.q().GetPropertyPhotoByID(ctx, pgconv.UUIDToPgtype(photoID))
	if err != nil {
		return domain.Photo{}, err
	}
	return photoFromRow(row), nil
}

// GetByIDAndPropertyID returns a photo only if it belongs to the given property.
func (r *PropertyPhotoRepository) GetByIDAndPropertyID(ctx context.Context, photoID, propertyID uuid.UUID) (domain.Photo, error) {
	row, err := r.q().GetPropertyPhotoByIDAndPropertyID(ctx, postgres.GetPropertyPhotoByIDAndPropertyIDParams{
		ID:         pgconv.UUIDToPgtype(photoID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Photo{}, application.ErrNotFound
		}
		return domain.Photo{}, err
	}
	return photoFromRow(row), nil
}

// Delete removes a photo record by its ID.
func (r *PropertyPhotoRepository) Delete(ctx context.Context, photoID uuid.UUID) error {
	return r.q().DeletePropertyPhoto(ctx, pgconv.UUIDToPgtype(photoID))
}

func photoFromRow(row postgres.PropertyPhoto) domain.Photo {
	return domain.Photo{
		ID:  pgconv.UUIDFromPgtype(row.ID),
		URL: row.Url,
	}
}

func photosFromRows(rows []postgres.PropertyPhoto) []domain.Photo {
	photos := make([]domain.Photo, 0, len(rows))
	for _, row := range rows {
		photos = append(photos, photoFromRow(row))
	}
	return photos
}
