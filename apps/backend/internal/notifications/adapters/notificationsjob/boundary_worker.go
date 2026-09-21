package notificationsjob

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/riverqueue/river"
)

// boundaryWorker is the shared skeleton of the boundary workers: wake at the
// boundary instant, hand the job args to the deliverer's delivery-time
// resolution, wrap the failure so River's retry ladder applies, log the
// finish. The per-kind differences — which deliverer method runs with which
// args and whose clock instant — live in the constructor's deliver closure.
type boundaryWorker[A river.JobArgs] struct {
	river.WorkerDefaults[A]
	what    string
	deliver func(ctx context.Context, args A) error
	log     *slog.Logger
}

// newBoundaryWorker builds the skeleton; a nil logger falls back to the
// process default.
func newBoundaryWorker[A river.JobArgs](
	what string, deliver func(ctx context.Context, args A) error, log *slog.Logger,
) *boundaryWorker[A] {
	if log == nil {
		log = slog.Default()
	}
	return &boundaryWorker[A]{what: what, deliver: deliver, log: log}
}

// Work delivers the boundary event — or nothing, when the deliverer's
// delivery-time resolution answers that the subject is no longer live.
func (w *boundaryWorker[A]) Work(ctx context.Context, job *river.Job[A]) error {
	if err := w.deliver(ctx, job.Args); err != nil {
		return fmt.Errorf("deliver %s %v: %w", w.what, job.Args, err)
	}
	w.log.DebugContext(ctx, "boundary job finished", slog.String("what", w.what), slog.Any("args", job.Args))
	return nil
}

// deferredBinding is the shared bind-later seam of the boundary deliverers:
// the River workers register before the client exists, while the publishers
// — the workers' real deliverers — are built after it, because their booking
// legs schedule jobs through the same client. The composition root binds the
// publisher once, strictly before the workers phase starts the client; the
// workers read the binding per job. The port-facing thin types embed it and
// delegate their methods through load.
type deferredBinding[T any] struct {
	deliverer T
	bound     bool
}

// Bind wires the real deliverer — called once from the composition root
// before the workers phase.
func (d *deferredBinding[T]) Bind(deliverer T) {
	d.deliverer, d.bound = deliverer, true
}

// load answers the bound deliverer — or an error when the composition root
// did not bind one: an unbound seam refuses to run rather than silently
// dropping the job.
func (d *deferredBinding[T]) load() (T, error) {
	if !d.bound {
		var zero T
		return zero, errors.New("notifications: deliverer is not bound")
	}
	return d.deliverer, nil
}
