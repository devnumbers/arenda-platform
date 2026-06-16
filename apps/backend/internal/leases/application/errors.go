package application

import (
	"errors"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

var (
	ErrNotFound              = errors.New("not found")
	ErrInvalidInput          = errors.New("invalid lease input")
	ErrPropertyNotAvailable  = errors.New("property is not available for a lease")
	ErrOpenLeaseExists       = errors.New("property already has an open lease")
	ErrInvalidTransition     = errors.New("invalid lease status transition")
	ErrAlreadyCompleted      = errors.New("lease is already completed")
	ErrArchivedLease         = errors.New("cannot modify an archived lease")
	ErrTenantContactNotFound = errors.New("tenant contact not found")
)

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
