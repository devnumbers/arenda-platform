package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertScanTask adds one task row: an optional property binding (nil = the
// task without a property, ADR 0052), an optional due time (the timed task;
// nil = date-only) and an optional completion date.
func insertScanTask(
	t *testing.T, pool *pgxpool.Pool, ownerID, propertyID, ruleID uuid.UUID,
	dueDate string, dueTime, completedDate *string,
) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	dueDateArg := any(dueDate)
	if dueDate == "" {
		dueDateArg = nil
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO tasks (id, owner_id, property_id, rule_id, due_date, due_time, title, completed_date)
		VALUES ($1, $2, $3, $4, $5::date, $6::time, $7, $8::date)`,
		id, ownerID, propertyPtrFor(propertyID), propertyPtrFor(ruleID), dueDateArg, dueTime, "Показ квартиры", completedDate)
	require.NoError(t, err)
	return id
}

// insertScanRule adds one task rule row — the FK target for a task's
// rule_id.
func insertScanRule(t *testing.T, pool *pgxpool.Pool, ownerID, propertyID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(context.Background(), `
		INSERT INTO task_rules (id, owner_id, property_id, title, repeat)
		VALUES ($1, $2, $3, 'Показ квартиры', 'once')`,
		id, ownerID, propertyID)
	require.NoError(t, err)
	return id
}

// propertyPtrFor passes the property binding through: a zero id means the
// task without a property.
func propertyPtrFor(propertyID uuid.UUID) any {
	if propertyID == uuid.Nil {
		return nil
	}
	return propertyID
}

// The sweep's zones: owners with active dated tasks — on non-archived
// properties or without one (ADR 0052).
func TestTaskScanStore_ListScanZones(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	mskProp := createLiveProperty(t, pool, msk)
	insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-19", new("15:13"), nil)

	kgd := createUserInZone(t, pool, "Europe/Kaliningrad")
	kgdProp := createLiveProperty(t, pool, kgd)
	insertScanTask(t, pool, kgd, kgdProp, uuid.Nil, "2026-09-19", nil, nil)

	// The task without a property sweeps its owner's zone too.
	lo := createUserInZone(t, pool, "Europe/Kaliningrad")
	insertScanTask(t, pool, lo, uuid.Nil, uuid.Nil, "2026-09-19", new("09:00"), nil)

	// An owner whose only task is undated — no zone (без срока — никогда).
	undated := createUserInZone(t, pool, "Europe/Moscow")
	insertScanTask(t, pool, undated, uuid.Nil, uuid.Nil, "", nil, nil)

	// An archived property's task stays out of the sweep (the ticks' canon):
	// the task's mutations are rejected there, «Выполнить» would be a dead
	// end.
	archived := createUserInZone(t, pool, "Europe/Moscow")
	archivedProp := createLiveProperty(t, pool, archived)
	insertScanTask(t, pool, archived, archivedProp, uuid.Nil, "2026-09-19", new("15:13"), nil)
	archiveProperty(t, pool, archived, archivedProp)

	store := NewTaskScanStore(pool)
	zones, err := store.ListScanZones(ctx)
	require.NoError(t, err)

	got := make([]string, 0, len(zones))
	for _, z := range zones {
		got = append(got, z.Timezone)
	}
	assert.Contains(t, got, "Europe/Moscow")
	assert.Contains(t, got, "Europe/Kaliningrad")
}

// The scheduled leg's booking list: the timed tasks whose term instant falls
// in (from, until], the instant read in the owner's timezone.
func TestTaskScanStore_ListScheduledTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	// The sweep instant: 13:00 UTC on the 19th — 16:00 in Moscow.
	now := time.Date(2026, 9, 19, 13, 0, 0, 0, time.UTC)
	until := now.Add(48 * time.Hour)

	msk := createUserInZone(t, pool, "Europe/Moscow")
	mskProp := createLiveProperty(t, pool, msk)

	// 18:30 Moscow on the 19th = 15:30 UTC — inside the window.
	inside := insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-19", new("18:30"), nil)
	// The task without a property gets its job too (ADR 0052).
	noProp := insertScanTask(t, pool, msk, uuid.Nil, uuid.Nil, "2026-09-21", new("09:00"), nil)
	// Exactly at the window's upper bound — included (until inclusive).
	atEdge := insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-21", new("16:00"), nil)

	// Right at the lower bound — the window is open on the left: a term
	// already at the sweep instant is the overdue leg's business.
	atLowerBound := insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-19", new("16:00"), nil)
	// Beyond the horizon — books on a later pass.
	beyondHorizon := insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-22", new("16:00"), nil)
	// The date-only task books at its day-after midnight (issue #777):
	// due 2026-09-20 → 00:00 Moscow of the 21st = 2026-09-20T21:00Z, inside
	// the window.
	dateOnly := insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-20", nil, nil)
	doneAt := "2026-09-18"
	completed := insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-20", new("10:00"), &doneAt)

	store := NewTaskScanStore(pool)
	targets, err := store.ListScheduledTargets(ctx, now, until)
	require.NoError(t, err)

	got := make(map[uuid.UUID]time.Time, len(targets))
	for _, target := range targets {
		got[target.TaskID] = target.DueAt
	}
	require.Contains(t, got, inside)
	assert.True(t, got[inside].Equal(time.Date(2026, 9, 19, 15, 30, 0, 0, time.UTC)),
		"18:30 Moscow on the 19th = 15:30 UTC, got %s", got[inside])
	require.Contains(t, got, noProp)
	assert.True(t, got[noProp].Equal(time.Date(2026, 9, 21, 6, 0, 0, 0, time.UTC)),
		"09:00 Moscow on the 21st = 06:00 UTC, got %s", got[noProp])
	require.Contains(t, got, atEdge)
	assert.True(t, got[atEdge].Equal(until), "the upper bound is inclusive, got %s", got[atEdge])
	require.Contains(t, got, dateOnly, "the date-only task books at its day-after midnight (issue #777)")
	assert.True(t, got[dateOnly].Equal(time.Date(2026, 9, 20, 21, 0, 0, 0, time.UTC)),
		"00:00 Moscow of the 21st = 2026-09-20T21:00Z, got %s", got[dateOnly])
	// The shared test database's parallel fixtures add targets of their own —
	// assert this test's tasks, not the world: the boundary, out-of-window
	// and completed ones stay out.
	for _, gone := range []uuid.UUID{atLowerBound, beyondHorizon, completed} {
		assert.NotContains(t, got, gone)
	}
}

// The zone's overdue list (решение #737, тип №4): the timed tasks past their
// term minute, the date-only ones after the day's end — the sweep instant
// decides, per zone.
func TestTaskScanStore_ListOverdueTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	// 21:30 UTC on the 19th — Moscow has just rolled past midnight (00:30 on
	// the 20th), Kaliningrad (UTC+2) is still on the 19th at 23:30.
	now := time.Date(2026, 9, 19, 21, 30, 0, 0, time.UTC)

	msk := createUserInZone(t, pool, "Europe/Moscow")
	mskProp := createLivePropertyFor(t, pool, msk, "Квартира на Ленина", "г. Москва, ул. Ленина, 1")

	// 00:15 Moscow on the 20th — the term minute has passed. Its rule id
	// travels for the action's screen (the rule's edit screen).
	rule := insertScanRule(t, pool, msk, mskProp)
	pastMinute := insertScanTask(t, pool, msk, mskProp, rule, "2026-09-20", new("00:15"), nil)
	// Exactly the current minute — overdue «в минуту срока, включительно».
	exactMinute := insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-20", new("00:30"), nil)
	// The date-only term on the 19th — its day's end has passed.
	dateOnly := insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-19", nil, nil)
	// The task without a property is its owner's business.
	noProp := insertScanTask(t, pool, msk, uuid.Nil, uuid.Nil, "2026-09-18", new("23:00"), nil)

	// A later minute today, a date-only term of today (its day is not over),
	// an undated task and a completed one — none fires.
	laterMinute := insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-20", new("00:45"), nil)
	todayDateOnly := insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-20", nil, nil)
	undated := insertScanTask(t, pool, msk, uuid.Nil, uuid.Nil, "", nil, nil)
	doneAt := "2026-09-19"
	completed := insertScanTask(t, pool, msk, mskProp, uuid.Nil, "2026-09-18", new("23:00"), &doneAt)

	// Kaliningrad's own calendar: a date-only term of the 18th fires, the
	// 19th's does not — the day has not ended there yet.
	kgd := createUserInZone(t, pool, "Europe/Kaliningrad")
	kgdProp := createLiveProperty(t, pool, kgd)
	kgdOverdue := insertScanTask(t, pool, kgd, kgdProp, uuid.Nil, "2026-09-18", nil, nil)
	kgdFuture := insertScanTask(t, pool, kgd, kgdProp, uuid.Nil, "2026-09-19", nil, nil)

	store := NewTaskScanStore(pool)

	mskTargets, err := store.ListOverdueTargets(ctx, "Europe/Moscow", time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	ids := make(map[uuid.UUID]application.TaskOverdueTarget, len(mskTargets))
	for _, target := range mskTargets {
		ids[target.TaskID] = target
	}
	for _, id := range []uuid.UUID{pastMinute, exactMinute, dateOnly, noProp} {
		assert.Contains(t, ids, id)
	}
	// The shared test database's parallel fixtures add targets of their own —
	// assert this test's tasks, not the world: the later minute, the
	// date-only term of today, the undated and the completed ones stay out.
	for _, gone := range []uuid.UUID{laterMinute, todayDateOnly, undated, completed} {
		assert.NotContains(t, ids, gone)
	}

	// The snapshot the publication carries (решение владельца 19.09.2026,
	// #745), the rule id — the action's screen — and the term's wall-clock
	// minute.
	target := ids[pastMinute]
	require.NotNil(t, target.PropertyID)
	assert.Equal(t, mskProp, *target.PropertyID)
	assert.Equal(t, "Квартира на Ленина", target.PropertyName)
	assert.Equal(t, "г. Москва, ул. Ленина, 1", target.PropertyAddress)
	require.NotNil(t, target.RuleID)
	assert.Equal(t, rule, *target.RuleID)
	require.NotNil(t, target.DueTime)
	assert.Equal(t, application.TaskDueTime(15), *target.DueTime)
	// The task without a property carries none.
	assert.Nil(t, ids[noProp].PropertyID)
	require.NotNil(t, ids[noProp].DueTime)
	assert.Equal(t, application.TaskDueTime(23*60), *ids[noProp].DueTime)
	// The date-only target has no time half.
	assert.Nil(t, ids[dateOnly].DueTime)

	kgdTargets, err := store.ListOverdueTargets(ctx, "Europe/Kaliningrad", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	kgdIDs := make([]uuid.UUID, 0, len(kgdTargets))
	for _, target := range kgdTargets {
		kgdIDs = append(kgdIDs, target.TaskID)
	}
	// Own fixtures only — the 19th has not ended in Kaliningrad, this test's
	// future task stays out.
	assert.Contains(t, kgdIDs, kgdOverdue)
	assert.NotContains(t, kgdIDs, kgdFuture)
}

// The due-minute job's reload: the live timed task answers its target, a
// gone, completed, date-only or archived-property task answers nil — the
// job finishes without publishing.
func TestTaskScanStore_GetScheduledOverdueTask(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLivePropertyFor(t, pool, msk, "Квартира на Ленина", "г. Москва, ул. Ленина, 1")
	rule := insertScanRule(t, pool, msk, prop)

	live := insertScanTask(t, pool, msk, prop, rule, "2026-09-19", new("15:13"), nil)
	// One (rule, date) per task — the materialization dedup index forbids
	// two rows of a rule on one date.
	doneAt := "2026-09-19"
	completed := insertScanTask(t, pool, msk, prop, rule, "2026-09-18", new("15:13"), &doneAt)
	dateOnly := insertScanTask(t, pool, msk, prop, rule, "2026-09-17", nil, nil)

	archived := createUserInZone(t, pool, "Europe/Moscow")
	archivedProp := createLiveProperty(t, pool, archived)
	onArchived := insertScanTask(t, pool, archived, archivedProp, uuid.Nil, "2026-09-19", new("15:13"), nil)
	archiveProperty(t, pool, archived, archivedProp)

	store := NewTaskScanStore(pool)

	target, isLive, err := store.GetScheduledOverdueTask(ctx, live)
	require.NoError(t, err)
	require.True(t, isLive)
	assert.Equal(t, live, target.TaskID)
	assert.Equal(t, "Показ квартиры", target.Title)
	assert.Equal(t, msk, target.OwnerID)
	require.NotNil(t, target.PropertyID)
	assert.Equal(t, prop, *target.PropertyID)
	require.NotNil(t, target.DueTime)
	assert.Equal(t, application.TaskDueTime(15*60+13), *target.DueTime)
	// The term's instant in the owner's timezone — the creation/edit seam's
	// future-vs-past decision (#775): 15:13 Moscow = 12:13 UTC.
	assert.True(t, target.DueAt.Equal(time.Date(2026, 9, 19, 12, 13, 0, 0, time.UTC)),
		"15:13 Moscow = 12:13 UTC, got %s", target.DueAt)

	_, isLive, err = store.GetScheduledOverdueTask(ctx, completed)
	require.NoError(t, err)
	assert.False(t, isLive, "a completed task does not fire at its old minute")

	dateTarget, isLive, err := store.GetScheduledOverdueTask(ctx, dateOnly)
	require.NoError(t, err)
	assert.True(t, isLive, "the date-only task answers at its own midnight boundary (issue #777)")
	assert.Nil(t, dateTarget.DueTime, "a date-only task carries no wall-clock minute")
	// The day-after midnight in the owner's zone — the booked job's
	// ScheduledAt: 00:00 Moscow of the 18th = 2026-09-17T21:00Z.
	assert.True(t, dateTarget.DueAt.Equal(time.Date(2026, 9, 17, 21, 0, 0, 0, time.UTC)),
		"00:00 Moscow of the 18th = 2026-09-17T21:00Z, got %s", dateTarget.DueAt)

	_, isLive, err = store.GetScheduledOverdueTask(ctx, onArchived)
	require.NoError(t, err)
	assert.False(t, isLive, "an archived property's task stays out (the ticks' canon)")

	_, isLive, err = store.GetScheduledOverdueTask(ctx, uuid.Must(uuid.NewV7()))
	require.NoError(t, err)
	assert.False(t, isLive, "a task gone with its rule edit is a no-op")
}
