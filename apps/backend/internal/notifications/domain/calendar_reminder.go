package domain

import (
	"time"

	"github.com/google/uuid"
)

// CalendarReminderType is the display category of a calendar entry.
type CalendarReminderType string

const (
	CalendarTypeOperation CalendarReminderType = "operation"
	CalendarTypeSystem    CalendarReminderType = "system"
)

// CalendarReminder is a read projection: a single entry in the calendar
// agenda. It unifies operation and system reminders into one display shape.
// Orphan reminders (property deleted in detach mode) have a nil PropertyID and
// PropertyName with HasProperty=false.
type CalendarReminder struct {
	ID           uuid.UUID
	Type         CalendarReminderType
	ScheduledAt  time.Time
	Title        string
	PropertyID   *uuid.UUID
	PropertyName *string
	HasProperty  bool
	Status       *ReminderStatus
	EventType    *EventType
	OperationID  *uuid.UUID
	LeaseID      *uuid.UUID
}

// CalendarReminderTypeFromTarget derives the display type from a reminder's
// target type: operation/recurring_operation->operation, lease->system.
func CalendarReminderTypeFromTarget(targetType TargetType) CalendarReminderType {
	switch targetType {
	case TargetOperation, TargetRecurringOperation:
		return CalendarTypeOperation
	case TargetLease:
		return CalendarTypeSystem
	default:
		return CalendarTypeOperation
	}
}
