package notificationsjob

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.RentalBoundaryDeliverer = (*DeferredRentalBoundaryDeliverer)(nil)

// DeferredRentalBoundaryDeliverer bridges the wiring cycle (issue #777, the
// boundary canon of #776/#750): the River workers register before the client
// exists, while the rental publisher — the boundary worker's real deliverer
// — is built after it, because its booking leg schedules the jobs through
// the same client. The composition root binds the publisher once, strictly
// before the workers phase starts the client; the worker reads the binding
// per job.
type DeferredRentalBoundaryDeliverer struct {
	deliverer application.RentalBoundaryDeliverer
}

// Bind wires the real deliverer — called once from the composition root
// before the workers phase.
func (d *DeferredRentalBoundaryDeliverer) Bind(deliverer application.RentalBoundaryDeliverer) {
	d.deliverer = deliverer
}

// DeliverRentalCompleted delegates to the bound publisher.
func (d *DeferredRentalBoundaryDeliverer) DeliverRentalCompleted(ctx context.Context, rentalID uuid.UUID, plannedEnd, now time.Time) error {
	if d.deliverer == nil {
		return errors.New("notifications rental completed: deliverer is not bound")
	}
	return d.deliverer.DeliverRentalCompleted(ctx, rentalID, plannedEnd, now)
}
