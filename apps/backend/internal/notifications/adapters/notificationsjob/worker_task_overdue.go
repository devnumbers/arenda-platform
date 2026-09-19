package notificationsjob

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/riverqueue/river"
)

// TaskOverdueWorker wakes at one timed task's due minute (issue #750) and
// hands the id to the publisher's delivery-time resolution: the task is
// reloaded at wake-up — a task gone with its rule edit, completed, or turned
// date-only finishes the job without publishing, a live one publishes
// «Задача просрочена» through the pipeline (the dedup key keeps the sweep's
// row and the job's row to one).
type TaskOverdueWorker struct {
	river.WorkerDefaults[TaskOverdueArgs]
	publisher application.TaskOverdueDeliverer
	log       *slog.Logger
}

// NewTaskOverdueWorker builds the due-minute worker over the publisher.
func NewTaskOverdueWorker(publisher application.TaskOverdueDeliverer, log *slog.Logger) *TaskOverdueWorker {
	if log == nil {
		log = slog.Default()
	}
	return &TaskOverdueWorker{publisher: publisher, log: log}
}

// Work publishes the task's overdue notification — or nothing, when the
// reload answers nil.
func (w *TaskOverdueWorker) Work(ctx context.Context, job *river.Job[TaskOverdueArgs]) error {
	if err := w.publisher.DeliverTaskOverdue(ctx, job.Args.TaskID); err != nil {
		return fmt.Errorf("deliver task overdue %s: %w", job.Args.TaskID, err)
	}
	w.log.DebugContext(ctx, "task overdue job finished",
		slog.String("task_id", job.Args.TaskID.String()))
	return nil
}
