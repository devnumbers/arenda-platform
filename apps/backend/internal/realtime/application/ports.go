// Package application provides the realtime stream's publication port used by
// the entity-owning contexts and the audience port its hub-facing adapter
// resolves recipients through (карта #714, тикет #716; ADR 0062).
package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
)

// Publisher is the port the mutation pipelines publish entity-changed frames
// through (ADR 0062 §3). The publication discipline is the caller's conveyor
// canon (the grace-events and history-journal discipline): the transaction
// captures the distinct (entity, object) pairs it changed, and EntityChanged
// runs strictly after the commit — a rolled-back transaction publishes
// nothing. Best-effort: a failed frame is logged and never fails or rolls
// back the committed mutation.
type Publisher interface {
	// EntityChanged publishes one coarse entity.changed frame per distinct
	// change (the per-transaction dedup is the adapter's: a bulk operation
	// whose callers hand over repeated pairs emits one frame per pair, not N).
	// The actor is the mutation's author: for the owner-book changes (a nil
	// PropertyID) they are the whole audience — the book has no members.
	EntityChanged(ctx context.Context, actor uuid.UUID, changes ...domain.Change)
}

// FrameAudience resolves a frame's recipients: the users with derived read
// access to the object at the moment of publication (ADR 0028, ADR 0062 §4).
// A member whose access was revoked or suspended resolves out — no connection
// management anywhere, the frames simply stop arriving.
type FrameAudience interface {
	Readers(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error)
}
