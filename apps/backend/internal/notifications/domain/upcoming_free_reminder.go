package domain

import (
	"time"

	"github.com/google/uuid"
)

// UpcomingFreeReminder is a single projected fire of a free reminder template.
// Unlike FreeReminder (a template), this represents one concrete occurrence:
// TriggerAt is the UTC instant at which this occurrence is scheduled to fire.
// It is a read projection assembled from a materialized reminders row joined
// with its parent template's periodicity.
type UpcomingFreeReminder struct {
	FreeReminderID uuid.UUID
	Title          string
	PropertyID     uuid.UUID
	TriggerAt      time.Time
	Periodicity    FreeReminderPeriodicity
}
