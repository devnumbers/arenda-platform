package domain

import (
	"time"

	"github.com/google/uuid"
)

type OperationType string

const (
	OperationTypeIncome  OperationType = "income"
	OperationTypeExpense OperationType = "expense"
)

type OperationCategory string

const (
	OperationCategoryRent OperationCategory = "rent"
)

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
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

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
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
