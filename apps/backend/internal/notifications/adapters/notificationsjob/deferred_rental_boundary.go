package notificationsjob

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.RentalBoundaryDeliverer = (*DeferredRentalBoundaryDeliverer)(nil)

// DeferredRentalBoundaryDeliverer is the rental boundary port over the
// shared bind-later seam (deferredBinding, issue #777).
type DeferredRentalBoundaryDeliverer struct {
	deferredBinding[application.RentalBoundaryDeliverer]
}

// DeliverRentalCompleted delegates to the bound publisher.
func (d *DeferredRentalBoundaryDeliverer) DeliverRentalCompleted(ctx context.Context, rentalID uuid.UUID, plannedEnd, now time.Time) error {
	inner, err := d.load()
	if err != nil {
		return err
	}
	return inner.DeliverRentalCompleted(ctx, rentalID, plannedEnd, now)
}
