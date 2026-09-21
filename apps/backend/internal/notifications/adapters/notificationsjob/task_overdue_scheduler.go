package notificationsjob

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/riverqueue/river"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.TaskOverdueScheduler = (*TaskOverdueScheduler)(nil)

// The task boundary job shares the package's retry budget
// (boundaryJobMaxAttempts) — the tasks', payments' and rentals' boundary
// jobs carry the same one-feed-write ladder.

// TaskOverdueScheduler books a dated task's boundary job on the River
// client (issues #750, #777): the job wakes at the task's boundary instant
// — the timed term or the date-only day-after midnight — in the owner's
// timezone — the minute precision the hourly scan cannot give (решение
// #737). Unique by the task id in every non-terminal state, so the hourly
// scan's repeated asks are idempotent — a duplicate returns the standing
// job, nothing is enqueued twice. A term moved by a rule edit removes the
// task and materializes a new one with a new id; the orphaned job wakes and
// finds nothing — a no-op by the worker's delivery-time resolution.
type TaskOverdueScheduler struct {
	client *river.Client[pgx.Tx]
}

// NewTaskOverdueScheduler builds the boundary scheduler over the delivery
// queue's River client.
func NewTaskOverdueScheduler(client *river.Client[pgx.Tx]) *TaskOverdueScheduler {
	return &TaskOverdueScheduler{client: client}
}

// ScheduleTaskOverdue books the task's boundary job at the given instant.
func (s *TaskOverdueScheduler) ScheduleTaskOverdue(ctx context.Context, taskID uuid.UUID, dueAt time.Time) error {
	_, err := s.client.Insert(ctx, TaskOverdueArgs{TaskID: taskID}, &river.InsertOpts{
		Queue:       QueueTasks,
		ScheduledAt: dueAt,
		MaxAttempts: boundaryJobMaxAttempts,
		UniqueOpts:  jobUniqueOpts,
	})
	if err != nil {
		return err
	}
	return nil
}
