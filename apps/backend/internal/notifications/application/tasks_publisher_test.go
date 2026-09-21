package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zoneKGD is the second sweep zone of the tests: the Kaliningrad owner
// timezone, an hour behind Moscow.
const zoneKGD = "Europe/Kaliningrad"

// fakeTaskSource is the tasks scan source stub: it answers the scheduled
// window, the per-zone overdue sweep and the scheduled job's reload, and
// records every question so the tests assert the sweep's shape.
type fakeTaskSource struct {
	// Scheduled answers the upcoming-timed-tasks question; scheduledAsked
	// records the "from|until" window in RFC3339.
	scheduled      []TaskScheduleTarget
	scheduledAsked []string
	scheduledErr   error
	// Overdue answers the per-zone sweep; asked records the
	// "zone|today|HH:MM(now)" questions in order.
	overdue map[string][]TaskOverdueTarget
	asked   []string
	// Live answers the scheduled job's reload: the id → the target, a
	// missing id → nil (the no-op).
	live   map[uuid.UUID]TaskOverdueTarget
	recips map[uuid.UUID][]uuid.UUID
	recErr map[uuid.UUID]error
}

func (f *fakeTaskSource) ListScheduledTargets(ctx context.Context, from, until time.Time) ([]TaskScheduleTarget, error) {
	f.scheduledAsked = append(f.scheduledAsked, from.Format(time.RFC3339)+"|"+until.Format(time.RFC3339))
	if f.scheduledErr != nil {
		return nil, f.scheduledErr
	}
	return f.scheduled, nil
}

func (f *fakeTaskSource) ListOverdueTargets(ctx context.Context, zone string, today, now time.Time) ([]TaskOverdueTarget, error) {
	f.asked = append(f.asked, zone+"|"+today.Format("2006-01-02")+"|"+now.Format("15:04"))
	return f.overdue[zone], nil
}

func (f *fakeTaskSource) GetScheduledOverdueTask(ctx context.Context, taskID uuid.UUID) (TaskOverdueTarget, bool, error) {
	target, ok := f.live[taskID]
	return target, ok, nil
}

func (f *fakeTaskSource) ListActiveRecipients(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error) {
	if err := f.recErr[propertyID]; err != nil {
		return nil, err
	}
	return f.recips[propertyID], nil
}

// fakeTaskScheduler is the scheduled-jobs stub: it books every (task, due
// instant) pair and can fail a task's booking on demand.
type fakeTaskScheduler struct {
	booked map[uuid.UUID]time.Time
	errFor map[uuid.UUID]error
	order  []uuid.UUID
}

func (f *fakeTaskScheduler) ScheduleTaskOverdue(ctx context.Context, taskID uuid.UUID, dueAt time.Time) error {
	if err := f.errFor[taskID]; err != nil {
		return err
	}
	if f.booked == nil {
		f.booked = map[uuid.UUID]time.Time{}
	}
	f.booked[taskID] = dueAt
	f.order = append(f.order, taskID)
	return nil
}

// taskScanHarness builds the tasks publisher over the real pipeline backed
// by the publisher tests' fakes: the tests read the feed rows the scan
// created.
type taskScanHarness struct {
	pipeline  *Publisher
	feed      *fakeFeedRepo
	queue     *fakeQueue
	zones     *fakeScanZones
	source    *fakeTaskSource
	scheduler *fakeTaskScheduler
}

func newTaskScanHarness(zones []ScanZone) *taskScanHarness {
	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	pipeline := NewPublisher(feed, queue, nil, &fakeUoW{}, nil)
	return &taskScanHarness{
		pipeline: pipeline,
		feed:     feed,
		queue:    queue,
		zones:    &fakeScanZones{zones: zones},
		source: &fakeTaskSource{
			overdue: map[string][]TaskOverdueTarget{},
			live:    map[uuid.UUID]TaskOverdueTarget{},
			recips:  map[uuid.UUID][]uuid.UUID{},
			recErr:  map[uuid.UUID]error{},
		},
		scheduler: &fakeTaskScheduler{},
	}
}

// overdueTaskTarget builds a property-bound overdue task target: the task id
// is the payload link and the dedup key's entity half, the rule id — the
// screen the action lands on.
func overdueTaskTarget(taskID uuid.UUID, title string, dueDate time.Time, dueTime *TaskDueTime) TaskOverdueTarget {
	ruleID := uuid.Must(uuid.NewV7())
	return TaskOverdueTarget{
		TaskID:          taskID,
		Title:           title,
		DueDate:         dueDate,
		DueTime:         dueTime,
		RuleID:          &ruleID,
		PropertyID:      &[]uuid.UUID{uuid.Must(uuid.NewV7())}[0],
		PropertyName:    scanPropertyName,
		PropertyAddress: scanPropertyAddress,
		OwnerID:         uuid.Must(uuid.NewV7()),
	}
}

func dueTimePtr(h, m int) *TaskDueTime {
	due := TaskDueTime(h*60 + m)
	return &due
}

// The timed task's trigger (решение #737, тип №4: в минуту срока): the
// overdue sweep or the due-minute job publishes «Задача просрочена» to the
// owner and the active members, the body names the wall-clock «Срок был».
func TestTasksPublisher_PublishesOverdueTimedTask(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	task := uuid.Must(uuid.NewV7())
	target := overdueTaskTarget(task, "Показ квартиры",
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), dueTimePtr(15, 13))
	h.source.overdue[zoneMSK] = []TaskOverdueTarget{target}
	member := uuid.Must(uuid.NewV7())
	h.source.recips[*target.PropertyID] = []uuid.UUID{member}

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))

	// Owner plus one active member — one row each, both channels behind it.
	require.Len(t, h.feed.inserted, 2)
	recipients := []uuid.UUID{h.feed.inserted[0].UserID, h.feed.inserted[1].UserID}
	assert.ElementsMatch(t, []uuid.UUID{target.OwnerID, member}, recipients)

	for _, n := range h.feed.inserted {
		assert.Equal(t, domain.EventTaskOverdue, n.EventType)
		assert.Equal(t, domain.CategoryTasks, n.Category)
		assert.Equal(t, "Задача просрочена", n.Title)
		assert.Equal(t,
			"Задача «Показ квартиры» по объекту «Квартира на Ленина» просрочена. Срок был: 18 сентября, 15:13",
			n.Body)
		assert.Equal(t, "Квартира на Ленина", n.ContextLabel)
		// The dedup key is the task id alone (решение #737, тип №4): one
		// task — one overdue notification, whoever fires first.
		assert.Equal(t, domain.DedupKey("task_overdue:"+task.String()), n.DedupKey)
		// The payload carries the property snapshot with its address line
		// (решение владельца 19.09.2026, #745) and the task id — the link
		// target is the task's screen.
		require.NotNil(t, n.Payload.Property)
		assert.Equal(t, *target.PropertyID, n.Payload.Property.ID)
		assert.Equal(t, scanPropertyName, n.Payload.Property.Name)
		assert.Equal(t, scanPropertyAddress, n.Payload.Property.Address)
		require.NotNil(t, n.Payload.TaskID)
		assert.Equal(t, task, *n.Payload.TaskID)
		// The action's screen is the rule's: the rule id travels alongside
		// the task id (the tasks UI navigates by it).
		require.NotNil(t, n.Payload.TaskRuleID)
		assert.Equal(t, *target.RuleID, *n.Payload.TaskRuleID)
		assert.Nil(t, n.Payload.Actor, "a system scan has no initiator")
	}
	assert.Len(t, h.queue.emails, 2)
	assert.Len(t, h.queue.pushes, 2)
}

// The date-only task's copy carries no time — «Срок был» names the day alone
// (решение #737: {дата, время}, the time half exists only when the task has
// one).
func TestTasksPublisher_PublishesOverdueDateOnlyTask(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	target := overdueTaskTarget(uuid.Must(uuid.NewV7()), "Сменить замок",
		time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC), nil)
	h.source.overdue[zoneMSK] = []TaskOverdueTarget{target}

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	require.Len(t, h.feed.inserted, 1)

	n := h.feed.inserted[0]
	assert.Equal(t,
		"Задача «Сменить замок» по объекту «Квартира на Ленина» просрочена. Срок был: 17 сентября",
		n.Body)
}

// The task without a property (ADR 0052): no object in the body, no context
// label, no property in the payload — and the owner alone as the recipient
// (решение #737: безобъектная — владельца).
func TestTasksPublisher_PublishesTaskWithoutProperty(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	owner := uuid.Must(uuid.NewV7())
	target := TaskOverdueTarget{
		TaskID:  uuid.Must(uuid.NewV7()),
		Title:   "Позвонить в ЖЭК",
		DueDate: time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC),
		RuleID:  &[]uuid.UUID{uuid.Must(uuid.NewV7())}[0],
		OwnerID: owner,
	}
	h.source.overdue[zoneMSK] = []TaskOverdueTarget{target}

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	require.Len(t, h.feed.inserted, 1)

	n := h.feed.inserted[0]
	assert.Equal(t, "Задача «Позвонить в ЖЭК» просрочена. Срок был: 16 сентября", n.Body)
	assert.Empty(t, n.ContextLabel, "no property — no context label (макет)")
	assert.Nil(t, n.Payload.Property)
	require.NotNil(t, n.Payload.TaskID)
	require.NotNil(t, n.Payload.TaskRuleID, "the rule id travels even without a property")
	assert.Equal(t, []uuid.UUID{owner}, []uuid.UUID{n.UserID})
}

// The hourly sweep runs the scheduled leg first — every upcoming timed task
// in the window gets its due-minute job booked at the instant the source
// named — and then sweeps the zones for the already-overdue ones.
func TestTasksPublisher_SchedulesUpcomingTimedTasks(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	first := uuid.Must(uuid.NewV7())
	second := uuid.Must(uuid.NewV7())
	due1 := time.Date(2026, 9, 20, 12, 13, 0, 0, time.UTC)
	due2 := time.Date(2026, 9, 21, 8, 30, 0, 0, time.UTC)
	h.source.scheduled = []TaskScheduleTarget{
		{TaskID: first, DueAt: due1},
		{TaskID: second, DueAt: due2},
	}

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))

	require.Len(t, h.scheduler.booked, 2)
	assert.Equal(t, due1, h.scheduler.booked[first])
	assert.Equal(t, due2, h.scheduler.booked[second])
	// The window covers the next two days from the sweep's instant: a task
	// whose term is further out joins on a later hourly pass, an already
	// passed term is the overdue leg's business.
	require.Len(t, h.source.scheduledAsked, 1)
	from, until, err := parseScheduledWindow(h.source.scheduledAsked[0])
	require.NoError(t, err)
	assert.Equal(t, scanNow, from)
	assert.Equal(t, scanNow.Add(scheduledHorizon), until)
}

// A broken booking does not stop the rest: the surviving tasks are booked,
// the joined error reports the failure (the scan's isolation canon).
func TestTasksPublisher_SchedulingFailuresAreIsolated(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	broken := uuid.Must(uuid.NewV7())
	survivor := uuid.Must(uuid.NewV7())
	h.source.scheduled = []TaskScheduleTarget{
		{TaskID: broken, DueAt: scanNow.Add(time.Hour)},
		{TaskID: survivor, DueAt: scanNow.Add(2 * time.Hour)},
	}
	h.scheduler.errFor = map[uuid.UUID]error{broken: errors.New("queue down")}

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	err := p.RunZoneScans(context.Background(), scanNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), broken.String())
	assert.Equal(t, scanNow.Add(2*time.Hour), h.scheduler.booked[survivor])
}

// The due-minute job's payload: reload the task and publish. The row is the
// same «Задача просрочена» the sweep would write — the dedup key keeps the
// two legs to one notification whichever fires first.
func TestTasksPublisher_DeliverTaskOverduePublishes(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	task := uuid.Must(uuid.NewV7())
	target := overdueTaskTarget(task, "Показ квартиры",
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), dueTimePtr(15, 13))
	h.source.live[task] = target

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.DeliverTaskOverdue(context.Background(), task))
	require.Len(t, h.feed.inserted, 1)
	assert.Equal(t, domain.DedupKey("task_overdue:"+task.String()), h.feed.inserted[0].DedupKey)
}

// The scheduled job's no-op: the task the job was booked for may be gone
// (the rule edit removes stale tasks) or completed — the reload answers nil
// and the job finishes without publishing. A date-only task is not a no-op:
// since #777 its own midnight job (and the seam) plans it like a timed one.
func TestTasksPublisher_DeliverTaskOverdueNoOpWhenGone(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	task := uuid.Must(uuid.NewV7())

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.DeliverTaskOverdue(context.Background(), task))
	assert.Empty(t, h.feed.inserted)
}

// The date-only task's boundary job (issue #777, решение владельца
// 21.09.2026: все уведомления — чётко по времени): the reload answers a live
// date-only task at its midnight — 00:00 of the day after the due date —
// and the publication is the day-only copy the sweep leg writes.
func TestTasksPublisher_DeliverTaskOverduePublishesDateOnlyAtMidnight(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	task := uuid.Must(uuid.NewV7())
	target := overdueTaskTarget(task, "Сменить замок",
		time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC), nil)
	h.source.live[task] = target

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.DeliverTaskOverdue(context.Background(), task))
	require.Len(t, h.feed.inserted, 1)
	assert.Equal(t,
		"Задача «Сменить замок» по объекту «Квартира на Ленина» просрочена. Срок был: 17 сентября",
		h.feed.inserted[0].Body)
	assert.Equal(t, domain.DedupKey("task_overdue:"+task.String()), h.feed.inserted[0].DedupKey)
}

// Both legs may fire for the same task — the scheduled job and the next
// sweep: the dedup key keeps the feed to one row.
func TestTasksPublisher_JobAndSweepShareOneRow(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	task := uuid.Must(uuid.NewV7())
	target := overdueTaskTarget(task, "Показ квартиры",
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), dueTimePtr(15, 13))
	h.source.overdue[zoneMSK] = []TaskOverdueTarget{target}
	h.source.recips[*target.PropertyID] = []uuid.UUID{uuid.Must(uuid.NewV7())}

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	require.Len(t, h.feed.inserted, 2, "owner + member from the sweep")

	h.source.overdue[zoneMSK] = []TaskOverdueTarget{target}
	require.NoError(t, p.DeliverTaskOverdue(context.Background(), task))
	assert.Len(t, h.feed.inserted, 2, "the job's pass inserts nothing")
}

// Each zone is swept with its own calendar date and the sweep's instant —
// the timed task's overdue minute is a zone-local fact.
func TestTasksPublisher_EachZoneSweptWithItsOwnToday(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}, {Timezone: zoneKGD}})
	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))

	assert.ElementsMatch(t, []string{
		"Europe/Moscow|2026-09-20|21:30",
		"Europe/Kaliningrad|2026-09-19|21:30",
	}, h.source.asked)
}

// The creation/edit seam (issue #775): a task born between the hourly passes
// with its term before the next one books its due-minute job at once — the
// notification lands exactly at the term's minute, not up to an hour late.
func TestTasksPublisher_NotifyMaterializedBooksFutureTerm(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	task := uuid.Must(uuid.NewV7())
	due := scanNow.Add(2 * time.Minute)
	target := overdueTaskTarget(task, "Показ квартиры",
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), dueTimePtr(9, 5))
	target.DueAt = due
	h.source.live[task] = target

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.NotifyMaterializedTasks(context.Background(), []uuid.UUID{task}, scanNow))

	require.Len(t, h.scheduler.booked, 1)
	assert.Equal(t, due, h.scheduler.booked[task])
	assert.Empty(t, h.feed.inserted, "a future term only books the job")
}

// A task born already overdue — its term passed before the handover —
// publishes at once: waiting for the next sweep would be up to an hour late,
// the delay the seam exists to remove. The boundary is the canon minute
// (включительно): a term at the sweep instant is overdue, not future.
func TestTasksPublisher_NotifyMaterializedPublishesBornOverdue(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	task := uuid.Must(uuid.NewV7())
	target := overdueTaskTarget(task, "Показ квартиры",
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), dueTimePtr(15, 13))
	target.DueAt = scanNow.Add(-time.Minute)
	h.source.live[task] = target
	member := uuid.Must(uuid.NewV7())
	h.source.recips[*target.PropertyID] = []uuid.UUID{member}

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.NotifyMaterializedTasks(context.Background(), []uuid.UUID{task}, scanNow))

	assert.Empty(t, h.scheduler.booked, "a passed term publishes, nothing to book")
	require.Len(t, h.feed.inserted, 2)
	assert.Equal(t, domain.DedupKey("task_overdue:"+task.String()), h.feed.inserted[0].DedupKey)

	// The boundary: the term at the handover instant itself is overdue too.
	boundary := uuid.Must(uuid.NewV7())
	atTerm := overdueTaskTarget(boundary, "Второй показ",
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), dueTimePtr(21, 40))
	atTerm.DueAt = scanNow
	h.source.live[boundary] = atTerm
	h.source.recips[*atTerm.PropertyID] = []uuid.UUID{member}
	require.NoError(t, p.NotifyMaterializedTasks(context.Background(), []uuid.UUID{boundary}, scanNow))
	assert.Empty(t, h.scheduler.booked, "the term minute reached is overdue, not future")
	require.Len(t, h.feed.inserted, 4)
}

// The seam's no-op: a task gone between the materialization and the handover
// (the rule edit removes stale rows; a completed task answers nothing too) —
// the reload answers nothing and the seam moves on without booking or
// publishing. A live date-only task is not a no-op since #777 — see the
// boundary tests below.
func TestTasksPublisher_NotifyMaterializedSkipsDeadTask(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	gone := uuid.Must(uuid.NewV7())

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.NotifyMaterializedTasks(context.Background(), []uuid.UUID{gone}, scanNow))

	assert.Empty(t, h.scheduler.booked)
	assert.Empty(t, h.feed.inserted)
}

// A date-only task born between the hourly passes (issue #777) plans at its
// own boundary, the same shape the timed tasks run: a boundary ahead — 00:00
// of the day after the due date, in the owner's zone — books the job at
// once, no wait for the next sweep.
func TestTasksPublisher_NotifyMaterializedBooksDateOnlyBoundary(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	task := uuid.Must(uuid.NewV7())
	// The due date 2026-09-20 (the zone's today at the handover) in Moscow:
	// the boundary is midnight of the 21st — 2026-09-20T21:00Z, still ahead.
	target := overdueTaskTarget(task, "Сменить замок",
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), nil)
	target.DueAt = time.Date(2026, 9, 20, 21, 0, 0, 0, time.UTC)
	h.source.live[task] = target

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.NotifyMaterializedTasks(context.Background(), []uuid.UUID{task}, scanNow))

	require.Len(t, h.scheduler.booked, 1)
	assert.Equal(t, time.Date(2026, 9, 20, 21, 0, 0, 0, time.UTC), h.scheduler.booked[task])
	assert.Empty(t, h.feed.inserted, "a future boundary only books the job")
}

// A date-only task born already overdue — its midnight passed before the
// handover — publishes at once; waiting for the next sweep would be up to an
// hour late, the delay the seam exists to remove.
func TestTasksPublisher_NotifyMaterializedPublishesDateOnlyBornOverdue(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	task := uuid.Must(uuid.NewV7())
	target := overdueTaskTarget(task, "Сменить замок",
		time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC), nil)
	target.DueAt = time.Date(2026, 9, 17, 21, 0, 0, 0, time.UTC)
	h.source.live[task] = target
	member := uuid.Must(uuid.NewV7())
	h.source.recips[*target.PropertyID] = []uuid.UUID{member}

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.NotifyMaterializedTasks(context.Background(), []uuid.UUID{task}, scanNow))

	assert.Empty(t, h.scheduler.booked, "a passed boundary publishes, nothing to book")
	require.Len(t, h.feed.inserted, 2)
	assert.Equal(t, domain.DedupKey("task_overdue:"+task.String()), h.feed.inserted[0].DedupKey)
}

// A broken task's planning does not stop the rest: the surviving tasks are
// handled, the joined error reports the failure — the caller is best-effort
// and logs it (the scan's isolation canon).
func TestTasksPublisher_NotifyMaterializedFailuresAreIsolated(t *testing.T) {
	t.Parallel()

	h := newTaskScanHarness([]ScanZone{{Timezone: zoneMSK}})
	broken := uuid.Must(uuid.NewV7())
	survivor := uuid.Must(uuid.NewV7())
	brokenTarget := overdueTaskTarget(broken, "Сломалась",
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), dueTimePtr(15, 13))
	brokenTarget.DueAt = scanNow.Add(time.Hour)
	h.source.live[broken] = brokenTarget
	survivorTarget := overdueTaskTarget(survivor, "Выжила",
		time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), dueTimePtr(16, 13))
	survivorTarget.DueAt = scanNow.Add(2 * time.Hour)
	h.source.live[survivor] = survivorTarget
	h.scheduler.errFor = map[uuid.UUID]error{broken: errors.New("queue down")}

	p := NewTasksPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	err := p.NotifyMaterializedTasks(context.Background(), []uuid.UUID{broken, survivor}, scanNow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), broken.String())
	assert.Equal(t, survivorTarget.DueAt, h.scheduler.booked[survivor])
}

func parseScheduledWindow(raw string) (from, until time.Time, err error) {
	parts := strings.Split(raw, "|")
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("malformed window %q", raw)
	}
	from, err = time.Parse(time.RFC3339, parts[0])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	until, err = time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return from, until, nil
}
