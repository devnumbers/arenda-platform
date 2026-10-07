package riverqueue

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/riverqueue/river"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.ContactChangeNotifier = (*Queue)(nil)

// Queue schedules contact-change letters through the shared River client.
// The client binds late — identity builds before the delivery queue (the
// grace-events canon) — and every enqueue happens inside the caller's
// transaction (InsertTx), so a letter job commits together with the change
// it announces (решение #1207).
type Queue struct {
	mu     sync.RWMutex
	client *river.Client[pgx.Tx]
	opts   *river.InsertOpts
}

// NewQueue builds the unbound change-letter queue. Bind the River client
// before the workers phase starts, right after WireRiverQueue.
func NewQueue() *Queue {
	return &Queue{
		opts: &river.InsertOpts{
			Queue:       QueueContactChanged,
			MaxAttempts: contactChangedMaxAttempts,
		},
	}
}

// Bind attaches the shared River client. Called once from the composition
// root after WireRiverQueue builds it; a ScheduleChanged before the bind is
// a wiring mistake and fails loudly.
func (q *Queue) Bind(client *river.Client[pgx.Tx]) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.client = client
}

func (q *Queue) bound() (*river.Client[pgx.Tx], error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.client == nil {
		return nil, errors.New("contact-change queue: river client is not bound yet (wiring: bind after WireRiverQueue)")
	}
	return q.client, nil
}

// WithTx binds the scheduling to the caller's transaction — the shape the
// change services receive through txStores inside runInTx.
func (q *Queue) WithTx(tx transaction.Tx) application.TransactionalContactChangeNotifier {
	return txQueue{queue: q, tx: tx}
}

// txQueue inserts the letter job into the bound transaction, mirroring the
// notifications queue's transactional enqueue.
type txQueue struct {
	queue *Queue
	tx    transaction.Tx
}

// ScheduleChanged inserts one letter job into the change's transaction.
func (t txQueue) ScheduleChanged(ctx context.Context, event application.ContactChangedEvent) error {
	client, err := t.queue.bound()
	if err != nil {
		return err
	}
	ptx, err := database.PgxTxOf(t.tx)
	if err != nil {
		return err
	}
	args := ContactChangedArgs{
		ChangeKind: string(event.Kind),
		UserID:     event.UserID,
		Recipient:  event.Recipient.String(),
		ChangedAt:  event.ChangedAt,
	}
	if _, err := client.InsertTx(ctx, ptx, args, t.queue.opts); err != nil {
		return fmt.Errorf("insert %s job: %w", args.Kind(), err)
	}
	return nil
}
