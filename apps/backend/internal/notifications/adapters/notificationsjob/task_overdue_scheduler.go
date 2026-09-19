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
var _ application.TaskOverdueScheduler = (*TaskOverdueScheduler)(nil)

// taskOverdueMaxAttempts is the due-minute job's retry budget: the
// publication is one feed write, a short ladder covers a database blip
// without pinning a dead task's job to the queue (a domain decision, not an
// env knob — the scan cadence canon).
const taskOverdueMaxAttempts = 5

// TaskOverdueScheduler books a timed task's due-minute job on the River
// client (issue #750): the job wakes at the term's instant in the owner's
// timezone — the minute precision the hourly scan cannot give (решение
// #737). Unique by the task id in every non-terminal state, so the hourly
// scan's repeated asks are idempotent — a duplicate returns the standing
// job, nothing is enqueued twice. A term moved by a rule edit removes the
// task and materializes a new one with a new id; the orphaned job wakes and
// finds nothing — a no-op by the worker's delivery-time resolution.
type TaskOverdueScheduler struct {
	client *river.Client[pgx.Tx]
}

// NewTaskOverdueScheduler builds the due-minute scheduler over the delivery
// queue's River client.
func NewTaskOverdueScheduler(client *river.Client[pgx.Tx]) *TaskOverdueScheduler {
	return &TaskOverdueScheduler{client: client}
}

// ScheduleTaskOverdue books the task's due-minute job at the given instant.
func (s *TaskOverdueScheduler) ScheduleTaskOverdue(ctx context.Context, taskID uuid.UUID, dueAt time.Time) error {
	_, err := s.client.Insert(ctx, TaskOverdueArgs{TaskID: taskID}, &river.InsertOpts{
		Queue:       QueueTasks,
		ScheduledAt: dueAt,
		MaxAttempts: taskOverdueMaxAttempts,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	})
	if err != nil {
		return err
	}
	return nil
}
