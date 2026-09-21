package notificationsjob

import (
	"context"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// NewPaymentDueWorker builds the due boundary worker over the publisher and
// the process clock (issue #776): the worker wakes at one operation's due
// midnight — 00:00 of the operation date in the owner's timezone — and hands
// the (rule, date) and the wake-up instant to the publisher's delivery-time
// resolution: the operation is reloaded at wake-up — paid, cancelled, the
// rule edited or deleted, the property archived, or the day already rolled
// over finishes the job without publishing; a live one publishes «Оплатите
// платёж» through the pipeline (the dedup key keeps the sweep's row and the
// job's row to one).
func NewPaymentDueWorker(
	publisher application.PaymentBoundaryDeliverer, clk clock.Clock, log *slog.Logger,
) *boundaryWorker[PaymentDueArgs] {
	return newBoundaryWorker("payment due", func(ctx context.Context, args PaymentDueArgs) error {
		return publisher.DeliverPaymentDue(ctx, args.PaymentID, args.DueDate, clk.Now())
	}, log)
}

// NewPaymentOverdueWorker builds the overdue boundary worker over the
// publisher and the process clock (issue #776): the worker wakes at one
// operation's overdue midnight — 00:00 of the day after the operation date
// in the owner's timezone — and hands the (rule, date) and the wake-up
// instant to the publisher's delivery-time resolution with the same shape
// the due worker runs.
func NewPaymentOverdueWorker(
	publisher application.PaymentBoundaryDeliverer, clk clock.Clock, log *slog.Logger,
) *boundaryWorker[PaymentOverdueArgs] {
	return newBoundaryWorker("payment overdue", func(ctx context.Context, args PaymentOverdueArgs) error {
		return publisher.DeliverPaymentOverdue(ctx, args.PaymentID, args.DueDate, clk.Now())
	}, log)
}
