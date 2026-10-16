package notificationsjob

import (
	"context"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// NewTaskOverdueWorker builds the boundary worker over the publisher (issues
// #750, #777): the worker wakes at one dated task's boundary instant — the
// timed term's minute or the date-only day-after midnight — and hands the id
// to the publisher's delivery-time resolution: the task is reloaded at
// wake-up — a task gone with its rule edit or completed finishes the job
// without publishing, a live one publishes «Задача просрочена» through the
// pipeline (the dedup key keeps the sweep's row and the job's row to one).
func NewTaskOverdueWorker(publisher application.TaskOverdueDeliverer, log *slog.Logger) *boundaryWorker[TaskOverdueArgs] {
	return newBoundaryWorker("task overdue", func(ctx context.Context, args TaskOverdueArgs) error {
		return publisher.DeliverTaskOverdue(ctx, args.TaskID)
	}, log)
}
