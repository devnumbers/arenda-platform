package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
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
	EventOperationOverdue    EventType = "operation_overdue"
	EventLeaseExpiring       EventType = "lease_expiring"
	EventLeaseRequiresAction EventType = "lease_requires_action"
	// EventSubscriptionGrace covers the billing subscription grace lifecycle
	// notices (issue #253): the failed-renewal alert when the subscription
	// enters grace and the reminder before the grace window ends. One event
	// type — both moments say the same thing to the user: fix the payment
	// method or lose the tariff.
	EventSubscriptionGrace EventType = "subscription_grace"
)

type ReminderStatus string

const (
	ReminderPending   ReminderStatus = "pending"
	ReminderSending   ReminderStatus = "sending"
	ReminderSent      ReminderStatus = "sent"
	ReminderFailed    ReminderStatus = "failed"
	ReminderCancelled ReminderStatus = "cancelled"
	// ReminderSkipped is terminal: the owner revoked permission for the
	// reminder's event type, so it was not sent and must not be retried.
	ReminderSkipped ReminderStatus = "skipped"
)

var ErrInvalidReminderDate = errors.New("reminder date must be today or in the future")

// propertyIDPtr returns a pointer to id if it is not the zero UUID; otherwise
// it returns nil. A zero UUID means the entity is not attached to a property
// (the property was deleted in detach mode); persisting it as-is would violate
// the reminders_property_id_fkey foreign key.
func propertyIDPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
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

// ScheduledAtForDate combines a user-selected date with a dispatch hour in the
// owner's timezone and returns the corresponding UTC instant.
func ScheduledAtForDate(date time.Time, loc *time.Location, hour int) time.Time {
	d := time.Date(date.In(loc).Year(), date.In(loc).Month(), date.In(loc).Day(), hour, 0, 0, 0, loc)
	return d.UTC()
}

// ReminderOffset returns the number of days between an operation date and a reminder date,
// interpreted in the given timezone.
func ReminderOffset(operationDate, reminderDate time.Time, loc *time.Location) int {
	op := timeutil.DateIn(operationDate, loc)
	rm := timeutil.DateIn(reminderDate, loc)
	return int(op.Sub(rm).Hours() / 24)
}

// ValidateReminderDate checks that a reminder date is today or in the future,
// interpreted in the given timezone.
func ValidateReminderDate(reminderDate, now time.Time, loc *time.Location) error {
	rm := timeutil.DateIn(reminderDate, loc)
	today := timeutil.DateIn(now, loc)
	if rm.Before(today) {
		return fmt.Errorf("%w: %s", ErrInvalidReminderDate, reminderDate)
	}
	return nil
}

// NewOperationReminder creates a pending reminder for a future operation.
func NewOperationReminder(
	ownerID, operationID, propertyID uuid.UUID,
	reminderDate time.Time,
	title, body string,
	now time.Time,
	loc *time.Location,
	hour int,
) (Reminder, error) {
	return newOperationEventReminder(ownerID, operationID, propertyID, EventOperationDue, reminderDate, title, body, now, loc, hour)
}

// NewOperationOverdueReminder creates a pending reminder for an overdue operation.
func NewOperationOverdueReminder(
	ownerID, operationID, propertyID uuid.UUID,
	reminderDate time.Time,
	title, body string,
	now time.Time,
	loc *time.Location,
	hour int,
) (Reminder, error) {
	return newOperationEventReminder(ownerID, operationID, propertyID, EventOperationOverdue, reminderDate, title, body, now, loc, hour)
}

func newOperationEventReminder(
	ownerID, operationID, propertyID uuid.UUID,
	eventType EventType,
	reminderDate time.Time,
	title, body string,
	now time.Time,
	loc *time.Location,
	hour int,
) (Reminder, error) {
	if err := ValidateReminderDate(reminderDate, now, loc); err != nil {
		return Reminder{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Reminder{}, fmt.Errorf("generate reminder id: %w", err)
	}
	return Reminder{
		ID:           id,
		OwnerID:      ownerID,
		TargetType:   TargetOperation,
		OperationID:  &operationID,
		PropertyID:   propertyIDPtr(propertyID),
		EventType:    eventType,
		Status:       ReminderPending,
		ScheduledAt:  ScheduledAtForDate(reminderDate, loc, hour),
		MessageTitle: title,
		MessageBody:  body,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// NewLeaseReminder creates a pending reminder for a lease event.
func NewLeaseReminder(
	ownerID, leaseID, propertyID uuid.UUID,
	reminderDate time.Time,
	title, body string,
	eventType EventType,
	now time.Time,
	loc *time.Location,
	hour int,
) (Reminder, error) {
	if err := ValidateReminderDate(reminderDate, now, loc); err != nil {
		return Reminder{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Reminder{}, fmt.Errorf("generate reminder id: %w", err)
	}
	return Reminder{
		ID:           id,
		OwnerID:      ownerID,
		TargetType:   TargetLease,
		LeaseID:      &leaseID,
		PropertyID:   propertyIDPtr(propertyID),
		EventType:    eventType,
		Status:       ReminderPending,
		ScheduledAt:  ScheduledAtForDate(reminderDate, loc, hour),
		MessageTitle: title,
		MessageBody:  body,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}
