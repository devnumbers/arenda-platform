package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
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
			return mutationOutcome[int64]{
				Response:    cleared,
				Audit:       auditdomain.ActionTaskCompletedCleared,
				AuditEntity: auditdomain.EntityTask,
				AuditCtx:    map[string]any{"count": cleared},
			}, nil
		})
	return cleared, err
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
