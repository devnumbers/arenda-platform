package notificationsjob

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.PaymentBoundaryDeliverer = (*DeferredPaymentBoundaryDeliverer)(nil)

// DeferredPaymentBoundaryDeliverer is the payment boundary port over the
// shared bind-later seam (deferredBinding, issue #776).
type DeferredPaymentBoundaryDeliverer struct {
	deferredBinding[application.PaymentBoundaryDeliverer]
}

// DeliverPaymentDue delegates to the bound publisher.
func (d *DeferredPaymentBoundaryDeliverer) DeliverPaymentDue(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	inner, err := d.load()
	if err != nil {
		return err
	}
	return inner.DeliverPaymentDue(ctx, paymentID, date, now)
}

// DeliverPaymentOverdue delegates to the bound publisher.
func (d *DeferredPaymentBoundaryDeliverer) DeliverPaymentOverdue(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	inner, err := d.load()
	if err != nil {
		return err
	}
	return inner.DeliverPaymentOverdue(ctx, paymentID, date, now)
}

// DeliverPaymentReminder delegates to the bound publisher.
func (d *DeferredPaymentBoundaryDeliverer) DeliverPaymentReminder(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	inner, err := d.load()
	if err != nil {
		return err
	}
	return inner.DeliverPaymentReminder(ctx, paymentID, date, now)
}

// DeliverPaymentAutoPaid delegates to the bound publisher.
func (d *DeferredPaymentBoundaryDeliverer) DeliverPaymentAutoPaid(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	inner, err := d.load()
	if err != nil {
		return err
	}
	return inner.DeliverPaymentAutoPaid(ctx, paymentID, date, now)
}
