package notificationsjob

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.PaymentBoundaryDeliverer = (*DeferredPaymentBoundaryDeliverer)(nil)

// DeferredPaymentBoundaryDeliverer bridges the wiring cycle (issue #776,
// the due-minute canon of #750): the River workers register before the
// client exists, while the payments publisher — the boundary workers' real
// deliverer — is built after it, because its booking leg schedules the jobs
// through the same client. The composition root binds the publisher once,
// strictly before the workers phase starts the client; the workers read the
// binding per job.
type DeferredPaymentBoundaryDeliverer struct {
	deliverer application.PaymentBoundaryDeliverer
}

// Bind wires the real deliverer — called once from the composition root
// before the workers phase.
func (d *DeferredPaymentBoundaryDeliverer) Bind(deliverer application.PaymentBoundaryDeliverer) {
	d.deliverer = deliverer
}

// DeliverPaymentDue delegates to the bound publisher.
func (d *DeferredPaymentBoundaryDeliverer) DeliverPaymentDue(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	if d.deliverer == nil {
		return errors.New("notifications payment due: deliverer is not bound")
	}
	return d.deliverer.DeliverPaymentDue(ctx, paymentID, date, now)
}

// DeliverPaymentOverdue delegates to the bound publisher.
func (d *DeferredPaymentBoundaryDeliverer) DeliverPaymentOverdue(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	if d.deliverer == nil {
		return errors.New("notifications payment overdue: deliverer is not bound")
	}
	return d.deliverer.DeliverPaymentOverdue(ctx, paymentID, date, now)
}
