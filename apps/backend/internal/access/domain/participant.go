package domain

import (
	"errors"

	"github.com/google/uuid"
)

// ParticipantEntryStatus is the per-object access status inside the owner's
// participant aggregate (issue #693): the lifecycle of the person's access to
// one property. Active/suspended mirror the membership status; pending marks
// a pending email invitation — not access yet.
type ParticipantEntryStatus string

const (
	// ParticipantEntryActive is a membership granting access to the object.
	ParticipantEntryActive ParticipantEntryStatus = "active"
	// ParticipantEntrySuspended is a membership hidden from the recipient by a
	// tariff slot shortage (issue #158, T4).
	ParticipantEntrySuspended ParticipantEntryStatus = "suspended"
	// ParticipantEntryPending is a pending email invitation waiting for the
	// invitee to register (issue #161, T5).
	ParticipantEntryPending ParticipantEntryStatus = "pending"
)

// String returns the string representation of the entry status.
func (s ParticipantEntryStatus) String() string { return string(s) }

// ParticipantAggregateStatus is the aggregate badge of an owner's participant
// (issue #693): «доступ ко всем объектам» (the person holds active access to
// every active property in the reading actor's scope), «доступно N объектов»
// (partial), «превышен лимит объектов» (at least one suspended membership).
// The roles shown to owners remain display-only («Редактирование»/«Просмотр»),
// the domain stays full_access/viewer — chart decision of map #692.
type ParticipantAggregateStatus string

const (
	// ParticipantStatusAllProperties — active access to every scoped property.
	ParticipantStatusAllProperties ParticipantAggregateStatus = "all_properties"
	// ParticipantStatusPartial — active access to some of the scoped
	// properties; the count travels alongside the status.
	ParticipantStatusPartial ParticipantAggregateStatus = "partial"
	// ParticipantStatusLimitExceeded — at least one suspended membership.
	ParticipantStatusLimitExceeded ParticipantAggregateStatus = "limit_exceeded"
)

// String returns the string representation of the aggregate status.
func (s ParticipantAggregateStatus) String() string { return string(s) }

// ErrParticipantNotFound is returned when the requested owner's participant is
// not in the reading actor's scope. One error covers every reason — an unknown
// identifier, a person whose access was fully revoked (the deep-link 404
// policy of the participant page, issue #698) and an actor without the manage
// scope — so the resource's existence stays private.
var ErrParticipantNotFound = errors.New("participant not found")

// ParticipantEntry is one property leg of a person's access in the owner's
// participant aggregate: the role and the lifecycle status on that property.
// Entries are unique per property — the aggregation layer collapses duplicate
// (property, user) membership rows (active wins over suspended) before they
// reach the aggregate.
type ParticipantEntry struct {
	PropertyID uuid.UUID
	Role       Role
	Status     ParticipantEntryStatus
}

// ComputeParticipantAggregate reduces a person's entries against the number of
// properties in the reading actor's scope into the aggregate badge and the
// accessible-properties count. The rule (issue #693):
//
//  1. Any suspended entry → limit_exceeded (the recipient's tariff limit was
//     hit at least once); the count still reports the active entries.
//  2. Otherwise, at least one active entry and actives cover the whole scope →
//     all_properties.
//  3. Otherwise → partial with the number of active entries. Pending entries
//     are not access: they count toward nothing.
//
// A scope of zero properties can never read as all_properties — an aggregate
// with access to «every» zero objects is meaningless, it stays partial.
func ComputeParticipantAggregate(
	entries []ParticipantEntry, scopePropertyCount int,
) (status ParticipantAggregateStatus, accessibleCount int) {
	active, suspended := 0, 0
	for _, e := range entries {
		switch e.Status {
		case ParticipantEntryActive:
			active++
		case ParticipantEntrySuspended:
			suspended++
		case ParticipantEntryPending:
			// Not access: counts toward neither the badge nor the number.
		}
	}
	switch {
	case suspended > 0:
		return ParticipantStatusLimitExceeded, active
	case active > 0 && active == scopePropertyCount:
		return ParticipantStatusAllProperties, active
	default:
		return ParticipantStatusPartial, active
	}
}
