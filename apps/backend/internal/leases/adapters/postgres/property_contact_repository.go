package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// PropertyContactRepository reads property contacts for the export use case.
type PropertyContactRepository struct {
	db postgres.DBTX
}

// NewPropertyContactRepository creates a new property contact repository for
// the lease export context.
func NewPropertyContactRepository(db postgres.DBTX) *PropertyContactRepository {
	return &PropertyContactRepository{db: db}
}

func (r *PropertyContactRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// ListForExport returns the property contacts ordered by created_at ASC. The
// query is scoped by owner_id, so a foreign property yields no rows.
func (r *PropertyContactRepository) ListForExport(ctx context.Context, propertyID, ownerID uuid.UUID) ([]application.ExportContactRow, error) {
	rows, err := r.q().ListPropertyContactsByProperty(ctx, postgres.ListPropertyContactsByPropertyParams{
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		return nil, fmt.Errorf("list property contacts: %w", err)
	}
	contacts := make([]application.ExportContactRow, 0, len(rows))
	for _, row := range rows {
		contacts = append(contacts, application.ExportContactRow{
			Name:  row.Name,
			Phone: row.Phone,
		})
	}
	return contacts, nil
}
