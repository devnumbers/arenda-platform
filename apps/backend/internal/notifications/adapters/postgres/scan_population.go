package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// scanPopulationStore answers the scan question shared verbatim by all three
// scan sources: who besides the owner receives the events of a property.
// The sources embed it, so the consumer-declared ports are satisfied by
// promotion; each source keeps its own zone query.
type scanPopulationStore struct {
	db postgres.DBTX
}

// ListActiveRecipients lists the property's active members' user ids — the
// event's recipients besides the owner (решение #737: «Просмотр» включён,
// suspended is not an active participant).
func (s scanPopulationStore) ListActiveRecipients(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := postgres.New(s.db).ListPropertyActiveRecipients(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return nil, fmt.Errorf("list property recipients %s: %w", propertyID, err)
	}
	return pgconv.UUIDSliceFromPgtype(ids), nil
}

// scanZones maps a scan's own zone query onto the sweep targets (ADR 0048
// p.3); what names the scan in the error message.
func scanZones(
	ctx context.Context, what string, query func(context.Context) ([]string, error),
) ([]application.ScanZone, error) {
	zones, err := query(ctx)
	if err != nil {
		return nil, fmt.Errorf("list %s scan zones: %w", what, err)
	}
	result := make([]application.ScanZone, 0, len(zones))
	for _, timezone := range zones {
		result = append(result, application.ScanZone{Timezone: timezone})
	}
	return result, nil
}
