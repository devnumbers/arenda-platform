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

// fakeTaskOverdueDeliverer is the publisher stub: it records the task ids the
// worker hands over and can fail on demand.
type fakeTaskOverdueDeliverer struct {
	delivered []uuid.UUID
	err       error
}

func (f *fakeTaskOverdueDeliverer) DeliverTaskOverdue(ctx context.Context, taskID uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.delivered = append(f.delivered, taskID)
	return nil
}

// The due-minute worker hands the job's task id to the publisher — or fails
// the job, so River's retry ladder applies.
func TestTaskOverdueWorkerWork(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	taskID := uuid.Must(uuid.NewV7())

	deliverer := &fakeTaskOverdueDeliverer{}
	worker := NewTaskOverdueWorker(deliverer, nil)
	job := &river.Job[TaskOverdueArgs]{Args: TaskOverdueArgs{TaskID: taskID}}
	require.NoError(t, worker.Work(ctx, job))
	assert.Equal(t, []uuid.UUID{taskID}, deliverer.delivered)

	broken := &fakeTaskOverdueDeliverer{err: errors.New("feed write failed")}
	worker = NewTaskOverdueWorker(broken, nil)
	assert.Error(t, worker.Work(ctx, job))
}

// The scheduler is the application port over the River client: a repeat ask
// for the same task answers the standing job (unique in all non-terminal
// states) — the hourly scan re-asks freely and one job per task stands.
//
// This test runs against a real Postgres via TEST_DATABASE_URL and is
// skipped when it is unset (the notifications integration convention): the
// River client is left unstarted — inserts work without the fetchers, the
// river_job rows are the assertions.
func TestTaskOverdueSchedulerScheduleIsIdempotent(t *testing.T) {
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
	scheduler := NewTaskOverdueScheduler(client)

	taskID := uuid.Must(uuid.NewV7())
	dueAt := time.Date(2026, 9, 20, 15, 13, 0, 0, time.UTC)
	require.NoError(t, scheduler.ScheduleTaskOverdue(ctx, taskID, dueAt))
	require.NoError(t, scheduler.ScheduleTaskOverdue(ctx, taskID, dueAt), "the repeat ask re-books nothing")

	// The queue holds exactly one job for the task, scheduled at the term's
	// instant.
	booked := listTaskOverdueJobs(t, pool, taskID)
	require.Len(t, booked, 1)
	assert.Equal(t, QueueTasks, booked[0].queue)
	assert.True(t, booked[0].scheduledAt.Equal(dueAt), "the job wakes at the term's minute")

	// A different task books its own job.
	other := uuid.Must(uuid.NewV7())
	require.NoError(t, scheduler.ScheduleTaskOverdue(ctx, other, dueAt))
	assert.Len(t, listTaskOverdueJobs(t, pool, other), 1)
}

// taskOverdueJobRow is the river_job shape the scheduler test asserts on.
type taskOverdueJobRow struct {
	queue       string
	scheduledAt time.Time
}

// listTaskOverdueJobs lists the queue rows carrying one task id — the shared
// river_job table outlives a test run, so the query is scoped to the rows
// this test created.
func listTaskOverdueJobs(t *testing.T, pool *pgxpool.Pool, taskID uuid.UUID) []taskOverdueJobRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT queue, scheduled_at
		FROM river_job
		WHERE kind = $1 AND (args->>'task_id')::uuid = $2
		ORDER BY id`, TaskOverdueArgs{}.Kind(), taskID)
	require.NoError(t, err)
	defer rows.Close()

	var jobs []taskOverdueJobRow
	for rows.Next() {
		var j taskOverdueJobRow
		require.NoError(t, rows.Scan(&j.queue, &j.scheduledAt))
		jobs = append(jobs, j)
	}
	require.NoError(t, rows.Err())
	return jobs
}
