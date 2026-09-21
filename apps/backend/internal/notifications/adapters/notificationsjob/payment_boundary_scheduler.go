package notificationsjob

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.PaymentBoundaryScheduler = (*PaymentBoundaryScheduler)(nil)

// boundaryJobMaxAttempts is every boundary job's retry budget (the tasks',
// payments' and rentals' jobs alike): the publication is one feed write, a
// short ladder covers a database blip without pinning a dead target's job
// to the queue (the scan cadence canon).
const boundaryJobMaxAttempts = 5

// PaymentBoundaryScheduler books a payment operation's boundary jobs on the
// River client (issue #776): the due leg's job wakes at 00:00 of the
// operation date in the owner's timezone, the overdue leg's — at 00:00 of
// the day after. Unique by (kind, rule id, date) in every non-terminal
// state, so the hourly scan's repeated asks are idempotent — a duplicate
// returns the standing job, nothing is enqueued twice. A moved or paid
// operation's job wakes and finds nothing — a no-op by the worker's
// delivery-time resolution.
type PaymentBoundaryScheduler struct {
	client *river.Client[pgx.Tx]
}

// NewPaymentBoundaryScheduler builds the boundary scheduler over the
// delivery queue's River client.
func NewPaymentBoundaryScheduler(client *river.Client[pgx.Tx]) *PaymentBoundaryScheduler {
	return &PaymentBoundaryScheduler{client: client}
}

// uniqueOpts is the boundary jobs' uniqueness: one in-flight job per
// (kind, args) across every non-terminal state.
var uniqueOpts = river.UniqueOpts{
	ByArgs: true,
	ByState: []rivertype.JobState{
		rivertype.JobStateAvailable,
		rivertype.JobStatePending,
		rivertype.JobStateRunning,
		rivertype.JobStateRetryable,
		rivertype.JobStateScheduled,
	},
}

// SchedulePaymentDue books the operation's «Оплатите платёж» job at the
// given boundary instant.
func (s *PaymentBoundaryScheduler) SchedulePaymentDue(ctx context.Context, paymentID uuid.UUID, date, fireAt time.Time) error {
	_, err := s.client.Insert(ctx, PaymentDueArgs{PaymentID: paymentID, DueDate: date}, &river.InsertOpts{
		Queue:       QueuePayments,
		ScheduledAt: fireAt,
		MaxAttempts: boundaryJobMaxAttempts,
		UniqueOpts:  uniqueOpts,
	})
	if err != nil {
		return err
	}
	return nil
}

// SchedulePaymentOverdue books the operation's «Платёж просрочен» job at the
// given boundary instant.
func (s *PaymentBoundaryScheduler) SchedulePaymentOverdue(ctx context.Context, paymentID uuid.UUID, date, fireAt time.Time) error {
	_, err := s.client.Insert(ctx, PaymentOverdueArgs{PaymentID: paymentID, DueDate: date}, &river.InsertOpts{
		Queue:       QueuePayments,
		ScheduledAt: fireAt,
		MaxAttempts: boundaryJobMaxAttempts,
		UniqueOpts:  uniqueOpts,
	})
	if err != nil {
		return err
	}
	return nil
}
