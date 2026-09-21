package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/notificationsjob"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	platformpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests run against a real Postgres via TEST_DATABASE_URL (the shared
// harness of the notifications integration tests). The River queue owns its
// own schema chain: setupRiverSchema applies it idempotently (river_migration
// tracks the applied versions), so the shared database also carries the
// river_job table the pipeline writes.

// setupRiverSchema applies the River migrations through rivermigrate — the
// same call the runtime migrate step makes.
func setupRiverSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	require.NoError(t, err)
	_, err = migrator.Migrate(context.Background(), rivermigrate.DirectionUp, nil)
	require.NoError(t, err)
}

// pipelineTitle and pipelineBody are the publication texts shared by the
// pipeline integration tests.
const (
	pipelineTitle = "Оплатите платёж"
	pipelineBody  = "Платёж «Аренда» по объекту «Объект»: 2000000. Срок оплаты: 1 октября"
)

// riverJobRow is the shape the pipeline asserts on: the queue's row for one
// delivery job.
type riverJobRow struct {
	Kind string
	Args string
}

// listRiverJobsFor lists the queue rows carrying one of the given
// notification ids — the shared river_job table outlives a test run, so the
// query is scoped to the rows this test created.
func listRiverJobsFor(t *testing.T, pool *pgxpool.Pool, notificationIDs []uuid.UUID) []riverJobRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT kind, args::text
		FROM river_job
		WHERE (args->>'notification_id')::uuid = ANY($1)
		ORDER BY id`, notificationIDs)
	require.NoError(t, err)
	defer rows.Close()

	var jobs []riverJobRow
	for rows.Next() {
		var j riverJobRow
		require.NoError(t, rows.Scan(&j.Kind, &j.Args))
		jobs = append(jobs, j)
	}
	require.NoError(t, rows.Err())
	return jobs
}

// setupPipeline wires the real publisher over the real feed repository, the
// real River queue (unstarted: inserts work without the fetchers) and a real
// UoW — the full publication path of #740.
func setupPipeline(t *testing.T) (*pgxpool.Pool, *application.Publisher) {
	t.Helper()
	pool := setupPushDB(t)
	setupRiverSchema(t, pool)

	feedRepo := NewNotificationRepository(pool)
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	require.NoError(t, err)
	queue := notificationsjob.NewRiverQueue(client, true, 8, 8)
	publisher := application.NewPublisher(feedRepo, queue, nil, platformpostgres.NewUoW(pool, nil), nil)
	return pool, publisher
}

func TestNotificationRepository_GetByIDRoundTripAndMissing(t *testing.T) {
	t.Parallel()

	_, repo, userID := setupFeedDB(t)
	ctx := context.Background()
	n := feedNotification(t, userID, "payment_due:getbyid:2026-10-01")

	inserted, err := repo.Insert(ctx, n)
	require.NoError(t, err)
	require.True(t, inserted)

	got, err := repo.GetByID(ctx, n.ID)
	require.NoError(t, err)
	assert.Equal(t, n.ID, got.ID)
	assert.Equal(t, n.UserID, got.UserID)
	assert.Equal(t, n.EventType, got.EventType)
	assert.Equal(t, n.Title, got.Title)
	assert.Equal(t, n.Payload, got.Payload)

	_, err = repo.GetByID(ctx, uuid.Must(uuid.NewV7()))
	assert.ErrorIs(t, err, application.ErrNotFound)
}

func TestNotificationRepository_WithTxSeesUncommittedRows(t *testing.T) {
	t.Parallel()

	pool, repo, userID := setupFeedDB(t)
	ctx := context.Background()
	n := feedNotification(t, userID, "payment_due:withtx:2026-10-01")

	// The publisher's shape: insert inside a rolled-back transaction — the
	// row must vanish with it. No deferred rollback: an assert that fails
	// mid-test leaves the tx to the pool's cleanup, the explicit rollback is
	// the behavior under test.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)

	txRepo, err := repo.WithTx(tx)
	require.NoError(t, err)
	inserted, err := txRepo.Insert(ctx, n)
	require.NoError(t, err)
	require.True(t, inserted)

	page, err := repo.ListPage(ctx, userID, false, nil, uuid.Nil, 0)
	require.NoError(t, err)
	assert.Empty(t, page, "an uncommitted feed row is invisible outside the tx")

	require.NoError(t, tx.Rollback(ctx))
}

func TestPublisherWithRiver_FansOutRowsAndJobs(t *testing.T) {
	t.Parallel()

	pool, publisher := setupPipeline(t)
	ctx := context.Background()

	r1, r2 := createPushTestUser(t, ctx, genpostgres.New(pool)), createPushTestUser(t, ctx, genpostgres.New(pool))
	pub := application.Publication{
		EventType:  domain.EventPaymentDue,
		DedupKey:   domain.DedupKey("payment_due:pipe-test:" + uuid.Must(uuid.NewV7()).String()),
		Title:      pipelineTitle,
		Body:       pipelineBody,
		Recipients: []uuid.UUID{r1, r2},
	}

	require.NoError(t, publisher.Publish(ctx, pub))

	feedRepo := NewNotificationRepository(pool)
	rowIDs := make([]uuid.UUID, 0, 2)
	for _, recipient := range []uuid.UUID{r1, r2} {
		page, err := feedRepo.ListPage(ctx, recipient, false, nil, uuid.Nil, 0)
		require.NoError(t, err)
		require.Len(t, page, 1, "one feed row per recipient")
		rowIDs = append(rowIDs, page[0].ID)
	}

	jobs := listRiverJobsFor(t, pool, rowIDs)
	require.Len(t, jobs, 4, "email + push job per created row")
	kinds := map[string]int{}
	ids := map[string]bool{}
	for _, j := range jobs {
		kinds[j.Kind]++
		var args struct {
			NotificationID uuid.UUID `json:"notification_id"`
		}
		require.NoError(t, json.Unmarshal([]byte(j.Args), &args))
		ids[args.NotificationID.String()] = true
	}
	assert.Equal(t, 2, kinds["notifications:deliver_email"])
	assert.Equal(t, 2, kinds["notifications:deliver_push"])
	assert.Len(t, ids, 2, "both channel jobs of a row carry that row's id")
}

func TestPublisherWithRiver_RepeatPublicationDuplicatesNothing(t *testing.T) {
	t.Parallel()

	pool, publisher := setupPipeline(t)
	ctx := context.Background()

	recipient := createPushTestUser(t, ctx, genpostgres.New(pool))
	dedup := domain.DedupKey("payment_due:pipe-dedup:" + uuid.Must(uuid.NewV7()).String())

	pub := application.Publication{
		EventType:  domain.EventPaymentDue,
		DedupKey:   dedup,
		Title:      pipelineTitle,
		Body:       pipelineBody,
		Recipients: []uuid.UUID{recipient},
	}
	require.NoError(t, publisher.Publish(ctx, pub))
	require.NoError(t, publisher.Publish(ctx, pub), "a repeat publication is a no-op, not an error")

	feedRepo := NewNotificationRepository(pool)
	page, err := feedRepo.ListPage(ctx, recipient, false, nil, uuid.Nil, 0)
	require.NoError(t, err)
	require.Len(t, page, 1, "the (recipient, dedup key) unique index held")

	assert.Len(t, listRiverJobsFor(t, pool, []uuid.UUID{page[0].ID}), 2, "the repeated rows enqueued no second deliveries")
}

func TestPublisherWithRiver_PushDisabledEnqueuesEmailOnly(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	setupRiverSchema(t, pool)
	ctx := context.Background()

	recipient := createPushTestUser(t, ctx, genpostgres.New(pool))
	feedRepo := NewNotificationRepository(pool)
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	require.NoError(t, err)
	queue := notificationsjob.NewRiverQueue(client, false, 8, 8)
	publisher := application.NewPublisher(feedRepo, queue, nil, platformpostgres.NewUoW(pool, nil), nil)

	require.NoError(t, publisher.Publish(ctx, application.Publication{
		EventType:  domain.EventPaymentDue,
		DedupKey:   domain.DedupKey("payment_due:pipe-nopush:" + uuid.Must(uuid.NewV7()).String()),
		Title:      pipelineTitle,
		Body:       pipelineBody,
		Recipients: []uuid.UUID{recipient},
	}))

	page, err := feedRepo.ListPage(ctx, recipient, false, nil, uuid.Nil, 0)
	require.NoError(t, err)
	require.Len(t, page, 1)
	jobs := listRiverJobsFor(t, pool, []uuid.UUID{page[0].ID})
	require.Len(t, jobs, 1)
	assert.Equal(t, "notifications:deliver_email", jobs[0].Kind, "without a push sender only the email leg is scheduled")
}
