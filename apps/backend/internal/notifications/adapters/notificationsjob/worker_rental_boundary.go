package notificationsjob

import (
	"context"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// NewRentalCompletedWorker builds the rental boundary worker over the
// publisher and the process clock (issue #777): the worker wakes at one
// rental's completed boundary — 00:00 of the day after its planned end in
// the owner's timezone — and hands the (rental, planned end) and the wake-up
// instant to the publisher's delivery-time resolution: the rental is
// reloaded at wake-up — completed, extended (the planned end moved off the
// booked date), the property archived — finishes the job without publishing;
// a live one publishes «Аренда завершена» through the pipeline (the dedup
// key keeps the sweep's row and the job's row to one).
func NewRentalCompletedWorker(
	publisher application.RentalBoundaryDeliverer, clk clock.Clock, log *slog.Logger,
) *boundaryWorker[RentalCompletedArgs] {
	return newBoundaryWorker("rental completed", func(ctx context.Context, args RentalCompletedArgs) error {
		return publisher.DeliverRentalCompleted(ctx, args.RentalID, args.PlannedEndDate, clk.Now())
	}, log)
}
