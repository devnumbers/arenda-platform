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
var _ application.RentalBoundaryScheduler = (*RentalBoundaryScheduler)(nil)

// RentalBoundaryScheduler books a rental's completed-boundary job on the
// River client (issue #777): the job wakes at 00:00 of the day after the
// rental's planned end in the owner's timezone. Unique by (rental id,
// planned end) in every non-terminal state, so the hourly scan's repeated
// asks are idempotent — a duplicate returns the standing job, nothing is
// enqueued twice. A completed or extended rental's job wakes and finds
// nothing — a no-op by the worker's delivery-time resolution; the extended
// rental's new boundary books its own job.
type RentalBoundaryScheduler struct {
	client *river.Client[pgx.Tx]
}

// NewRentalBoundaryScheduler builds the boundary scheduler over the delivery
// queue's River client.
func NewRentalBoundaryScheduler(client *river.Client[pgx.Tx]) *RentalBoundaryScheduler {
	return &RentalBoundaryScheduler{client: client}
}

// ScheduleRentalCompleted books the rental's «Аренда завершена» job at the
// given boundary instant.
func (s *RentalBoundaryScheduler) ScheduleRentalCompleted(ctx context.Context, rentalID uuid.UUID, plannedEnd, fireAt time.Time) error {
	_, err := s.client.Insert(ctx, RentalCompletedArgs{RentalID: rentalID, PlannedEndDate: plannedEnd}, &river.InsertOpts{
		Queue:       QueueRentals,
		ScheduledAt: fireAt,
		// The same budget every boundary job carries: the publication is
		// one feed write.
		MaxAttempts: boundaryJobMaxAttempts,
		UniqueOpts:  jobUniqueOpts,
	})
	if err != nil {
		return err
	}
	return nil
}
