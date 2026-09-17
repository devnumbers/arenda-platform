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

// QueueEmail and QueuePush are the two River queues. Separate queues keep the
// SMTP ceiling and the push fan-out from starving each other.
const (
	QueueEmail = "notifications_email"
	QueuePush  = "notifications_push"
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
