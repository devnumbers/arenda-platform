// Package notificationsjob is the delivery-queue adapter of the notifications
// context (карта #734, #740): River job args and workers that deliver a
// stored feed row over email and Web Push, and the transactional enqueuer the
// application publisher schedules deliveries through (ADR 0057). Jobs carry
// only the notification id — the worker reloads the committed feed row, so
// the row stays the single source of the content.
package notificationsjob

import (
	"github.com/google/uuid"
)

// QueueEmail and QueuePush are the two delivery queues; QueueTasks carries
// the timed tasks' due-minute jobs (issue #750). Separate queues keep the
// SMTP ceiling, the push fan-out and the scheduled task jobs from starving
// each other.
const (
	QueueEmail = "notifications_email"
	QueuePush  = "notifications_push"
	QueueTasks = "notifications_tasks"
)

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

// TaskOverdueArgs publishes one timed task's «Задача просрочена» at its due
// minute (issue #750). The args are the job's dedup key: one task has at
// most one overdue job in flight — the hourly scan re-asks freely and River
// answers the standing job. The worker reloads the task at wake-up, so the
// job never carries content, only the id.
type TaskOverdueArgs struct {
	TaskID uuid.UUID `json:"task_id"`
}

// Kind identifies the job kind to River.
func (TaskOverdueArgs) Kind() string { return "notifications:task_overdue" }
