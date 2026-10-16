// Package postgres adapts the realtime context's audience port to the
// database (карта #714, тикет #716; ADR 0062 §4): the frame recipients are
// the object's derived read access at the moment of publication.
package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	realtimeapp "github.com/nambers/arenda-planform/apps/backend/internal/realtime/application"
)

// AudienceStore resolves a property's frame audience.
type AudienceStore struct {
	queries *postgres.Queries
}

// The store conforms to the consumer-declared port (CODING_STANDARDS).
var _ realtimeapp.FrameAudience = (*AudienceStore)(nil)

// NewAudienceStore creates the audience store over the shared pool.
func NewAudienceStore(db *database.InstrumentedPool) *AudienceStore {
	return &AudienceStore{queries: postgres.New(db)}
}

// Readers returns the users with derived read access to the object at the
// moment of the call (ADR 0028): the owner plus the active members — a
// suspended or revoked membership resolves out, no connection is touched.
func (s *AudienceStore) Readers(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.queries.ListRealtimePropertyReaders(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return nil, err
	}
	readers := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		readers = append(readers, pgconv.UUIDFromPgtype(row))
	}
	return readers, nil
}
