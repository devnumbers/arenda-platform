package notificationsjob

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.TaskOverdueDeliverer = (*DeferredTaskOverdueDeliverer)(nil)

// DeferredTaskOverdueDeliverer is the task overdue port over the shared
// bind-later seam (deferredBinding, issue #750).
type DeferredTaskOverdueDeliverer struct {
	deferredBinding[application.TaskOverdueDeliverer]
}

// DeliverTaskOverdue delegates to the bound publisher.
func (d *DeferredTaskOverdueDeliverer) DeliverTaskOverdue(ctx context.Context, taskID uuid.UUID) error {
	inner, err := d.load()
	if err != nil {
		return err
	}
	return inner.DeliverTaskOverdue(ctx, taskID)
}
