package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
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

// WithTx returns an instance bound to the provided transaction.
func (p *OccupancyProvider) WithTx(tx transaction.Tx) application.OccupancyProvider {
	dbx, ok := tx.(postgres.DBTX)
	if !ok {
		panic(fmt.Sprintf("transaction.Tx does not implement postgres.DBTX: %T", tx))
	}
	return NewOccupancyProvider(dbx)
}

// IsOccupied reports whether the given property currently has an open lease.
func (p *OccupancyProvider) IsOccupied(ctx context.Context, ownerID, propertyID uuid.UUID) (bool, error) {
	count, err := p.q().CountOpenLeasesByProperty(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// OccupiedPropertyIDs returns a set of property IDs that currently have an open lease for the owner.
func (p *OccupancyProvider) OccupiedPropertyIDs(ctx context.Context, ownerID uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := p.q().ListOpenLeasePropertyIDsByOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, err
	}

	occupied := make(map[uuid.UUID]bool, len(rows))
	for _, row := range rows {
		if row.Valid {
			occupied[uuid.UUID(row.Bytes)] = true
		}
	}
	return occupied, nil
}

// Compile-time interface check.
var _ application.OccupancyProvider = (*OccupancyProvider)(nil)
