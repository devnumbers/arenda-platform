// Package taskschedule adapts the tasks context's scheduling seam (issue
// #775) to the notifications context: the rule create/edit flows hand their
// standing tasks' ids over strictly after their transaction commits, and
// the seam plans each one through the tasks publisher — the due-minute job
// at the term's instant, or the immediate publication of a task born
// overdue. The adapter owns the process clock the term-vs-now decision runs
// on; the composition root wires it into the tasks module once the delivery
// queue exists (the tasks module builds earlier than the queue).
package taskschedule

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	tasksapp "github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ tasksapp.MaterializedTaskNotifier = (*Seam)(nil)

// Seam implements the tasks context's MaterializedTaskNotifier (issue #775)
// over the notifications tasks publisher.
type Seam struct {
	publisher *application.TasksPublisher
	clock     clock.Clock
}

// NewSeam builds the scheduling seam over the tasks publisher and the
// process clock.
func NewSeam(publisher *application.TasksPublisher, clk clock.Clock) *Seam {
	return &Seam{publisher: publisher, clock: clk}
}

// NotifyMaterializedTasks plans the handed-over tasks as of the clock's
// now: the future-vs-past decision is the seam's, the per-task liveness
// check and the booking or publication are the publisher's.
func (s *Seam) NotifyMaterializedTasks(ctx context.Context, taskIDs []uuid.UUID) error {
	return s.publisher.NotifyMaterializedTasks(ctx, taskIDs, s.clock.Now())
}
