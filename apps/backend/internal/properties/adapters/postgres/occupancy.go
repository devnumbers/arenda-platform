package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// OccupancyProvider reports which properties have an open lease.
type OccupancyProvider struct {
	db postgres.DBTX
}

// NewOccupancyProvider creates a new occupancy provider.
func NewOccupancyProvider(db postgres.DBTX) *OccupancyProvider {
	return &OccupancyProvider{db: db}
}

func (p *OccupancyProvider) q() *postgres.Queries {
	return postgres.New(p.db)
}

// OccupiedPropertyIDs returns a set of property IDs that currently have an open lease.
func (p *OccupancyProvider) OccupiedPropertyIDs(ctx context.Context, ownerID uuid.UUID, propertyIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := p.q().ListOpenLeasePropertyIDsByOwner(ctx, pgtype.UUID{Bytes: ownerID, Valid: true})
	if err != nil {
		return nil, err
	}

	occupied := make(map[uuid.UUID]bool, len(propertyIDs))
	for _, id := range propertyIDs {
		occupied[id] = false
	}
	for _, row := range rows {
		if row.Valid {
			occupied[uuid.UUID(row.Bytes)] = true
		}
	}
	return occupied, nil
}

// Compile-time interface check.
var _ application.OccupancyProvider = (*OccupancyProvider)(nil)
