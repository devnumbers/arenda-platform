package application

// The unit tests of the global tasks listing (ticket #521): the query
// preparation and the use case's branch, per-owner-clock and property-name
// discipline over func-backed doubles. The SQL visibility predicate and the
// ordering live in the postgres adapter and are the integration families'
// subject.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// globalFixtures: the reading actor, the shared property's owner and their
// moments. The actor's frame says 2026-09-10 12:00 Moscow; the foreign
// owner's frame is already 2026-09-11, which turns an equal due date into a
// overdue for them and not for the actor.
var (
	globalActor    = uuid.Must(uuid.NewV7())
	globalOther    = uuid.Must(uuid.NewV7())
	globalProperty = uuid.Must(uuid.NewV7())

	actorMoment = MomentAt(
		time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), mustLoc("Europe/Moscow"))
	otherMoment = MomentAt(
		time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC), mustLoc("Europe/Moscow"))
)

func mustLoc(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func globalTask(owner uuid.UUID, propertyID *uuid.UUID, due *time.Time) domain.Task {
	return domain.Task{
		ID:         uuid.Must(uuid.NewV7()),
		OwnerID:    owner,
		PropertyID: propertyID,
		DueDate:    due,
		Title:      "Вынести мусор",
	}
}

func globalRow(owner uuid.UUID, propertyID *uuid.UUID, due *time.Time, name string) GlobalTaskRow {
	return GlobalTaskRow{Task: globalTask(owner, propertyID, due), PropertyName: name}
}

// globalClockDouble serves canned moments and logs the owners it was asked
// about — the per-owner moment discipline is visible in the call log.
type globalClockDouble struct {
	moments map[uuid.UUID]OwnerMoment
	calls   []uuid.UUID
}

func (c *globalClockDouble) Moment(_ context.Context, owner uuid.UUID) (OwnerMoment, error) {
	c.calls = append(c.calls, owner)
	moment, ok := c.moments[owner]
	if !ok {
		return OwnerMoment{}, errors.New("no moment stubbed for " + owner.String())
	}
	return moment, nil
}

// globalTaskStoreDouble is the TaskStore double: only the seats the global
// listing reads are func-backed; the embedded nil interface panics on any
// other call, so an unexpected store touch fails the test loudly.
type globalTaskStoreDouble struct {
	TaskStore
	listGlobal                func(ctx context.Context, actor uuid.UUID, q TasksListQuery) ([]GlobalTaskRow, int, error)
	listGlobalWithoutProperty func(ctx context.Context, actor uuid.UUID, q TasksListQuery) ([]GlobalTaskRow, int, error)
	listByProperty            func(ctx context.Context, scope, propertyID uuid.UUID, q TasksListQuery) ([]domain.Task, int, error)
}

func (s *globalTaskStoreDouble) ListGlobal(
	ctx context.Context, actor uuid.UUID, q TasksListQuery,
) ([]GlobalTaskRow, int, error) {
	if s.listGlobal == nil {
		return nil, 0, errors.New("unexpected ListGlobal call")
	}
	return s.listGlobal(ctx, actor, q)
}

func (s *globalTaskStoreDouble) ListGlobalWithoutProperty(
	ctx context.Context, actor uuid.UUID, q TasksListQuery,
) ([]GlobalTaskRow, int, error) {
	if s.listGlobalWithoutProperty == nil {
		return nil, 0, errors.New("unexpected ListGlobalWithoutProperty call")
	}
	return s.listGlobalWithoutProperty(ctx, actor, q)
}

func (s *globalTaskStoreDouble) ListByProperty(
	ctx context.Context, scope, propertyID uuid.UUID, q TasksListQuery,
) ([]domain.Task, int, error) {
	if s.listByProperty == nil {
		return nil, 0, errors.New("unexpected ListByProperty call")
	}
	return s.listByProperty(ctx, scope, propertyID, q)
}

// globalPropertyStoreDouble is the PropertyStore double serving one canned
// reference from Get.
type globalPropertyStoreDouble struct {
	PropertyStore
	ref PropertyRef
	err error
}

func (s *globalPropertyStoreDouble) Get(context.Context, uuid.UUID) (PropertyRef, error) {
	return s.ref, s.err
}

type globalPolicyDouble struct{ role sharedpolicy.Role }

func (p globalPolicyDouble) Role(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleNone, nil
}

func (p globalPolicyDouble) RoleForProperty(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return p.role, nil
}

func newGlobalTaskService(
	store TaskStore, properties PropertyStore, clock OwnerClock, policy sharedpolicy.Policy,
) *TaskService {
	return NewTaskService(
		NewTxStoreFactory(nilTickStore{}, nilRuleStore{}, store, properties, nil, nil),
		nil, clock, policy,
	)
}

// nilTickStore/nilRuleStore fill the factory's unused seats: the listing is
// a read and never binds them to a transaction.
type nilTickStore struct{}

func (nilTickStore) LockOwnerProperties(context.Context, uuid.UUID) error { panic("unused") }
func (nilTickStore) LockOwner(context.Context, uuid.UUID) error           { panic("unused") }
func (nilTickStore) LoadOwnerSnapshot(context.Context, uuid.UUID) (OwnerSnapshot, error) {
	panic("unused")
}

func (nilTickStore) ApplyTickPlan(context.Context, domain.TaskRule, time.Time, domain.TaskTickPlan) error {
	panic("unused")
}
func (nilTickStore) WithTx(transaction.Tx) (TickStore, error) { panic("unused") }

type nilRuleStore struct{ RuleStore }

func TestPrepareGlobalTasksQuery(t *testing.T) {
	t.Parallel()

	t.Run("zero limit becomes the default page", func(t *testing.T) {
		t.Parallel()
		q := GlobalTasksListQuery{}
		require.NoError(t, PrepareGlobalTasksQuery(&q))
		assert.Equal(t, DefaultTasksPageSize, q.Limit)
	})

	t.Run("explicit values travel through", func(t *testing.T) {
		t.Parallel()
		q := GlobalTasksListQuery{
			TasksListQuery:  TasksListQuery{Completed: true, Limit: 7, Offset: 14},
			WithoutProperty: true,
		}
		require.NoError(t, PrepareGlobalTasksQuery(&q))
		assert.Equal(t, 7, q.Limit)
		assert.Equal(t, 14, q.Offset)
		assert.True(t, q.Completed)
	})

	t.Run("both property filters together are invalid input", func(t *testing.T) {
		t.Parallel()
		prop := uuid.Must(uuid.NewV7())
		q := GlobalTasksListQuery{
			TasksListQuery:  TasksListQuery{Limit: 10},
			PropertyID:      &prop,
			WithoutProperty: true,
		}
		assert.ErrorIs(t, PrepareGlobalTasksQuery(&q), ErrInvalidInput)
	})

	t.Run("pagination bounds propagate", func(t *testing.T) {
		t.Parallel()
		q := GlobalTasksListQuery{TasksListQuery: TasksListQuery{Limit: MaxTasksPageSize + 1}}
		assert.ErrorIs(t, PrepareGlobalTasksQuery(&q), ErrInvalidInput)
	})
}

func TestListGlobalTasks_MergedFeed(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC) // Today in the actor's frame.
	rows := []GlobalTaskRow{
		globalRow(globalActor, nil, &due, ""), // The actor's own task, still active in their frame.
		globalRow(globalOther, &globalProperty, &due, "Дача"),
	}
	store := &globalTaskStoreDouble{
		listGlobal: func(_ context.Context, actor uuid.UUID, q TasksListQuery) ([]GlobalTaskRow, int, error) {
			assert.Equal(t, globalActor, actor)
			assert.Equal(t, DefaultTasksPageSize, q.Limit)
			assert.False(t, q.Completed)
			return rows, 2, nil
		},
	}
	clock := &globalClockDouble{moments: map[uuid.UUID]OwnerMoment{
		globalActor: actorMoment,
		globalOther: otherMoment,
	}}
	svc := newGlobalTaskService(store, &globalPropertyStoreDouble{}, clock, globalPolicyDouble{role: sharedpolicy.RoleOwner})

	page, err := svc.ListGlobalTasks(context.Background(), globalActor, GlobalTasksListQuery{})
	require.NoError(t, err)

	require.Len(t, page.Items, 2)
	assert.Equal(t, domain.ViewActive, page.Items[0].Status, "today's task in the owner's frame is active")
	assert.Equal(t, domain.ViewOverdue, page.Items[1].Status, "the same date in the foreign owner's frame is overdue")
	assert.Empty(t, page.Items[0].PropertyName)
	assert.Equal(t, "Дача", page.Items[1].PropertyName)
	assert.Equal(t, 2, page.Total)
	assert.Equal(t, actorMoment.Today, page.Today, "today is the reader's calendar date")
	assert.Equal(t, []uuid.UUID{globalActor, globalOther}, clock.calls, "one moment per distinct data owner")
}

func TestListGlobalTasks_PerOwnerMomentIsCached(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	rows := []GlobalTaskRow{
		globalRow(globalOther, &globalProperty, &due, "Дача"),
		globalRow(globalOther, &globalProperty, &due, "Дача"),
		globalRow(globalOther, &globalProperty, &due, "Дача"),
	}
	store := &globalTaskStoreDouble{
		listGlobal: func(context.Context, uuid.UUID, TasksListQuery) ([]GlobalTaskRow, int, error) {
			return rows, 3, nil
		},
	}
	clock := &globalClockDouble{moments: map[uuid.UUID]OwnerMoment{
		globalActor: actorMoment,
		globalOther: otherMoment,
	}}
	svc := newGlobalTaskService(store, &globalPropertyStoreDouble{}, clock, globalPolicyDouble{role: sharedpolicy.RoleOwner})

	page, err := svc.ListGlobalTasks(context.Background(), globalActor, GlobalTasksListQuery{})
	require.NoError(t, err)
	require.Len(t, page.Items, 3)
	assert.Equal(t, []uuid.UUID{globalActor, globalOther}, clock.calls, "one moment per owner, not per row")
}

func TestListGlobalTasks_WithoutPropertyBranch(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	store := &globalTaskStoreDouble{
		listGlobalWithoutProperty: func(_ context.Context, actor uuid.UUID, q TasksListQuery) ([]GlobalTaskRow, int, error) {
			assert.Equal(t, globalActor, actor)
			assert.False(t, q.Completed)
			return []GlobalTaskRow{globalRow(globalActor, nil, &due, "")}, 1, nil
		},
	}
	clock := &globalClockDouble{moments: map[uuid.UUID]OwnerMoment{globalActor: actorMoment}}
	svc := newGlobalTaskService(store, &globalPropertyStoreDouble{}, clock, globalPolicyDouble{role: sharedpolicy.RoleOwner})

	page, err := svc.ListGlobalTasks(context.Background(), globalActor, GlobalTasksListQuery{WithoutProperty: true})
	require.NoError(t, err)

	require.Len(t, page.Items, 1)
	assert.Equal(t, domain.ViewActive, page.Items[0].Status)
	assert.Empty(t, page.Items[0].PropertyName)
	assert.Equal(t, actorMoment.Today, page.Today)
}

func TestListGlobalTasks_PropertyBranch(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	store := &globalTaskStoreDouble{
		listByProperty: func(_ context.Context, scope, propertyID uuid.UUID, q TasksListQuery) ([]domain.Task, int, error) {
			assert.Equal(t, globalOther, scope, "the SQL scope is the data owner, not the actor")
			assert.Equal(t, globalProperty, propertyID)
			assert.Equal(t, DefaultTasksPageSize, q.Limit)
			return []domain.Task{globalTask(globalOther, &globalProperty, &due)}, 1, nil
		},
	}
	clock := &globalClockDouble{moments: map[uuid.UUID]OwnerMoment{globalOther: otherMoment}}
	properties := &globalPropertyStoreDouble{ref: PropertyRef{OwnerID: globalOther, Name: "Дача"}}
	svc := newGlobalTaskService(store, properties, clock, globalPolicyDouble{role: sharedpolicy.RoleFullAccess})

	page, err := svc.ListGlobalTasks(context.Background(), globalActor, GlobalTasksListQuery{PropertyID: &globalProperty})
	require.NoError(t, err)

	require.Len(t, page.Items, 1)
	assert.Equal(t, domain.ViewOverdue, page.Items[0].Status, "buckets follow the data owner's moment")
	assert.Equal(t, "Дача", page.Items[0].PropertyName)
	assert.Equal(t, otherMoment.Today, page.Today, "today is the data owner's calendar date")
}

func TestListGlobalTasks_PropertyBranchPrivacy(t *testing.T) {
	t.Parallel()

	var listed bool
	store := &globalTaskStoreDouble{
		listByProperty: func(context.Context, uuid.UUID, uuid.UUID, TasksListQuery) ([]domain.Task, int, error) {
			listed = true
			return nil, 0, nil
		},
	}
	svc := newGlobalTaskService(
		store, &globalPropertyStoreDouble{}, &globalClockDouble{},
		globalPolicyDouble{role: sharedpolicy.RoleNone},
	)

	_, err := svc.ListGlobalTasks(context.Background(), globalActor, GlobalTasksListQuery{PropertyID: &globalProperty})
	require.ErrorIs(t, err, ErrNotFound, "a stranger gets the privacy 404")
	assert.False(t, listed, "the store is never touched without the view capability")
}

func TestListGlobalTasks_BothFiltersRejected(t *testing.T) {
	t.Parallel()

	store := &globalTaskStoreDouble{
		listGlobal: func(context.Context, uuid.UUID, TasksListQuery) ([]GlobalTaskRow, int, error) {
			t.Error("the store must not be called for an invalid query")
			return nil, 0, nil
		},
	}
	svc := newGlobalTaskService(store, &globalPropertyStoreDouble{}, &globalClockDouble{}, globalPolicyDouble{role: sharedpolicy.RoleOwner})

	prop := globalProperty
	_, err := svc.ListGlobalTasks(context.Background(), globalActor, GlobalTasksListQuery{
		TasksListQuery:  TasksListQuery{Limit: 10},
		PropertyID:      &prop,
		WithoutProperty: true,
	})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestListGlobalTasks_StoreErrorPasses(t *testing.T) {
	t.Parallel()

	store := &globalTaskStoreDouble{
		listGlobal: func(context.Context, uuid.UUID, TasksListQuery) ([]GlobalTaskRow, int, error) {
			return nil, 0, errors.New("boom")
		},
	}
	svc := newGlobalTaskService(store, &globalPropertyStoreDouble{}, &globalClockDouble{moments: map[uuid.UUID]OwnerMoment{
		globalActor: actorMoment,
	}}, globalPolicyDouble{role: sharedpolicy.RoleOwner})

	_, err := svc.ListGlobalTasks(context.Background(), globalActor, GlobalTasksListQuery{})
	assert.ErrorContains(t, err, "boom")
}
