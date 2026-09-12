package application

import (
	"errors"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrLimitExceeded     = errors.New("active property limit exceeded")
	ErrInvalidTransition = errors.New("invalid property status transition")
	ErrInvalidInput      = errors.New("invalid property input")
	// ErrForbidden is returned when an actor can view a property (so its
	// existence is not secret) but lacks the capability for the requested
	// operation — e.g. a viewer attempting an edit, or a member attempting a
	// lifecycle change (T3, issue #156).
	ErrForbidden = errors.New("forbidden")
	// ErrAccessSuspended is returned when the actor's membership on the
	// property is suspended because their tariff's active-property limit is
	// exceeded (issue #158, T4). It is the single exception to the privacy
	// 404-policy: the suspended recipient gets a distinguishable signal so the
	// UI can show an honest "limit exceeded" screen (T9); every other access
	// failure stays an indistinguishable ErrNotFound.
	ErrAccessSuspended = errors.New("membership access suspended")

	ErrAddressSuggestFailed = errors.New("address suggestion request failed")

	ErrArchivedProperty  = errors.New("cannot modify an archived property")
	ErrAlreadyArchived   = errors.New("property is already archived")
	ErrNotArchived       = errors.New("property is not archived")
	ErrPhotoLimitReached = errors.New("property photo limit reached")
	// ErrPropertyOccupied is returned when the owner deletes a property that
	// still has an unfinished rental (issue #632, wire code
	// property_occupied): the rental must be completed first — the path the
	// rental completion flow (#627) already serves.
	ErrPropertyOccupied = errors.New("property has an unfinished rental")
)

// InvalidStatusTransitionError describes a status change that is not allowed.
type InvalidStatusTransitionError struct {
	From domain.PropertyStatus
	To   domain.PropertyStatus
}

func (e *InvalidStatusTransitionError) Error() string {
	return fmt.Sprintf("invalid status transition from %s to %s", e.From, e.To)
}

func (e *InvalidStatusTransitionError) Unwrap() error {
	return ErrInvalidTransition
}

// AttributesValidationError carries one or more catalog validation failures,
// each bound to a specific attribute field. It wraps ErrInvalidInput so the
// HTTP layer can unwrap it and render field-level errors in the problem+json
// response.
type AttributesValidationError struct {
	Errors []domain.AttributeValidationError
}

func (e *AttributesValidationError) Error() string {
	return fmt.Sprintf("invalid property attributes: %d error(s)", len(e.Errors))
}

func (e *AttributesValidationError) Unwrap() error {
	return ErrInvalidInput
}
