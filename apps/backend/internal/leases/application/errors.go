package application

import (
	"errors"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

var (
	ErrNotFound              = errors.New("not found")
	ErrInvalidInput          = errors.New("invalid input")
	ErrPropertyNotAvailable  = errors.New("property is not available for a lease")
	ErrOpenLeaseExists       = errors.New("property already has an open lease")
	ErrInvalidTransition     = errors.New("invalid lease status transition")
	ErrAlreadyCompleted      = errors.New("lease is already completed")
	ErrOperationAlreadyCompleted          = errors.New("operation is already completed")
	ErrRecurringOperationLeaseCreated     = errors.New("recurring operation created by a lease cannot be deleted")
	ErrArchivedLease                      = errors.New("cannot modify an archived lease")
	ErrTenantContactNotFound = errors.New("tenant contact not found")
	ErrDuplicatePhone        = errors.New("tenant contact with this phone already exists")
)

// invalidInputError is a user-facing invalid-input error that reports 400 in the
// transport layer without embedding the "invalid input: " sentinel text.
type invalidInputError struct {
	detail string
}

func (e *invalidInputError) Error() string { return e.detail }
func (e *invalidInputError) Is(target error) bool {
	return target == ErrInvalidInput
}

// newInvalidInputError returns an error that matches ErrInvalidInput but whose
// message is the supplied detail string.
func newInvalidInputError(detail string) error {
	return &invalidInputError{detail: detail}
}

// InvalidStatusTransitionError describes a status change that is not allowed.
type InvalidStatusTransitionError struct {
	From domain.LeaseStatus
	To   domain.LeaseStatus
}

func (e *InvalidStatusTransitionError) Error() string {
	return fmt.Sprintf("invalid status transition from %s to %s", e.From, e.To)
}

func (e *InvalidStatusTransitionError) Unwrap() error {
	return ErrInvalidTransition
}
