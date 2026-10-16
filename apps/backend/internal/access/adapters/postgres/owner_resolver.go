package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// OwnerResolver resolves the data owner (properties.owner_id) of a property by
// id, directly via sqlc, so the access context does not depend on the
// properties bounded context's repository. A missing property maps to
// ErrMemberNotFound so callers can preserve object privacy (RoleNone → 404).
type OwnerResolver struct {
	db postgres.DBTX
}

// NewOwnerResolver creates an OwnerResolver.
func NewOwnerResolver(db postgres.DBTX) *OwnerResolver {
	return &OwnerResolver{db: db}
}

// GetOwnerID returns the owner id of the property.
func (r *OwnerResolver) GetOwnerID(ctx context.Context, propertyID uuid.UUID) (uuid.UUID, error) {
	row, err := postgres.New(r.db).GetPropertyByID(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.UUID{}, domain.ErrMemberNotFound
		}
		return uuid.UUID{}, err
	}
	return pgconv.UUIDFromPgtype(row.OwnerID), nil
}

// GetTitle returns the display name of the property, used by the invite email
// text (issue #161, T5). A missing property maps to ErrMemberNotFound, mirroring
// GetOwnerID.
func (r *OwnerResolver) GetTitle(ctx context.Context, propertyID uuid.UUID) (string, error) {
	row, err := postgres.New(r.db).GetPropertyByID(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", domain.ErrMemberNotFound
		}
		return "", err
	}
	return row.Name, nil
}

// propertyStatusArchived mirrors the properties module archived status. The
// access module must not depend on the properties module, so the status value
// is duplicated here.
const propertyStatusArchived = "archived"

// IsArchived reports whether the property is in the archived status (issue
// #163). A missing property maps to ErrMemberNotFound, mirroring GetOwnerID.
func (r *OwnerResolver) IsArchived(ctx context.Context, propertyID uuid.UUID) (bool, error) {
	row, err := postgres.New(r.db).GetPropertyByID(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, domain.ErrMemberNotFound
		}
		return false, err
	}
	return row.Status == propertyStatusArchived, nil
}
