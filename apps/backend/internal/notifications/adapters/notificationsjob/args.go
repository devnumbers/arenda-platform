// Package notificationsjob is the delivery-queue adapter of the notifications
// context (карта #734, #740): River job args and workers that deliver a
// stored feed row over email and Web Push, and the transactional enqueuer the
// application publisher schedules deliveries through (ADR 0059). Jobs carry
// only the notification id — the worker reloads the committed feed row, so
// the row stays the single source of the content.
package notificationsjob

import (
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// QueueEmail and QueuePush are the two delivery queues; QueueTasks carries
// the dated tasks' boundary jobs (issues #750, #777), QueuePayments — the
// payment operations' boundary jobs (issue #776), QueueRentals — the
// rentals' completed-boundary jobs (issue #777). Separate queues keep the
// SMTP ceiling, the push fan-out and the scheduled jobs from starving each
// other.
const (
	QueueEmail    = "notifications_email"
	QueuePush     = "notifications_push"
	QueueTasks    = "notifications_tasks"
	QueuePayments = "notifications_payments"
	QueueRentals  = "notifications_rentals"
)

// jobUniqueOpts is every notifications job's uniqueness: one in-flight job
// per (kind, args) in every non-terminal state — the publications are
// idempotent, and a finished job frees the key for a future one.
var jobUniqueOpts = river.UniqueOpts{
	ByArgs: true,
	ByState: []rivertype.JobState{
		rivertype.JobStateAvailable,
		rivertype.JobStatePending,
		rivertype.JobStateRunning,
		rivertype.JobStateRetryable,
		rivertype.JobStateScheduled,
	},
}

// DeliverEmailArgs delivers one notification's email leg. The args are the
// job's dedup key: one notification has at most one email job in flight.
type DeliverEmailArgs struct {
	NotificationID uuid.UUID `json:"notification_id"`
}

// Kind identifies the job kind to River.
func (DeliverEmailArgs) Kind() string { return "notifications:deliver_email" }

// DeliverPushArgs delivers one notification's Web Push leg (the fan-out over
// the recipient's subscriptions happens inside the job). One notification has
// at most one push job in flight.
type DeliverPushArgs struct {
	NotificationID uuid.UUID `json:"notification_id"`
}

// Kind identifies the job kind to River.
func (DeliverPushArgs) Kind() string { return "notifications:deliver_push" }

// TaskOverdueArgs publishes one dated task's «Задача просрочена» at its
// boundary instant — the timed term's minute or the date-only day-after
// midnight (issues #750, #777). The args are the job's dedup key: one task
// has at most one overdue job in flight — the hourly scan re-asks freely
// and River answers the standing job. The worker reloads the task at
// wake-up, so the job never carries content, only the id.
type TaskOverdueArgs struct {
	TaskID uuid.UUID `json:"task_id"`
}

// Kind identifies the job kind to River.
func (TaskOverdueArgs) Kind() string { return "notifications:task_overdue" }

// PaymentDueArgs publishes one operation's «Оплатите платёж» at 00:00 of the
// operation date in the owner's timezone (issue #776). The args are the
// job's dedup key — the publication key's (rule, date) pair —: one
// operation has at most one due job in flight, the hourly scan re-asks
// freely and River answers the standing job. DueDate is the module's
// UTC-midnight date convention. The worker reloads the operation at
// wake-up, so the job never carries content, only the identity.
type PaymentDueArgs struct {
	PaymentID uuid.UUID `json:"payment_id"`
	DueDate   time.Time `json:"due_date"`
}

// Kind identifies the job kind to River.
func (PaymentDueArgs) Kind() string { return "notifications:payment_due" }

// PaymentOverdueArgs publishes one operation's «Платёж просрочен» at 00:00
// of the day after the operation date in the owner's timezone (issue #776).
// The args are the job's dedup key, the same shape as PaymentDueArgs; the
// two legs are different kinds, so one operation's due and overdue jobs
// coexist.
type PaymentOverdueArgs struct {
	PaymentID uuid.UUID `json:"payment_id"`
	DueDate   time.Time `json:"due_date"`
}

// Kind identifies the job kind to River.
func (PaymentOverdueArgs) Kind() string { return "notifications:payment_overdue" }

// RentalCompletedArgs publishes one rental's «Аренда завершена» at 00:00 of
// the day after its planned end in the owner's timezone (issue #777). The
// args are the job's dedup key — the publication key's (rental, planned end)
// pair —: one rental has at most one job per planned end in flight, the
// hourly scan re-asks freely and River answers the standing job.
// PlannedEndDate is the module's UTC-midnight date convention. The worker
// reloads the rental at wake-up, so the job never carries content, only the
// identity.
type RentalCompletedArgs struct {
	RentalID       uuid.UUID `json:"rental_id"`
	PlannedEndDate time.Time `json:"planned_end_date"`
}

// Kind identifies the job kind to River.
func (RentalCompletedArgs) Kind() string { return "notifications:rental_completed" }
