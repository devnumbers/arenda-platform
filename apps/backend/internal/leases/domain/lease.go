package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type LeaseStatus string

const (
	LeaseStatusAwaitingStart  LeaseStatus = "awaiting_start"
	LeaseStatusActive         LeaseStatus = "active"
	LeaseStatusRequiresAction LeaseStatus = "requires_action"
	LeaseStatusCompleted      LeaseStatus = "completed"
	LeaseStatusArchived       LeaseStatus = "archived"
)

var ErrInvalidLeaseStatus = errors.New("invalid lease status")

func ParseLeaseStatus(s string) (LeaseStatus, error) {
	st := LeaseStatus(s)
	if !st.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidLeaseStatus, s)
	}
	return st, nil
}

func (s LeaseStatus) Valid() bool {
	switch s {
	case LeaseStatusAwaitingStart,
		LeaseStatusActive,
		LeaseStatusRequiresAction,
		LeaseStatusCompleted,
		LeaseStatusArchived:
		return true
	}
	return false
}

func (s LeaseStatus) IsOpen() bool {
	switch s {
	case LeaseStatusAwaitingStart,
		LeaseStatusActive,
		LeaseStatusRequiresAction:
		return true
	case LeaseStatusCompleted,
		LeaseStatusArchived:
		return false
	}
	return false
}

type Lease struct {
	ID                   uuid.UUID
	OwnerID              uuid.UUID
	PropertyID           uuid.UUID
	TenantContactID      *uuid.UUID
	Status               LeaseStatus
	StartDate            time.Time
	EndDate              *time.Time
	RentAmountKopecks    int64
	DepositAmountKopecks int64
	PaymentDay           int
	Comment              string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

var (
	ErrInvalidRentAmount    = errors.New("rent amount must be non-negative")
	ErrInvalidDepositAmount = errors.New("deposit amount must be non-negative")
	ErrInvalidPaymentDay    = errors.New("payment day must be between 1 and 31")
	ErrEndDateBeforeStart   = errors.New("end date must be on or after start date")
)

func NewLease(ownerID, propertyID uuid.UUID, startDate time.Time, rent int64, paymentDay int) (Lease, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Lease{}, fmt.Errorf("generate lease id: %w", err)
	}

	lease := Lease{
		ID:                id,
		OwnerID:           ownerID,
		PropertyID:        propertyID,
		StartDate:         startDate,
		RentAmountKopecks: rent,
		PaymentDay:        paymentDay,
		Status:            LeaseStatusAwaitingStart,
	}

	if err := lease.Validate(); err != nil {
		return Lease{}, err
	}

	return lease, nil
}

func (l Lease) Validate() error {
	if l.RentAmountKopecks < 0 {
		return ErrInvalidRentAmount
	}
	if l.DepositAmountKopecks < 0 {
		return ErrInvalidDepositAmount
	}
	if l.PaymentDay < 1 || l.PaymentDay > 31 {
		return ErrInvalidPaymentDay
	}
	if l.EndDate != nil && date(*l.EndDate).Before(date(l.StartDate)) {
		return ErrEndDateBeforeStart
	}
	if !l.Status.Valid() {
		return ErrInvalidLeaseStatus
	}
	return nil
}

// CalculateStatus returns the date-driven status for the lease.
// It does not account for terminal statuses such as completed or archived.
func (l Lease) CalculateStatus(now time.Time) LeaseStatus {
	today := date(now)
	start := date(l.StartDate)

	if today.Before(start) {
		return LeaseStatusAwaitingStart
	}

	if l.EndDate != nil {
		end := date(*l.EndDate)
		if today.After(end) {
			return LeaseStatusRequiresAction
		}
	}

	return LeaseStatusActive
}

// EffectiveStatus returns the status that should be observed for the lease at
// the given moment. Terminal statuses (completed, archived) are preserved; open
// statuses are recomputed from the current date.
func (l Lease) EffectiveStatus(now time.Time) LeaseStatus {
	if !l.Status.IsOpen() {
		return l.Status
	}
	return l.CalculateStatus(now)
}

func (l Lease) IsOpen() bool {
	return l.Status.IsOpen()
}
