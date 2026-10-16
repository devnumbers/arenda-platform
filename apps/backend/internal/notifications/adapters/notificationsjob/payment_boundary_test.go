package notificationsjob

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePaymentBoundaryDeliverer is the publisher stub: it records the (rule,
// date, now) triples the workers hand over and can fail on demand.
type fakePaymentBoundaryDeliverer struct {
	due        []paymentBoundaryCall
	overdue    []paymentBoundaryCall
	reminder   []paymentBoundaryCall
	autoPaid   []paymentBoundaryCall
	errForDue  error
	errForAuto error
}

type paymentBoundaryCall struct {
	paymentID uuid.UUID
	date      time.Time
	now       time.Time
}

func (f *fakePaymentBoundaryDeliverer) DeliverPaymentDue(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	if f.errForDue != nil {
		return f.errForDue
	}
	f.due = append(f.due, paymentBoundaryCall{paymentID, date, now})
	return nil
}

func (f *fakePaymentBoundaryDeliverer) DeliverPaymentOverdue(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	f.overdue = append(f.overdue, paymentBoundaryCall{paymentID, date, now})
	return nil
}

func (f *fakePaymentBoundaryDeliverer) DeliverPaymentReminder(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	f.reminder = append(f.reminder, paymentBoundaryCall{paymentID, date, now})
	return nil
}

func (f *fakePaymentBoundaryDeliverer) DeliverPaymentAutoPaid(ctx context.Context, paymentID uuid.UUID, date, now time.Time) error {
	if f.errForAuto != nil {
		return f.errForAuto
	}
	f.autoPaid = append(f.autoPaid, paymentBoundaryCall{paymentID, date, now})
	return nil
}

// stubClock answers the instant the test fixes — the workers read the
// wake-up instant from it and hand it to the publisher's reload.
type stubClock struct{ now time.Time }

func (c stubClock) Now() time.Time { return c.now }

// The boundary workers hand the job's (rule, date) and the clock's wake-up
// instant to the publisher — or fail the job, so River's retry ladder
// applies.
func TestPaymentBoundaryWorkersWork(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	paymentID := uuid.Must(uuid.NewV7())
	date := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 19, 21, 0, 5, 0, time.UTC)

	deliverer := &fakePaymentBoundaryDeliverer{}
	dueJob := &river.Job[PaymentDueArgs]{Args: PaymentDueArgs{PaymentID: paymentID, DueDate: date}}
	require.NoError(t, NewPaymentDueWorker(deliverer, stubClock{now}, nil).Work(ctx, dueJob))
	require.Len(t, deliverer.due, 1)
	assert.Equal(t, paymentID, deliverer.due[0].paymentID)
	assert.Equal(t, date, deliverer.due[0].date)
	assert.Equal(t, now, deliverer.due[0].now, "the reload runs as of the wake-up instant")

	overdueJob := &river.Job[PaymentOverdueArgs]{Args: PaymentOverdueArgs{PaymentID: paymentID, DueDate: date}}
	require.NoError(t, NewPaymentOverdueWorker(deliverer, stubClock{now}, nil).Work(ctx, overdueJob))
	require.Len(t, deliverer.overdue, 1)
	assert.Equal(t, date, deliverer.overdue[0].date)

	reminderJob := &river.Job[PaymentReminderArgs]{Args: PaymentReminderArgs{PaymentID: paymentID, DueDate: date}}
	require.NoError(t, NewPaymentReminderWorker(deliverer, stubClock{now}, nil).Work(ctx, reminderJob))
	require.Len(t, deliverer.reminder, 1)
	assert.Equal(t, paymentID, deliverer.reminder[0].paymentID)
	assert.Equal(t, now, deliverer.reminder[0].now)

	// Автоплатёжная джоба (#1169): та же форма (правило, дата, мгновение).
	autoJob := &river.Job[PaymentAutoPaidArgs]{Args: PaymentAutoPaidArgs{PaymentID: paymentID, DueDate: date}}
	require.NoError(t, NewPaymentAutoPaidWorker(deliverer, stubClock{now}, nil).Work(ctx, autoJob))
	require.Len(t, deliverer.autoPaid, 1)
	assert.Equal(t, paymentID, deliverer.autoPaid[0].paymentID)
	assert.Equal(t, now, deliverer.autoPaid[0].now)

	broken := &fakePaymentBoundaryDeliverer{errForDue: errors.New("feed write failed")}
	assert.Error(t, NewPaymentDueWorker(broken, stubClock{now}, nil).Work(ctx, dueJob))
}

// The unbound deferred deliverer refuses to run — the composition root must
// bind the publisher before the workers phase starts the client.
func TestDeferredPaymentBoundaryDelivererRequiresBinding(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	bridge := &DeferredPaymentBoundaryDeliverer{}
	require.Error(t, bridge.DeliverPaymentDue(ctx, uuid.Must(uuid.NewV7()), time.Now(), time.Now()))
	require.Error(t, bridge.DeliverPaymentOverdue(ctx, uuid.Must(uuid.NewV7()), time.Now(), time.Now()))
	require.Error(t, bridge.DeliverPaymentReminder(ctx, uuid.Must(uuid.NewV7()), time.Now(), time.Now()))
	require.Error(t, bridge.DeliverPaymentAutoPaid(ctx, uuid.Must(uuid.NewV7()), time.Now(), time.Now()))

	bridge.Bind(&fakePaymentBoundaryDeliverer{})
	paymentID := uuid.Must(uuid.NewV7())
	date := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	assert.NoError(t, bridge.DeliverPaymentDue(ctx, paymentID, date, date))
	assert.NoError(t, bridge.DeliverPaymentOverdue(ctx, paymentID, date, date))
	assert.NoError(t, bridge.DeliverPaymentReminder(ctx, paymentID, date, date))
	assert.NoError(t, bridge.DeliverPaymentAutoPaid(ctx, paymentID, date, date))
}

// The scheduler is the application port over the River client: a repeat ask
// for the same (leg, rule, date) answers the standing job (unique in all
// non-terminal states) — the hourly scan re-asks freely and one job per
// boundary stands. The three legs book different kinds, so a rule's due,
// reminder and overdue jobs coexist.
//
// This test runs against a real Postgres via TEST_DATABASE_URL and is
// skipped when it is unset (the notifications integration convention): the
// River client is left unstarted — inserts work without the fetchers, the
// river_job rows are the assertions.
func TestPaymentBoundarySchedulerScheduleIsIdempotent(t *testing.T) {
	t.Parallel()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	cfg := database.DefaultPoolConfig()
	cfg.MinConns = 0
	cfg.MaxConns = 2
	pool, err := database.NewPoolWithConfig(ctx, databaseURL, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	require.NoError(t, err)
	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	require.NoError(t, err)

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	require.NoError(t, err)
	scheduler := NewPaymentBoundaryScheduler(client)

	paymentID := uuid.Must(uuid.NewV7())
	date := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	fireAt := time.Date(2026, 9, 19, 21, 0, 0, 0, time.UTC)
	require.NoError(t, scheduler.SchedulePaymentDue(ctx, paymentID, date, fireAt))
	require.NoError(t, scheduler.SchedulePaymentDue(ctx, paymentID, date, fireAt), "the repeat ask re-books nothing")
	require.NoError(t, scheduler.SchedulePaymentOverdue(ctx, paymentID, date, fireAt))
	require.NoError(t, scheduler.SchedulePaymentOverdue(ctx, paymentID, date, fireAt))
	require.NoError(t, scheduler.SchedulePaymentReminder(ctx, paymentID, date, fireAt))
	require.NoError(t, scheduler.SchedulePaymentReminder(ctx, paymentID, date, fireAt), "the repeat ask re-books nothing")
	require.NoError(t, scheduler.SchedulePaymentAutoPaid(ctx, paymentID, date, fireAt))
	require.NoError(t, scheduler.SchedulePaymentAutoPaid(ctx, paymentID, date, fireAt), "the repeat ask re-books nothing")

	// One job per leg, each on the payments queue at its boundary instant.
	due := listPaymentBoundaryJobs(t, pool, PaymentDueArgs{}.Kind(), paymentID)
	require.Len(t, due, 1)
	assert.Equal(t, QueuePayments, due[0].queue)
	assert.True(t, due[0].scheduledAt.Equal(fireAt), "the due job wakes at 00:00 of the operation date")
	overdue := listPaymentBoundaryJobs(t, pool, PaymentOverdueArgs{}.Kind(), paymentID)
	require.Len(t, overdue, 1)
	assert.Equal(t, QueuePayments, overdue[0].queue)
	assert.True(t, overdue[0].scheduledAt.Equal(fireAt))
	reminder := listPaymentBoundaryJobs(t, pool, PaymentReminderArgs{}.Kind(), paymentID)
	require.Len(t, reminder, 1)
	assert.Equal(t, QueuePayments, reminder[0].queue)
	assert.True(t, reminder[0].scheduledAt.Equal(fireAt))
	autoPaid := listPaymentBoundaryJobs(t, pool, PaymentAutoPaidArgs{}.Kind(), paymentID)
	require.Len(t, autoPaid, 1)
	assert.Equal(t, QueuePayments, autoPaid[0].queue)
	assert.True(t, autoPaid[0].scheduledAt.Equal(fireAt))

	// A different operation date books its own jobs.
	other := date.AddDate(0, 1, 0)
	require.NoError(t, scheduler.SchedulePaymentDue(ctx, paymentID, other, fireAt))
	assert.Len(t, listPaymentBoundaryJobs(t, pool, PaymentDueArgs{}.Kind(), paymentID), 2)
}

// paymentBoundaryJobRow is the river_job shape the scheduler test asserts on.
type paymentBoundaryJobRow struct {
	queue       string
	scheduledAt time.Time
}

// listPaymentBoundaryJobs lists the queue rows carrying one rule id of one
// kind — the shared river_job table outlives a test run, so the query is
// scoped to the rows this test created.
func listPaymentBoundaryJobs(t *testing.T, pool *pgxpool.Pool, kind string, paymentID uuid.UUID) []paymentBoundaryJobRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT queue, scheduled_at
		FROM river_job
		WHERE kind = $1 AND (args->>'payment_id')::uuid = $2
		ORDER BY id`, kind, paymentID)
	require.NoError(t, err)
	defer rows.Close()

	var jobs []paymentBoundaryJobRow
	for rows.Next() {
		var j paymentBoundaryJobRow
		require.NoError(t, rows.Scan(&j.queue, &j.scheduledAt))
		jobs = append(jobs, j)
	}
	require.NoError(t, rows.Err())
	return jobs
}
