package domain

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type TargetType string

const (
	TargetOperation          TargetType = "operation"
	TargetRecurringOperation TargetType = "recurring_operation"
	TargetLease              TargetType = "lease"
)

type EventType string

const (
	EventOperationDue        EventType = "operation_due"
	EventLeaseExpiring       EventType = "lease_expiring"
	EventLeaseRequiresAction EventType = "lease_requires_action"
)

type ReminderStatus string

const (
	ReminderPending   ReminderStatus = "pending"
	ReminderSending   ReminderStatus = "sending"
	ReminderSent      ReminderStatus = "sent"
	ReminderFailed    ReminderStatus = "failed"
	ReminderCancelled ReminderStatus = "cancelled"
)

const fixedDispatchHour = 10
const fixedDispatchTZ = "Europe/Moscow"

var ErrInvalidReminderDate = errors.New("reminder date must be today or in the future")

var (
	moscowLoc     *time.Location
	moscowLocOnce sync.Once
)

func moscowLocation() *time.Location {
	moscowLocOnce.Do(func() {
		var err error
		moscowLoc, err = time.LoadLocation(fixedDispatchTZ)
		if err != nil {
			panic(fmt.Sprintf("load %s timezone: %v", fixedDispatchTZ, err))
		}
	})
	return moscowLoc
}

// Reminder is a concrete scheduled notification.
type Reminder struct {
	ID                   uuid.UUID
	OwnerID              uuid.UUID
	TargetType           TargetType
	OperationID          *uuid.UUID
	RecurringOperationID *uuid.UUID
	LeaseID              *uuid.UUID
	PropertyID           *uuid.UUID
	EventType            EventType
	Status               ReminderStatus
	ScheduledAt          time.Time
	SentAt               *time.Time
	FailedAttempts       int
	NextAttemptAt        *time.Time
	MessageTitle         string
	MessageBody          string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// ScheduledAtForDate combines a user-selected date with the fixed dispatch time.
func ScheduledAtForDate(date time.Time) time.Time {
	d := time.Date(date.Year(), date.Month(), date.Day(), fixedDispatchHour, 0, 0, 0, moscowLocation())
	return d.UTC()
}

// ReminderOffset returns the number of days between an operation date and a reminder date.
func ReminderOffset(operationDate, reminderDate time.Time) int {
	op := operationDate.UTC().Truncate(24 * time.Hour)
	rm := reminderDate.UTC().Truncate(24 * time.Hour)
	return int(op.Sub(rm).Hours() / 24)
}

// ValidateReminderDate checks that a reminder date is today or in the future.
func ValidateReminderDate(reminderDate, now time.Time) error {
	rm := reminderDate.UTC().Truncate(24 * time.Hour)
	today := now.UTC().Truncate(24 * time.Hour)
	if rm.Before(today) {
		return fmt.Errorf("%w: %s", ErrInvalidReminderDate, reminderDate)
	}
	return nil
}

// NewOperationReminder creates a pending reminder for a future operation.
func NewOperationReminder(ownerID, operationID, propertyID uuid.UUID, reminderDate time.Time, title, body string, now time.Time) (Reminder, error) {
	if err := ValidateReminderDate(reminderDate, now); err != nil {
		return Reminder{}, err
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return Reminder{}, fmt.Errorf("generate reminder id: %w", err)
	}
	return Reminder{
		ID:           id,
		OwnerID:      ownerID,
		TargetType:   TargetOperation,
		OperationID:  &operationID,
		PropertyID:   &propertyID,
		EventType:    EventOperationDue,
		Status:       ReminderPending,
		ScheduledAt:  ScheduledAtForDate(reminderDate),
		MessageTitle: title,
		MessageBody:  body,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// NewLeaseReminder creates a pending reminder for a lease event.
func NewLeaseReminder(ownerID, leaseID, propertyID uuid.UUID, reminderDate time.Time, title, body string, eventType EventType, now time.Time) (Reminder, error) {
	if err := ValidateReminderDate(reminderDate, now); err != nil {
		return Reminder{}, err
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return Reminder{}, fmt.Errorf("generate reminder id: %w", err)
	}
	return Reminder{
		ID:           id,
		OwnerID:      ownerID,
		TargetType:   TargetLease,
		LeaseID:      &leaseID,
		PropertyID:   &propertyID,
		EventType:    eventType,
		Status:       ReminderPending,
		ScheduledAt:  ScheduledAtForDate(reminderDate),
		MessageTitle: title,
		MessageBody:  body,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}
