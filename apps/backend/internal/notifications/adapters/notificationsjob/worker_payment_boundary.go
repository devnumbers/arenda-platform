package notificationsjob

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/riverqueue/river"
)

// PaymentDueWorker wakes at one operation's due midnight — 00:00 of the
// operation date in the owner's timezone (issue #776) — and hands the
// (rule, date) and the wake-up instant to the publisher's delivery-time
// resolution: the operation is reloaded at wake-up — paid, cancelled, the
// rule edited or deleted, the property archived, or the day already rolled
// over finishes the job without publishing; a live one publishes «Оплатите
// платёж» through the pipeline (the dedup key keeps the sweep's row and the
// job's row to one).
type PaymentDueWorker struct {
	river.WorkerDefaults[PaymentDueArgs]
	publisher application.PaymentBoundaryDeliverer
	clock     clock.Clock
	log       *slog.Logger
}

// NewPaymentDueWorker builds the due boundary worker over the publisher and
// the process clock.
func NewPaymentDueWorker(publisher application.PaymentBoundaryDeliverer, clk clock.Clock, log *slog.Logger) *PaymentDueWorker {
	if log == nil {
		log = slog.Default()
	}
	return &PaymentDueWorker{publisher: publisher, clock: clk, log: log}
}

// Work publishes the operation's due notification — or nothing, when the
// reload answers no live operation.
func (w *PaymentDueWorker) Work(ctx context.Context, job *river.Job[PaymentDueArgs]) error {
	if err := w.publisher.DeliverPaymentDue(ctx, job.Args.PaymentID, job.Args.DueDate, w.clock.Now()); err != nil {
		return fmt.Errorf("deliver payment due %s/%s: %w", job.Args.PaymentID, job.Args.DueDate.Format("2006-01-02"), err)
	}
	w.log.DebugContext(ctx, "payment due job finished",
		slog.String("payment_id", job.Args.PaymentID.String()))
	return nil
}

// PaymentOverdueWorker wakes at one operation's overdue midnight — 00:00 of
// the day after the operation date in the owner's timezone (issue #776) —
// and hands the (rule, date) and the wake-up instant to the publisher's
// delivery-time resolution with the same shape the due worker runs.
type PaymentOverdueWorker struct {
	river.WorkerDefaults[PaymentOverdueArgs]
	publisher application.PaymentBoundaryDeliverer
	clock     clock.Clock
	log       *slog.Logger
}

// NewPaymentOverdueWorker builds the overdue boundary worker over the
// publisher and the process clock.
func NewPaymentOverdueWorker(publisher application.PaymentBoundaryDeliverer, clk clock.Clock, log *slog.Logger) *PaymentOverdueWorker {
	if log == nil {
		log = slog.Default()
	}
	return &PaymentOverdueWorker{publisher: publisher, clock: clk, log: log}
}

// Work publishes the operation's overdue notification — or nothing, when
// the reload answers no live operation.
func (w *PaymentOverdueWorker) Work(ctx context.Context, job *river.Job[PaymentOverdueArgs]) error {
	if err := w.publisher.DeliverPaymentOverdue(ctx, job.Args.PaymentID, job.Args.DueDate, w.clock.Now()); err != nil {
		return fmt.Errorf("deliver payment overdue %s/%s: %w", job.Args.PaymentID, job.Args.DueDate.Format("2006-01-02"), err)
	}
	w.log.DebugContext(ctx, "payment overdue job finished",
		slog.String("payment_id", job.Args.PaymentID.String()))
	return nil
}
