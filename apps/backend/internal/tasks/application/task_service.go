package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

// TaskService carries the task use cases (ADR 0051): the property-wide
// listings with the server-computed view buckets, the completion toggle and
// the «Удалить все выполненные» journal clear. It runs through the same
// serialization and read-scope discipline as the rule service.
type TaskService struct {
	txStoreFactory
	policy    sharedpolicy.Policy
	calendar  OwnerCalendar
	clock     OwnerClock
	writeGate gateFunc // Full Access and Owner: complete/uncomplete/clear (resolution #496).
}

// NewTaskService builds the task use cases over the shared transactional
// store factory, the owner calendar (mutations' day boundary), the owner
// clock (the listings' computed buckets) and the authorization policy. A nil
// calendar or clock fails on first use rather than computing statuses
// against a zero time.
func NewTaskService(
	factory txStoreFactory, calendar OwnerCalendar, clock OwnerClock, policy sharedpolicy.Policy,
) *TaskService {
	return &TaskService{
		txStoreFactory: factory,
		policy:         policy,
		calendar:       calendar,
		clock:          clock,
		writeGate:      newCapabilityGate(policy, sharedpolicy.CanEdit),
	}
}

// conveyor bundles this service's factory and calendar for the shared
// mutation conveyor.
func (s *TaskService) conveyor() mutationGates {
	return mutationGates{factory: s.txStoreFactory, calendar: s.calendar}
}

// ListTasks returns one page of the property's tasks — the active ones or
// the completed journal per the query — each with its server-computed view
// bucket derived against the owner's current moment (minute precision,
// resolution #496); the client never needs the owner's timezone. The page
// carries the total count of the same filter (the «Выполненные N» counter)
// and the owner's today — the day boundary for the «Сегодня»/«Завтра»
// sections. Any actor with the view capability may read; a stranger gets
// ErrNotFound. Reads never tick.
func (s *TaskService) ListTasks(
	ctx context.Context, actor, propertyID uuid.UUID, q TasksListQuery,
) (TasksPage, error) {
	scope, err := resolveReadScope(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return TasksPage{}, err
	}
	if err := PrepareTasksQuery(&q); err != nil {
		return TasksPage{}, err
	}
	moment, err := s.ownerMoment(ctx, scope)
	if err != nil {
		return TasksPage{}, err
	}
	tasks, total, err := s.tasks.ListByProperty(ctx, scope, propertyID, q)
	if err != nil {
		return TasksPage{}, fmt.Errorf("list tasks: %w", err)
	}
	items := make([]TaskListItem, len(tasks))
	for i, task := range tasks {
		items[i] = TaskListItem{Task: task, Status: domain.ViewStatus(task, moment.Now)}
	}
	return TasksPage{Items: items, Total: total, Today: moment.Today}, nil
}

// CompleteTask implements «Выполнить» (POST …/tasks/{id}/complete): the
// active task gets the completion fact stamped with today in the owner's
// timezone; a repeated completion is ErrAlreadyCompleted. The verdict keeps
// Tick=true — the same-tick discipline of every mutation (resolution #496);
// for a completed task the tick is a no-op by the dedup keys. Full Access
// and Owner may complete; a viewer gets ErrForbidden, a stranger or a
// foreign task the privacy ErrNotFound.
func (s *TaskService) CompleteTask(
	ctx context.Context, actor, propertyID, taskID uuid.UUID,
) (domain.Task, error) {
	conveyor := s.conveyor()
	return runMutation(conveyor, ctx, actor, propertyID, uuid.Nil, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.TaskRule, today time.Time,
		) (mutationOutcome[domain.Task], error) {
			task, err := stores.tasks.Get(ctx, taskID, scope, propertyID)
			if err != nil {
				return mutationOutcome[domain.Task]{}, err
			}
			if task.CompletedDate != nil {
				return mutationOutcome[domain.Task]{}, ErrAlreadyCompleted
			}
			if err := stores.tasks.Complete(ctx, taskID, scope, today); err != nil {
				return mutationOutcome[domain.Task]{}, err
			}
			completed := today
			task.CompletedDate = &completed
			history := historydomain.TaskCompleted(ruleLinkID(task.RuleID), task.Title, taskDueDate(task))
			return mutationOutcome[domain.Task]{
				Response:      task,
				Audit:         auditdomain.ActionTaskCompleted,
				AuditEntity:   auditdomain.EntityTask,
				AuditEntityID: &task.ID,
				AuditCtx:      auditRuleCtx(task.RuleID),
				History:       new(history),
				Tick:          true,
			}, nil
		})
}

// UncompleteTask implements «Отменить выполнение» (POST
// …/tasks/{id}/uncomplete): the completion fact is cleared and the task
// returns to the active ones; the rule's schedule is untouched. Possible
// only while the rule lives (tasks/CONTEXT.md «Выполненная задача») — a
// journal row of a deleted rule is ErrRuleDeleted; an already active task
// is ErrNotCompleted. Full Access and Owner may uncomplete.
func (s *TaskService) UncompleteTask(
	ctx context.Context, actor, propertyID, taskID uuid.UUID,
) (domain.Task, error) {
	conveyor := s.conveyor()
	return runMutation(conveyor, ctx, actor, propertyID, uuid.Nil, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.TaskRule, _ time.Time,
		) (mutationOutcome[domain.Task], error) {
			task, err := stores.tasks.Get(ctx, taskID, scope, propertyID)
			if err != nil {
				return mutationOutcome[domain.Task]{}, err
			}
			if task.CompletedDate == nil {
				return mutationOutcome[domain.Task]{}, ErrNotCompleted
			}
			if task.RuleID == nil {
				return mutationOutcome[domain.Task]{}, ErrRuleDeleted
			}
			if err := stores.tasks.Uncomplete(ctx, taskID, scope); err != nil {
				return mutationOutcome[domain.Task]{}, err
			}
			task.CompletedDate = nil
			return mutationOutcome[domain.Task]{
				Response:      task,
				Audit:         auditdomain.ActionTaskUncompleted,
				AuditEntity:   auditdomain.EntityTask,
				AuditEntityID: &task.ID,
				AuditCtx:      auditRuleCtx(task.RuleID),
				History:       new(historydomain.TaskUncompleted(ruleLinkID(task.RuleID), task.Title)),
				Tick:          true,
			}, nil
		})
}

// ClearCompletedJournal implements «Удалить все выполненные» (DELETE
// …/tasks/completed, resolution #497): the completed tasks of the property's
// deleted rules (rule_id IS NULL) are removed forever — rules and active
// tasks are untouched. The completed tasks of live rules stay: they hold the
// tick's dedup keys, and clearing them would make the next tick
// re-materialize the rule's whole past as active overdue tasks (ADR 0051
// documents the scoping). The verdict states Tick=false: no rule data
// changed, the tick is a no-op by construction.
func (s *TaskService) ClearCompletedJournal(
	ctx context.Context, actor, propertyID uuid.UUID,
) (int64, error) {
	conveyor := s.conveyor()
	cleared, err := runMutation(conveyor, ctx, actor, propertyID, uuid.Nil, s.writeGate,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.TaskRule, _ time.Time,
		) (mutationOutcome[int64], error) {
			cleared, err := stores.tasks.DeleteCompletedJournal(ctx, scope, propertyID)
			if err != nil {
				return mutationOutcome[int64]{}, fmt.Errorf("clear completed journal: %w", err)
			}
			// An empty cleanup is no action: the journal row would read
			// «удалены выполненные задачи: 0».
			var history *historydomain.Entry
			if cleared > 0 {
				history = new(historydomain.TaskCompletedCleared(int(cleared)))
			}
			return mutationOutcome[int64]{
				Response:    cleared,
				Audit:       auditdomain.ActionTaskCompletedCleared,
				AuditEntity: auditdomain.EntityTask,
				AuditCtx:    map[string]any{"count": cleared},
				History:     history,
			}, nil
		})
	return cleared, err
}

// ClearCompletedJournalOwnerBook implements «Удалить все выполненные» on the
// global «Задачи» screen (DELETE /tasks/completed, ticket #536): the
// book-wide twin of ClearCompletedJournal. The completed tasks of the
// actor's deleted rules are removed forever across their own book — the
// bound rows and the property-less ones (ADR 0052) in the same store query;
// the shared-to properties' journals are other owners' books (owner-scope,
// ADR 0028) and the archived ones stay frozen (ADR 0025). The same live-rule
// protection as the property-scoped clear (ADR 0051). The journal follows
// the bulk canon (ADR 0061 §3): one row per touched object with its removed
// count, the property-less legs anchor none. Serialized on the owner's users
// row — the book-level anchor (ADR 0052); the verdict states Tick=false: no
// rule data changed, the tick is a no-op by construction.
func (s *TaskService) ClearCompletedJournalOwnerBook(
	ctx context.Context, actor uuid.UUID,
) (int64, error) {
	return runOwnerMutation(s.conveyor(), ctx, actor, uuid.Nil,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.TaskRule, _ time.Time,
		) (mutationOutcome[int64], error) {
			removed, err := stores.tasks.DeleteCompletedJournalOwnerBook(ctx, scope)
			if err != nil {
				return mutationOutcome[int64]{}, fmt.Errorf("clear completed journal of owner book: %w", err)
			}
			return mutationOutcome[int64]{
				Response:          int64(len(removed)),
				Audit:             auditdomain.ActionTaskCompletedCleared,
				AuditEntity:       auditdomain.EntityTask,
				AuditCtx:          map[string]any{"count": int64(len(removed))},
				HistoryByProperty: clearedJournalByProperty(removed),
			}, nil
		})
}

// clearedJournalByProperty groups the removed rows' anchors into the bulk
// journal rows — one task.completed_cleared entry per touched object with
// its removed count (ADR 0061 §3); the property-less legs (uuid.Nil) anchor
// no row — every journal row anchors to a property.
func clearedJournalByProperty(removed []uuid.UUID) map[uuid.UUID]historydomain.Entry {
	counts := make(map[uuid.UUID]int)
	for _, propertyID := range removed {
		if propertyID == uuid.Nil {
			continue
		}
		counts[propertyID]++
	}
	rows := make(map[uuid.UUID]historydomain.Entry, len(counts))
	for propertyID, count := range counts {
		rows[propertyID] = historydomain.TaskCompletedCleared(count)
	}
	return rows
}

// ListGlobalTasks returns one page of the actor's visible tasks for the
// global «Задачи» screen (ticket #521). Filter states: one listed property —
// resolved through its view gate (a stranger gets the privacy ErrNotFound)
// and read in the data owner's scope; several listed properties — the merged
// feed cut to the list, every id proven visible first (one invisible id is
// the privacy ErrNotFound of the whole request, ticket #547); the
// property-less slice — the actor's own book only (ADR 0052); the union of
// the listed properties and the property-less flag — the feed filter's
// «Общие задачи» + objects (решение владельца 2026-09-07, the same proof
// first); no filter — the merged feed: their own tasks plus the bound tasks
// of the properties they can view (ADR 0028), the visibility predicate
// living in the store's SQL. Every item's view bucket is computed against
// its data owner's current moment (CONTEXT.md «Просрочка» — a merged page
// may span owners); the page's today is the reading actor's calendar date
// on the merged branches, the data owner's on the single-property branch.
// Reads never tick.
func (s *TaskService) ListGlobalTasks(
	ctx context.Context, actor uuid.UUID, q GlobalTasksListQuery,
) (TasksPage, error) {
	if err := PrepareGlobalTasksQuery(&q); err != nil {
		return TasksPage{}, err
	}
	switch {
	case len(q.PropertyIDs) > 0 && q.WithoutProperty:
		// Union «Общие задачи» + объекты (решение владельца 2026-09-07).
		return s.listGlobalOfPropertiesSlice(ctx, actor, q, true)
	case len(q.PropertyIDs) == 1:
		return s.listGlobalOfProperty(ctx, actor, q.PropertyIDs[0], q.TasksListQuery)
	case len(q.PropertyIDs) > 1:
		return s.listGlobalOfPropertiesSlice(ctx, actor, q, false)
	case q.WithoutProperty:
		return s.listGlobalSlice(ctx, actor, q.TasksListQuery, s.tasks.ListGlobalWithoutProperty)
	default:
		return s.listGlobalSlice(ctx, actor, q.TasksListQuery, s.tasks.ListGlobal)
	}
}

// proveGlobalPropertiesVisible runs every listed property through its view
// gate before the store is touched (ticket #547): one invisible or unknown
// id is the privacy ErrNotFound of the whole request — the same discipline
// as the single-property branch, so a mixed list never leaks which ids
// exist.
func (s *TaskService) proveGlobalPropertiesVisible(
	ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID,
) error {
	for _, propertyID := range propertyIDs {
		if _, err := resolveReadScopeRef(ctx, s.policy, s.properties, actor, propertyID); err != nil {
			return err
		}
	}
	return nil
}

// listGlobalOfPropertiesSlice is the shared body of the multi-property and
// union branches of the global listing (ticket #547, «Общие задачи» —
// решение владельца 2026-09-07): every listed property is proven visible
// before the store is touched, then the cut (optionally unioned with the
// reader's own property-less rows) is read in one merged page.
func (s *TaskService) listGlobalOfPropertiesSlice(
	ctx context.Context, actor uuid.UUID, q GlobalTasksListQuery, withoutProperty bool,
) (TasksPage, error) {
	if err := s.proveGlobalPropertiesVisible(ctx, actor, q.PropertyIDs); err != nil {
		return TasksPage{}, err
	}
	ids := q.PropertyIDs
	return s.listGlobalSlice(ctx, actor, q.TasksListQuery,
		func(ctx context.Context, actor uuid.UUID, q TasksListQuery) ([]GlobalTaskRow, int, error) {
			return s.tasks.ListGlobalOfProperties(ctx, actor, ids, withoutProperty, q)
		})
}

// listGlobalOfProperty is the property-filtered branch of the global listing:
// the same read scope and store query as the property screen, plus the
// property's display name projected into every row.
func (s *TaskService) listGlobalOfProperty(
	ctx context.Context, actor, propertyID uuid.UUID, q TasksListQuery,
) (TasksPage, error) {
	scope, err := resolveReadScopeRef(ctx, s.policy, s.properties, actor, propertyID)
	if err != nil {
		return TasksPage{}, err
	}
	moment, err := s.ownerMoment(ctx, scope.OwnerID)
	if err != nil {
		return TasksPage{}, err
	}
	tasks, total, err := s.tasks.ListByProperty(ctx, scope.OwnerID, propertyID, q)
	if err != nil {
		return TasksPage{}, fmt.Errorf("list tasks of property %s: %w", propertyID, err)
	}
	items := make([]TaskListItem, len(tasks))
	for i, task := range tasks {
		items[i] = TaskListItem{
			Task:         task,
			Status:       domain.ViewStatus(task, moment.Now),
			PropertyName: scope.Name,
		}
	}
	return TasksPage{Items: items, Total: total, Today: moment.Today}, nil
}

// listGlobalSlice is the actor-scoped branch of the global listing — the
// merged feed and the property-less slice differ only in the store query.
// The rows may span several data owners, so each row's bucket is computed
// against its owner's moment, resolved once per owner per page; the actor's
// moment seeds the cache and drives the page's today.
func (s *TaskService) listGlobalSlice(
	ctx context.Context, actor uuid.UUID, q TasksListQuery,
	list func(ctx context.Context, actor uuid.UUID, q TasksListQuery) ([]GlobalTaskRow, int, error),
) (TasksPage, error) {
	actorMoment, err := s.ownerMoment(ctx, actor)
	if err != nil {
		return TasksPage{}, err
	}
	moments := map[uuid.UUID]OwnerMoment{actor: actorMoment}
	rows, total, err := list(ctx, actor, q)
	if err != nil {
		return TasksPage{}, fmt.Errorf("list global tasks: %w", err)
	}
	items := make([]TaskListItem, len(rows))
	for i, row := range rows {
		moment, ok := moments[row.Task.OwnerID]
		if !ok {
			moment, err = s.ownerMoment(ctx, row.Task.OwnerID)
			if err != nil {
				return TasksPage{}, err
			}
			moments[row.Task.OwnerID] = moment
		}
		items[i] = TaskListItem{
			Task:         row.Task,
			Status:       domain.ViewStatus(row.Task, moment.Now),
			PropertyName: row.PropertyName,
		}
	}
	return TasksPage{Items: items, Total: total, Today: actorMoment.Today}, nil
}

// CompleteTaskWithoutProperty implements «Выполнить» on the property-less
// slice (POST /tasks/{taskId}/complete, ADR 0052): the same completion fact
// as CompleteTask, stamped with today in the owner's timezone and serialized
// on the owner's users row. A stranger — or a bound task, the slices never
// mix — gets the privacy 404.
func (s *TaskService) CompleteTaskWithoutProperty(
	ctx context.Context, actor, taskID uuid.UUID,
) (domain.Task, error) {
	return runOwnerMutation(s.conveyor(), ctx, actor, uuid.Nil,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.TaskRule, today time.Time,
		) (mutationOutcome[domain.Task], error) {
			task, err := stores.tasks.GetWithoutProperty(ctx, taskID, scope)
			if err != nil {
				return mutationOutcome[domain.Task]{}, err
			}
			if task.CompletedDate != nil {
				return mutationOutcome[domain.Task]{}, ErrAlreadyCompleted
			}
			if err := stores.tasks.Complete(ctx, taskID, scope, today); err != nil {
				return mutationOutcome[domain.Task]{}, err
			}
			completed := today
			task.CompletedDate = &completed
			return mutationOutcome[domain.Task]{
				Response:      task,
				Audit:         auditdomain.ActionTaskCompleted,
				AuditEntity:   auditdomain.EntityTask,
				AuditEntityID: &task.ID,
				AuditCtx:      auditRuleCtx(task.RuleID),
				Tick:          true,
			}, nil
		})
}

// UncompleteTaskWithoutProperty implements «Отменить выполнение» on the
// property-less slice (POST /tasks/{taskId}/uncomplete, ADR 0052): the same
// semantics as UncompleteTask — possible only while the rule lives —
// serialized on the owner's users row.
func (s *TaskService) UncompleteTaskWithoutProperty(
	ctx context.Context, actor, taskID uuid.UUID,
) (domain.Task, error) {
	return runOwnerMutation(s.conveyor(), ctx, actor, uuid.Nil,
		func(
			ctx context.Context, stores *txStores, scope uuid.UUID, _ domain.TaskRule, _ time.Time,
		) (mutationOutcome[domain.Task], error) {
			task, err := stores.tasks.GetWithoutProperty(ctx, taskID, scope)
			if err != nil {
				return mutationOutcome[domain.Task]{}, err
			}
			if task.CompletedDate == nil {
				return mutationOutcome[domain.Task]{}, ErrNotCompleted
			}
			if task.RuleID == nil {
				return mutationOutcome[domain.Task]{}, ErrRuleDeleted
			}
			if err := stores.tasks.Uncomplete(ctx, taskID, scope); err != nil {
				return mutationOutcome[domain.Task]{}, err
			}
			task.CompletedDate = nil
			return mutationOutcome[domain.Task]{
				Response:      task,
				Audit:         auditdomain.ActionTaskUncompleted,
				AuditEntity:   auditdomain.EntityTask,
				AuditEntityID: &task.ID,
				AuditCtx:      auditRuleCtx(task.RuleID),
				Tick:          true,
			}, nil
		})
}

// ownerMoment resolves the data owner's current moment for the computed
// view buckets, failing loudly when the clock was never wired.
func (s *TaskService) ownerMoment(ctx context.Context, ownerID uuid.UUID) (OwnerMoment, error) {
	if s.clock == nil {
		return OwnerMoment{}, errors.New("tasks: owner clock must be configured")
	}
	moment, err := s.clock.Moment(ctx, ownerID)
	if err != nil {
		return OwnerMoment{}, fmt.Errorf("resolve owner moment: %w", err)
	}
	return moment, nil
}

// auditRuleCtx names the producing rule in the audit context when the task
// still hangs on a live one; a journal row has none.
func auditRuleCtx(ruleID *uuid.UUID) map[string]any {
	if ruleID == nil {
		return nil
	}
	return map[string]any{"rule_id": *ruleID}
}

// ruleLinkID dereferences the occurrence's rule binding for the journal
// link (#713): the only task page is the rule's edit screen, so the row
// links the rule; nil (the rule hard-deleted, resolution #496) yields
// uuid.Nil, which the builder's entityLink turns into no link.
func ruleLinkID(ruleID *uuid.UUID) uuid.UUID {
	if ruleID == nil {
		return uuid.Nil
	}
	return *ruleID
}

// taskDueDate is the occurrence's due date for the row text; the undated
// task yields a zero time and the builder omits the term.
func taskDueDate(task domain.Task) time.Time {
	if task.DueDate == nil {
		return time.Time{}
	}
	return *task.DueDate
}
