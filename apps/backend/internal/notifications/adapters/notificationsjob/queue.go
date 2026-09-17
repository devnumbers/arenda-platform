package notificationsjob

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.DeliveryQueue = (*RiverQueue)(nil)

// RiverQueue schedules notification deliveries through the River client. The
// enqueue happens inside the publisher's transaction (InsertTx), so a
// delivery job commits together with its feed row.
//
// Unique jobs make the enqueue idempotent: one notification has at most one
// in-flight job per channel (args = the notification id, all states except
// the terminal ones), so a retried publication after a crash cannot
// double-book a delivery. Delivery itself stays at-least-once — a crash
// between the SMTP send and the job's completion can resend (ADR 0057): a
// duplicate message beats a lost one.
type RiverQueue struct {
	client *river.Client[pgx.Tx]
	// PushEnabled-style flag: without VAPID keys there is no push sender, so
	// push jobs are not enqueued (the same email-only local mode the
	// direct-notification service has).
	pushEnabled bool

	emailInsertOpts *river.InsertOpts
	pushInsertOpts  *river.InsertOpts
}

// NewRiverQueue builds the enqueuer over a River client. The max-attempts
// budgets come from the notifications queue config; unique in-flight
// enforcement covers every non-terminal state, so a completed job frees the
// notification's key for a future publication.
func NewRiverQueue(client *river.Client[pgx.Tx], pushEnabled bool, emailMaxAttempts, pushMaxAttempts int) *RiverQueue {
	uniqueOpts := river.UniqueOpts{
		ByArgs: true,
		ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRunning,
			rivertype.JobStateRetryable,
			rivertype.JobStateScheduled,
		},
	}
	return &RiverQueue{
		client:      client,
		pushEnabled: pushEnabled,
		emailInsertOpts: &river.InsertOpts{
			Queue:       QueueEmail,
			MaxAttempts: emailMaxAttempts,
			UniqueOpts:  uniqueOpts,
		},
		pushInsertOpts: &river.InsertOpts{
			Queue:       QueuePush,
			MaxAttempts: pushMaxAttempts,
			UniqueOpts:  uniqueOpts,
		},
	}
}

// EnqueueEmail schedules the notification's email leg in the caller's
// transaction.
func (q *RiverQueue) EnqueueEmail(ctx context.Context, tx transaction.Tx, notificationID uuid.UUID) error {
	ptx, err := database.PgxTxOf(tx)
	if err != nil {
		return err
	}
	if _, err := q.client.InsertTx(ctx, ptx, DeliverEmailArgs{NotificationID: notificationID}, q.emailInsertOpts); err != nil {
		return fmt.Errorf("insert %s job: %w", DeliverEmailArgs{}.Kind(), err)
	}
	return nil
}

// EnqueuePush schedules the notification's Web Push leg in the caller's
// transaction. Without a configured push sender it is a no-op.
func (q *RiverQueue) EnqueuePush(ctx context.Context, tx transaction.Tx, notificationID uuid.UUID) error {
	if !q.pushEnabled {
		return nil
	}
	ptx, err := database.PgxTxOf(tx)
	if err != nil {
		return err
	}
	if _, err := q.client.InsertTx(ctx, ptx, DeliverPushArgs{NotificationID: notificationID}, q.pushInsertOpts); err != nil {
		return fmt.Errorf("insert %s job: %w", DeliverPushArgs{}.Kind(), err)
	}
	return nil
}
