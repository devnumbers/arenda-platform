package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type OperationType string

const (
	OperationTypeIncome  OperationType = "income"
	OperationTypeExpense OperationType = "expense"
)

var ErrInvalidOperationType = fmt.Errorf("invalid operation type")

func ParseOperationType(s string) (OperationType, error) {
	t := OperationType(s)
	switch t {
	case OperationTypeIncome, OperationTypeExpense:
		return t, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidOperationType, s)
	}
}

type OperationCategory struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Type      OperationType
	Name      string
	Code      *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OperationCategoryDefaultCode string

const (
	OperationCategoryCodeRent      OperationCategoryDefaultCode = "rent"
	OperationCategoryCodeUtilities OperationCategoryDefaultCode = "utilities"
	OperationCategoryCodeRepair    OperationCategoryDefaultCode = "repair"
	OperationCategoryCodeTax       OperationCategoryDefaultCode = "tax"
)

type OperationStatus string

const (
	OperationStatusPending  OperationStatus = "pending"
	OperationStatusOverdue  OperationStatus = "overdue"
	OperationStatusPaid     OperationStatus = "paid"
	OperationStatusReceived OperationStatus = "received"
)

var ErrInvalidOperationStatus = fmt.Errorf("invalid operation status")

func ParseOperationStatus(s string) (OperationStatus, error) {
	st := OperationStatus(s)
	if !st.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidOperationStatus, s)
	}
	return st, nil
}

func (s OperationStatus) Valid() bool {
	switch s {
	case OperationStatusPending,
		OperationStatusOverdue,
		OperationStatusPaid,
		OperationStatusReceived:
		return true
	}
	return false
}

// IsCompleted reports whether the operation has reached a terminal status.
func (s OperationStatus) IsCompleted() bool {
	switch s {
	case OperationStatusPaid, OperationStatusReceived:
		return true
	}
	return false
}

// CanComplete reports whether the operation may be marked as completed.
func (s OperationStatus) CanComplete() bool {
	switch s {
	case OperationStatusPending, OperationStatusOverdue:
		return true
	}
	return false
}

// CanBecomeOverdue reports whether the operation may transition to overdue.
func (s OperationStatus) CanBecomeOverdue() bool {
	return s == OperationStatusPending
}

var (
	ErrStatusPaidRequiresExpense    = fmt.Errorf("status paid is only valid for expense operations")
	ErrStatusReceivedRequiresIncome = fmt.Errorf("status received is only valid for income operations")
)

// ValidateStatusForType checks that the status is valid and compatible with the operation type.
func (o Operation) ValidateStatusForType() error {
	if !o.Status.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidOperationStatus, o.Status)
	}
	switch o.Status {
	case OperationStatusPaid:
		if o.Type != OperationTypeExpense {
			return ErrStatusPaidRequiresExpense
		}
	case OperationStatusReceived:
		if o.Type != OperationTypeIncome {
			return ErrStatusReceivedRequiresIncome
		}
	}
	return nil
}

type Operation struct {
	ID                   uuid.UUID
	OwnerID              uuid.UUID
	PropertyID           uuid.UUID
	LeaseID              uuid.UUID
	RecurringOperationID uuid.UUID
	Type                 OperationType
	CategoryID           uuid.UUID
	Status               OperationStatus
	Name                 string
	AmountKopecks        int64
	OperationDate        time.Time
	SourceOperationDate  *time.Time
	Comment              string
	ReminderOffsetDays   *int
	IsException          bool
	DeletedAt            *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type RecurringOperationStatus string

const (
	RecurringOperationStatusActive RecurringOperationStatus = "active"
	RecurringOperationStatusPaused RecurringOperationStatus = "paused"
)

type RecurringOperationPeriodicity string

const (
	RecurringOperationPeriodicityMonthly RecurringOperationPeriodicity = "monthly"
	RecurringOperationPeriodicityYearly  RecurringOperationPeriodicity = "yearly"
)

var ErrInvalidRecurringOperationPeriodicity = fmt.Errorf("invalid recurring operation periodicity")

func ParseRecurringOperationPeriodicity(s string) (RecurringOperationPeriodicity, error) {
	p := RecurringOperationPeriodicity(s)
	if !p.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidRecurringOperationPeriodicity, s)
	}
	return p, nil
}

func (p RecurringOperationPeriodicity) Valid() bool {
	switch p {
	case RecurringOperationPeriodicityMonthly, RecurringOperationPeriodicityYearly:
		return true
	}
	return false
}

type RecurringOperation struct {
	ID                 uuid.UUID
	OwnerID            uuid.UUID
	PropertyID         uuid.UUID
	LeaseID            uuid.UUID
	Type               OperationType
	CategoryID         uuid.UUID
	AmountKopecks      int64
	StartDate          time.Time
	PaymentDay         int
	EndDate            *time.Time
	ReminderOffsetDays *int
	Periodicity        RecurringOperationPeriodicity
	Status             RecurringOperationStatus
	Name               string
	Comment            string
	DeletedAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
