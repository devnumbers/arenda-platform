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

// fakeRentalBoundaryDeliverer is the publisher stub: it records the (rental,
// planned end, now) triples the worker hands over and can fail on demand.
type fakeRentalBoundaryDeliverer struct {
	calls    []rentalBoundaryCall
	errOnDel error
}

type rentalBoundaryCall struct {
	rentalID   uuid.UUID
	plannedEnd time.Time
	now        time.Time
}

func (f *fakeRentalBoundaryDeliverer) DeliverRentalCompleted(ctx context.Context, rentalID uuid.UUID, plannedEnd, now time.Time) error {
	if f.errOnDel != nil {
		return f.errOnDel
	}
	f.calls = append(f.calls, rentalBoundaryCall{rentalID, plannedEnd, now})
	return nil
}

// The rental boundary worker hands the job's (rental, planned end) and the
// clock's wake-up instant to the publisher — or fails the job, so River's
// retry ladder applies.
func TestRentalCompletedWorkerWork(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rentalID := uuid.Must(uuid.NewV7())
	plannedEnd := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 9, 18, 21, 0, 5, 0, time.UTC)

	deliverer := &fakeRentalBoundaryDeliverer{}
	job := &river.Job[RentalCompletedArgs]{Args: RentalCompletedArgs{RentalID: rentalID, PlannedEndDate: plannedEnd}}
	require.NoError(t, NewRentalCompletedWorker(deliverer, stubClock{now}, nil).Work(ctx, job))
	require.Len(t, deliverer.calls, 1)
	assert.Equal(t, rentalID, deliverer.calls[0].rentalID)
	assert.Equal(t, plannedEnd, deliverer.calls[0].plannedEnd)
	assert.Equal(t, now, deliverer.calls[0].now, "the reload runs as of the wake-up instant")

	broken := &fakeRentalBoundaryDeliverer{errOnDel: errors.New("feed write failed")}
	assert.Error(t, NewRentalCompletedWorker(broken, stubClock{now}, nil).Work(ctx, job))
}

// The unbound deferred deliverer refuses to run — the composition root must
// bind the publisher before the workers phase starts the client.
func TestDeferredRentalBoundaryDelivererRequiresBinding(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	bridge := &DeferredRentalBoundaryDeliverer{}
	require.Error(t, bridge.DeliverRentalCompleted(ctx, uuid.Must(uuid.NewV7()), time.Now(), time.Now()))

	bridge.Bind(&fakeRentalBoundaryDeliverer{})
	rentalID := uuid.Must(uuid.NewV7())
	plannedEnd := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	assert.NoError(t, bridge.DeliverRentalCompleted(ctx, rentalID, plannedEnd, plannedEnd))
}

// The scheduler is the application port over the River client: a repeat ask
// for the same (rental, planned end) answers the standing job (unique in all
// non-terminal states) — the hourly scan re-asks freely and one job per
// boundary stands.
//
// This test runs against a real Postgres via TEST_DATABASE_URL and is
// skipped when it is unset (the notifications integration convention): the
// River client is left unstarted — inserts work without the fetchers, the
// river_job rows are the assertions.
func TestRentalBoundarySchedulerScheduleIsIdempotent(t *testing.T) {
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
	scheduler := NewRentalBoundaryScheduler(client)

	rentalID := uuid.Must(uuid.NewV7())
	plannedEnd := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	fireAt := time.Date(2026, 9, 18, 21, 0, 0, 0, time.UTC)

	require.NoError(t, scheduler.ScheduleRentalCompleted(ctx, rentalID, plannedEnd, fireAt))
	require.NoError(t, scheduler.ScheduleRentalCompleted(ctx, rentalID, plannedEnd, fireAt),
		"the repeat ask re-books nothing")

	// One job, on the rentals queue at the boundary instant.
	jobs := listRentalBoundaryJobs(t, pool, RentalCompletedArgs{}.Kind(), rentalID)
	require.Len(t, jobs, 1)
	assert.Equal(t, QueueRentals, jobs[0].queue)
	assert.True(t, jobs[0].scheduledAt.Equal(fireAt), "the job wakes at 00:00 of the day after the planned end")

	// A moved planned end — the extension — is a different job: it coexists
	// with the first, its own boundary instant.
	extended := plannedEnd.AddDate(0, 1, 0)
	require.NoError(t, scheduler.ScheduleRentalCompleted(ctx, rentalID, extended, fireAt.AddDate(0, 1, 0)))
	assert.Len(t, listRentalBoundaryJobs(t, pool, RentalCompletedArgs{}.Kind(), rentalID), 2)
}

// rentalBoundaryJobRow is the river_job shape the scheduler test asserts on.
type rentalBoundaryJobRow struct {
	queue       string
	scheduledAt time.Time
}

// listRentalBoundaryJobs lists the queue rows carrying one rental id of one
// kind — the shared river_job table outlives a test run, so the query is
// scoped to the rows this test created.
func listRentalBoundaryJobs(t *testing.T, pool *pgxpool.Pool, kind string, rentalID uuid.UUID) []rentalBoundaryJobRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT queue, scheduled_at
		FROM river_job
		WHERE kind = $1 AND (args->>'rental_id')::uuid = $2
		ORDER BY id`, kind, rentalID)
	require.NoError(t, err)
	defer rows.Close()

	var jobs []rentalBoundaryJobRow
	for rows.Next() {
		var j rentalBoundaryJobRow
		require.NoError(t, rows.Scan(&j.queue, &j.scheduledAt))
		jobs = append(jobs, j)
	}
	require.NoError(t, rows.Err())
	return jobs
}
