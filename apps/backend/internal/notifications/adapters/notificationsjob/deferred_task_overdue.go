package notificationsjob

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.TaskOverdueDeliverer = (*DeferredTaskOverdueDeliverer)(nil)

// DeferredTaskOverdueDeliverer bridges the wiring cycle (issue #750): the
// River workers register before the client exists, while the tasks
// publisher — the boundary worker's real deliverer — is built after it,
// because its scheduled leg books the jobs through the same client. The
// composition root binds the publisher once, strictly before the workers
// phase starts the client; the worker reads the binding per job.
type DeferredTaskOverdueDeliverer struct {
	deliverer application.TaskOverdueDeliverer
}

// Bind wires the real deliverer — called once from the composition root
// before the workers phase.
func (d *DeferredTaskOverdueDeliverer) Bind(deliverer application.TaskOverdueDeliverer) {
	d.deliverer = deliverer
}

// DeliverTaskOverdue delegates to the bound publisher.
func (d *DeferredTaskOverdueDeliverer) DeliverTaskOverdue(ctx context.Context, taskID uuid.UUID) error {
	if d.deliverer == nil {
		return errors.New("notifications task overdue: deliverer is not bound")
	}
	return d.deliverer.DeliverTaskOverdue(ctx, taskID)
}
