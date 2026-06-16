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

type OperationCategory string

const (
	OperationCategoryRent        OperationCategory = "rent"
	OperationCategoryOtherIncome OperationCategory = "other_income"
	OperationCategoryUtilities   OperationCategory = "utilities"
	OperationCategoryRepair      OperationCategory = "repair"
	OperationCategoryTax         OperationCategory = "tax"
	OperationCategoryOtherExpense OperationCategory = "other_expense"
)

var ErrInvalidOperationCategory = fmt.Errorf("invalid operation category")

func ParseOperationCategory(s string) (OperationCategory, error) {
	c := OperationCategory(s)
	switch c {
	case OperationCategoryRent,
		OperationCategoryOtherIncome,
		OperationCategoryUtilities,
		OperationCategoryRepair,
		OperationCategoryTax,
		OperationCategoryOtherExpense:
		return c, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidOperationCategory, s)
	}
}

func IsValidCategoryForType(category OperationCategory, opType OperationType) bool {
	switch opType {
	case OperationTypeIncome:
		switch category {
		case OperationCategoryRent, OperationCategoryOtherIncome:
			return true
		}
	case OperationTypeExpense:
		switch category {
		case OperationCategoryUtilities, OperationCategoryRepair, OperationCategoryTax, OperationCategoryOtherExpense:
			return true
		}
	}
	return false
}

type Operation struct {
	ID                   uuid.UUID
	OwnerID              uuid.UUID
	PropertyID           uuid.UUID
	LeaseID              uuid.UUID
	RecurringOperationID uuid.UUID
	Type                 string
	Category             string
	AmountKopecks        int64
	OperationDate        time.Time
	Comment              string
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
)

type RecurringOperation struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	PropertyID    uuid.UUID
	LeaseID       uuid.UUID
	Type          string
	Category      string
	AmountKopecks int64
	StartDate     time.Time
	PaymentDay    int
	EndDate       *time.Time
	Periodicity   string
	Status        string
	Comment       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
