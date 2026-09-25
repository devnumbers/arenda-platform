// Package realtimetest provides the recording Publisher double for the
// realtime carrier (карта #714, #716; ADR 0062) — the shared replacement for
// per-module fake carriers in the use case suites.
package realtimetest

import (
	"context"

	"github.com/google/uuid"
	realtimeapp "github.com/nambers/arenda-planform/apps/backend/internal/realtime/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
)

// Publication is one recorded EntityChanged call.
type Publication struct {
	Actor   uuid.UUID
	Changes []domain.Change
}

// RecordingPublisher is a realtimeapp.Publisher that captures publications
// instead of delivering frames. The zero value captures silently.
type RecordingPublisher struct {
	// Publications holds the calls in arrival order. Tests reset it by
	// assigning nil between a setup step and the asserted action.
	Publications []Publication

	// OnPublish, when set, fires before the call is recorded — the seam for
	// a post-commit proof: a callback querying the database at dispatch time
	// sees only committed rows.
	OnPublish func(Publication)
}

var _ realtimeapp.Publisher = (*RecordingPublisher)(nil)

// EntityChanged records the call.
func (p *RecordingPublisher) EntityChanged(ctx context.Context, actor uuid.UUID, changes ...domain.Change) {
	publication := Publication{Actor: actor, Changes: changes}
	if p.OnPublish != nil {
		p.OnPublish(publication)
	}
	p.Publications = append(p.Publications, publication)
}

// Pairs flattens every recorded publication into (entity, propertyID) string
// pairs — the assertion vocabulary for a dispatch's shape.
func (p *RecordingPublisher) Pairs() []string {
	pairs := make([]string, 0)
	for _, publication := range p.Publications {
		for _, c := range publication.Changes {
			pair := string(c.Entity) + ":"
			if c.PropertyID != nil {
				pair += c.PropertyID.String()
			}
			pairs = append(pairs, pair)
		}
	}
	return pairs
}
