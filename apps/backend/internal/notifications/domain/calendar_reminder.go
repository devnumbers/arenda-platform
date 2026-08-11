package domain

import (
	"time"

	"github.com/google/uuid"
)

// CalendarReminderType is the display category of a calendar entry.
type CalendarReminderType string

const (
	CalendarTypeFree      CalendarReminderType = "free"
	CalendarTypeOperation CalendarReminderType = "operation"
	CalendarTypeSystem    CalendarReminderType = "system"
)

// CalendarReminder is a read projection: a single entry in the calendar
// agenda. It unifies free, operation, and system reminders into one display
// shape. Free reminders are expanded from their template on read and carry no
// status; operation/system reminders come from the reminders table with a
// status. Orphan reminders (property deleted in detach mode) have a nil
// PropertyID and PropertyName with HasProperty=false.
type CalendarReminder struct {
	ID             uuid.UUID
	Type           CalendarReminderType
	ScheduledAt    time.Time
	Title          string
	PropertyID     *uuid.UUID
	PropertyName   *string
	HasProperty    bool
	Status         *ReminderStatus // nil for free reminders
	EventType      *EventType      // nil for free reminders
	OperationID    *uuid.UUID
	LeaseID        *uuid.UUID
	FreeReminderID *uuid.UUID
	Periodicity    *FreeReminderPeriodicity // non-nil for free reminders
}

// CalendarReminderTypeFromTarget derives the display type from a reminder's
// target/event type: free->free, operation/recurring_operation->operation,
// lease->system.
func CalendarReminderTypeFromTarget(targetType TargetType) CalendarReminderType {
	switch targetType {
	case TargetFree:
		return CalendarTypeFree
	case TargetOperation, TargetRecurringOperation:
		return CalendarTypeOperation
	case TargetLease:
		return CalendarTypeSystem
	default:
		return CalendarTypeOperation
	}
}
