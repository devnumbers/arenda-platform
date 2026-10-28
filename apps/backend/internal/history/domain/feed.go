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
	// ActorFirstName/ActorLastName are the actor's live profile parts (карта
	// #1324, протокол D5, ADR 0065 pattern): read through the users JOIN at
	// feed time; empty when the actor has no name/surname or the row is
	// anonymized. The composition «Имя Фамилия» / «(Вы)» / «Пользователь»
	// and the fallback to the ActorName snapshot (by ActorID == nil) are the
	// frontend's D4 helper — the server ships the parts, not the assembled
	// display name.
	ActorFirstName string
	ActorLastName  string
	// ActorPhotoURL is the actor's profile photo streaming path (ADR 0065,
	// решение #1286), live at read; empty when the actor has no photo or the
	// row is anonymized. A revoked actor's path answers 404 and the client
	// falls back to the placeholder.
	ActorPhotoURL string
	// ActorPhotoBlur is the actor's avatar blur frame's ready base64 (ADR
	// 0065 аменд, #1603), live at read; '' when there is no frame.
	ActorPhotoBlur string
	ActorRole      ActorRole
	Kind           Kind
	Action         Action
	BaseAction     BaseAction
	Segments       Segments
	// Details carries the new template's gray detail lines (DetailLines,
	// решение #1317 §2); nil for the rows recorded before the template —
	// those render as the header alone.
	Details DetailLines
	// Context carries the structured extras (amounts in kopecks, dates,
	// old/new values); never rendered as the row text, never searched.
	Context   map[string]any
	CreatedAt time.Time
}

// FilterParticipant is one «Участники» option of the filter sheet (ADR 0061
// §7): a current member or a historical actor of the scope — revoked and
// exited actors stay filterable because their rows survive. The chip shows
// the live display name and email: UI metadata, not a row fact. FirstName
// is the name without the surname (the «(Вы)» row, макет 2067-163528) and
// stays empty when the user has no name — Name is the anonymous label
// «Пользователь» then (карта #1105, аменд #1123);
// IsOwner marks the owner of at least one object of the scope — the
// owner-lock icon. Role is the widest access in the scope ('owner' /
// 'full_access' / 'viewer', the row role icon, макет 2184-94261): the live
// membership outranks the journal snapshot of an exited actor. Status is
// the filter sheet's «Текущие»/«Удалённые» section marker (карта #1324,
// протокол D6, решение #1317 §5): computed at read time — 'current' when
// the user has a live leg in the scope (owner or membership, suspended
// included), 'removed' when only journal rows remain (revoked/exited).
type FilterParticipant struct {
	ID        uuid.UUID
	Name      string
	Email     string
	FirstName string
	// PhotoURL is the profile photo's streaming path (ADR 0065, решение
	// #1286), live at read; '' when the user has no photo.
	PhotoURL string
	// PhotoBlur is the avatar's blur frame's ready base64 (ADR 0065 аменд,
	// #1603), live at read; '' when there is no frame.
	PhotoBlur string
	IsOwner   bool
	Role      string
	Status    string
}

// FilterObject is one «Объекты» option of the filter sheet: the object with
// its card photo avatar — the first (oldest) one, ” when it has none.
type FilterObject struct {
	ID       uuid.UUID
	Name     string
	Address  string
	PhotoURL string
	// PhotoBlur is the card avatar's blur frame's ready base64 (ADR 0065
	// аменд, #1603), live at read; '' when there is no frame.
	PhotoBlur string
	// Type is the property's type — the filter row's avatar placeholder
	// glyph's key (карта #1217, #1244).
	Type string
}
