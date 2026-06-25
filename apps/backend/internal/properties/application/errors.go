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

	ErrAddressSuggestFailed = errors.New("address suggestion request failed")

	ErrArchivedProperty     = errors.New("cannot modify an archived property")
	ErrAlreadyArchived      = errors.New("property is already archived")
	ErrNotArchived          = errors.New("property is not archived")
	ErrPropertyHasOpenLease = errors.New("property has an open lease")
	ErrPhotoLimitReached    = errors.New("property photo limit reached")
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
