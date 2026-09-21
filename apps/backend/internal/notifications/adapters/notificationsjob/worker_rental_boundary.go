package notificationsjob

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/riverqueue/river"
)

// RentalCompletedWorker wakes at one rental's completed boundary — 00:00 of
// the day after its planned end in the owner's timezone (issue #777) — and
// hands the (rental, planned end) and the wake-up instant to the publisher's
// delivery-time resolution: the rental is reloaded at wake-up — completed,
// extended (the planned end moved off the booked date), the property
// archived — finishes the job without publishing; a live one publishes
// «Аренда завершена» through the pipeline (the dedup key keeps the sweep's
// row and the job's row to one).
type RentalCompletedWorker struct {
	river.WorkerDefaults[RentalCompletedArgs]
	publisher application.RentalBoundaryDeliverer
	clock     clock.Clock
	log       *slog.Logger
}

// NewRentalCompletedWorker builds the rental boundary worker over the
// publisher and the process clock.
func NewRentalCompletedWorker(publisher application.RentalBoundaryDeliverer, clk clock.Clock, log *slog.Logger) *RentalCompletedWorker {
	if log == nil {
		log = slog.Default()
	}
	return &RentalCompletedWorker{publisher: publisher, clock: clk, log: log}
}

// Work publishes the rental's «Аренда завершена» — or nothing, when the
// reload answers no live rental.
func (w *RentalCompletedWorker) Work(ctx context.Context, job *river.Job[RentalCompletedArgs]) error {
	if err := w.publisher.DeliverRentalCompleted(ctx, job.Args.RentalID, job.Args.PlannedEndDate, w.clock.Now()); err != nil {
		return fmt.Errorf("deliver rental completed %s/%s: %w",
			job.Args.RentalID, job.Args.PlannedEndDate.Format("2006-01-02"), err)
	}
	w.log.DebugContext(ctx, "rental completed job finished",
		slog.String("rental_id", job.Args.RentalID.String()))
	return nil
}
