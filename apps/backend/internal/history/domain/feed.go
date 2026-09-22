package domain

import (
	"time"

	"github.com/google/uuid"
)

// FeedEntry is the read-side projection of one journal row (ADR 0061 §7):
// the recorded snapshot as written plus the property's live display name —
// the screen's group header (date → object → actor) is built from it; the
// row's own snapshots are never re-resolved, the header name is not a row
// fact and may follow renames.
type FeedEntry struct {
	ID           uuid.UUID
	PropertyID   uuid.UUID
	PropertyName string
	// ActorID is nil when the acting user was deleted: the row is
	// anonymized, its snapshots below survive. Such rows never match an
	// actor_ids filter.
	ActorID    *uuid.UUID
	ActorName  string
	ActorEmail string
	ActorRole  ActorRole
	Kind       Kind
	Action     Action
	BaseAction BaseAction
	Segments   Segments
	// Context carries the structured extras (amounts in kopecks, dates,
	// old/new values); never rendered as the row text, never searched.
	Context   map[string]any
	CreatedAt time.Time
}

// FilterParticipant is one «Участники» option of the filter sheet (ADR 0061
// §7): a current member or a historical actor of the scope — revoked and
// exited actors stay filterable because their rows survive. The chip shows
// the live display name and email: UI metadata, not a row fact.
type FilterParticipant struct {
	ID    uuid.UUID
	Name  string
	Email string
}

// FilterObject is one «Объекты» option of the filter sheet: the object with
// its card photo avatar — the first (oldest) one, ” when it has none.
type FilterObject struct {
	ID       uuid.UUID
	Name     string
	Address  string
	PhotoURL string
}
