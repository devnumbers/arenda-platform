// Package application holds the tasks use cases and ports: the rule CRUD,
// the task completion use cases and the property-wide listings, and the
// materialization tick run (ADR 0051). The context is modelled on Payments
// (ADR 0049 §3) without the payments-specific machinery: no pauses, no
// end dates, no money — the rule generates tasks, the tick materializes
// them, and the journal of completed tasks survives rule deletion.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The application error vocabulary of the tasks use cases: the transport
// maps them onto the wire contract (400/403/404/409).
var (
	// ErrNotFound covers a missing property, a missing or foreign rule or
	// task and an actor without the view capability — the privacy 404.
	ErrNotFound = errors.New("tasks: not found")
	// ErrInvalidInput marks a command that violates the create/update
	// contract (empty title, an unknown repeat, time without date, a
	// backdated anchor, pagination out of bounds).
	ErrInvalidInput = errors.New("tasks: invalid input")
	// ErrForbidden marks an actor whose role grants the view capability but
	// not the mutation capability (viewer — the ADR 0028 matrix).
	ErrForbidden = errors.New("tasks: forbidden")
	// ErrArchivedProperty marks a mutation on an archived property — the
	// read-only state (ADR 0025 in the ADR 0049 revision).
	ErrArchivedProperty = errors.New("tasks: property is archived")
	// ErrAlreadyCompleted marks a completion of an already completed task —
	// the contract's 409 on a repeated complete.
	ErrAlreadyCompleted = errors.New("tasks: task already completed")
	// ErrNotCompleted marks «Отменить выполнение» on an active task.
	ErrNotCompleted = errors.New("tasks: task is not completed")
	// ErrRuleDeleted marks «Отменить выполнение» on a journal row whose rule
	// is gone: the completion is irreversible there (tasks/CONTEXT.md
	// «Выполненная задача» — только пока правило живо).
	ErrRuleDeleted = errors.New("tasks: task rule is deleted")
)

// PropertyRef is the tasks view of the property a use case targets: the data
// owner (the SQL scope, ADR 0028), the archived flag and the display name
// (the global listing's row projection, ticket #521). The properties context
// owns the entity; tasks never needs its rest.
type PropertyRef struct {
	OwnerID  uuid.UUID
	Archived bool
	Name     string
}

// PropertyStore resolves the property a tasks use case targets. The
// ForUpdate variant is the context's serialization point (the payments
// precedent, ADR 0049 §3): every mutation locks the property row before
// reading or writing a rule, so mutation-vs-tick, mutation-vs-mutation and
// archive-vs-mutation serialize on one point.
type PropertyStore interface {
	// Get loads the property reference without locking; ErrNotFound when no
	// such property exists.
	Get(ctx context.Context, propertyID uuid.UUID) (PropertyRef, error)
	// GetForUpdate loads the property reference with the row locked inside
	// the caller's transaction.
	GetForUpdate(ctx context.Context, propertyID uuid.UUID) (PropertyRef, error)
	WithTx(tx transaction.Tx) (PropertyStore, error)
}

// RuleStore is the persistence port of the task rules. Reads and writes are
// scoped by the data owner (ADR 0028) and the nested path rule→property is
// enforced in the queries themselves; the property-less cut (ADR 0052) is a
// separate explicit key — the slices never mix.
type RuleStore interface {
	// Get loads one rule; ErrNotFound when the id is unknown, belongs to
	// another owner or hangs on another property.
	Get(ctx context.Context, id, scope, propertyID uuid.UUID) (domain.TaskRule, error)
	// GetWithoutProperty loads one rule of the property-less cut; ErrNotFound
	// when the id is unknown, belongs to another owner or hangs on a property
	// (ADR 0052).
	GetWithoutProperty(ctx context.Context, id, scope uuid.UUID) (domain.TaskRule, error)
	// Create inserts a new rule (id and owner_id are app-side).
	Create(ctx context.Context, rule domain.TaskRule) error
	// Update writes the editable fields of the rule.
	Update(ctx context.Context, rule domain.TaskRule) error
	// Delete removes the rule row; the journal rows keep their snapshots
	// with rule_id set to NULL by the FK. The caller has already resolved
	// the fate of the rule's uncompleted tasks.
	Delete(ctx context.Context, id, scope uuid.UUID) error
	// DeleteNotDueUncompleted removes the rule's uncompleted tasks that have
	// not fallen due yet — the undated one and the strictly future ones
	// (date > today): the edit invalidation whose in-transaction tick then
	// stands the single future again with fresh snapshots. Already due and
	// completed tasks keep their frozen snapshots.
	DeleteNotDueUncompleted(ctx context.Context, ruleID uuid.UUID, today time.Time) error
	// DeleteUncompleted removes every uncompleted task of the rule —
	// planned, overdue, today's and the single future: the rule deletion
	// semantics (resolution #496 — всё невыполненное гасится физически).
	DeleteUncompleted(ctx context.Context, ruleID uuid.UUID) error
	WithTx(tx transaction.Tx) (RuleStore, error)
}

// TaskStore is the persistence port of the tasks (the occurrences and the
// completed journal). Reads and writes are scoped by the data owner and the
// nested property path lives in the queries themselves. The mutating methods
// must run inside the transaction holding the property serialization lock.
type TaskStore interface {
	// Get loads one task; ErrNotFound when the id is unknown, foreign or
	// hangs on another property.
	Get(ctx context.Context, id, scope, propertyID uuid.UUID) (domain.Task, error)
	// GetWithoutProperty loads one task of the property-less cut; ErrNotFound
	// when the id is unknown, belongs to another owner or hangs on a property
	// (ADR 0052).
	GetWithoutProperty(ctx context.Context, id, scope uuid.UUID) (domain.Task, error)
	// ListByProperty returns one page of the property's tasks — active
	// (uncompleted) or completed per the query — with the total count of the
	// same filter (the «Выполненные N» section header).
	ListByProperty(ctx context.Context, scope, propertyID uuid.UUID, q TasksListQuery) ([]domain.Task, int, error)
	// ListGlobal returns one page of the actor's visible merged feed
	// (ticket #521): their own tasks — bound and property-less — plus the
	// bound tasks of the properties they can view (ADR 0028, the merged
	// visibility ADR 0052 decision 3). The rows carry the bound property's
	// display name; Status is zero and stays the use case's to compute
	// against each row owner's moment.
	ListGlobal(ctx context.Context, actor uuid.UUID, q TasksListQuery) ([]GlobalTaskRow, int, error)
	// ListGlobalWithoutProperty returns one page of the property-less cut of
	// the actor's own book (ADR 0052) — the global listing's «без объекта»
	// filter. The same row contract as ListGlobal; PropertyName is always
	// empty there.
	ListGlobalWithoutProperty(ctx context.Context, actor uuid.UUID, q TasksListQuery) ([]GlobalTaskRow, int, error)
	// ListGlobalOfProperties returns one page of the listed properties'
	// tasks in the merged-feed visibility (ticket #547): the SQL cuts the
	// feed to t.property_id in the list; archived properties contribute
	// nothing (ADR 0025/решение 9). The same row contract as ListGlobal;
	// the use case has already proven every listed property's visibility
	// (the privacy 404 lives there). The withoutProperty flag extends the
	// cut with the actor's own property-less rows — the feed filter's
	// «Общие задачи» union (решение владельца 2026-09-07); false
	// reproduces the pure property cut of #547.
	ListGlobalOfProperties(
		ctx context.Context, actor uuid.UUID, propertyIDs []uuid.UUID, withoutProperty bool, q TasksListQuery,
	) ([]GlobalTaskRow, int, error)
	// Complete stamps the completion fact (completed_date = day) on the
	// still-active task; rows affected = 0 surfaces as ErrAlreadyCompleted.
	Complete(ctx context.Context, id, scope uuid.UUID, day time.Time) error
	// Uncomplete clears the completion fact; rows affected = 0 surfaces as
	// ErrNotCompleted.
	Uncomplete(ctx context.Context, id, scope uuid.UUID) error
	// DeleteCompletedJournal removes every completed task of the property —
	// the deleted-rule journal and the live rules' rows alike — the «Удалить
	// все выполненные» operation (resolution #497, ADR 0051 as amended
	// 2026-10-01). The horizon leg runs first: each live rule losing rows
	// gets its history horizon raised to max(deleted due)+1 (today stamps
	// the undated dormancy marker), so the cleared dates never materialize
	// again. It returns the number of removed rows.
	DeleteCompletedJournal(ctx context.Context, scope, propertyID uuid.UUID, today time.Time) (int64, error)
	// DeleteCompletedJournalOwnerBook is the book-wide twin (ticket #536):
	// every completed task of the owner's book across both slices in one
	// query — the bound rows on non-archived properties and the property-less
	// ones (ADR 0052); the archived properties' journals stay frozen (ADR
	// 0025) and other owners' books are untouched (owner-scope, ADR 0028).
	// The same horizon leg as the property-scoped variant. It returns the
	// removed rows' property anchors — one per removed row, uuid.Nil for the
	// property-less ones — for the use case to group the per-object journal
	// counts (ADR 0061 §3).
	DeleteCompletedJournalOwnerBook(ctx context.Context, scope uuid.UUID, today time.Time) ([]uuid.UUID, error)
	// ListUncompletedTaskIDs returns the rule's standing uncompleted tasks'
	// ids — the scheduling seam's in-transaction handover (issue #775):
	// read after the materialization tick has settled the rule's rows, so
	// the freshly materialized and the kept standing tasks alike travel to
	// notifications, which re-resolves each task's liveness and term itself.
	ListUncompletedTaskIDs(ctx context.Context, ruleID uuid.UUID) ([]uuid.UUID, error)
	WithTx(tx transaction.Tx) (TaskStore, error)
}

// MaterializedTaskNotifier is the scheduling seam to the notifications
// context (issue #775): the rule create/edit flows hand over the standing
// tasks' ids strictly after their materializing transaction commits, and
// notifications plans each one — a live timed task books its due-minute job
// at the term's instant (or publishes at once when the term already
// passed), everything else is a no-op. Best-effort by the grace-events
// canon: the call happens post-commit, a failure is logged and never fails
// the committed mutation. Consumer-declared (CODING_STANDARDS); the
// composition root wires the notifications adapter in.
type MaterializedTaskNotifier interface {
	// NotifyMaterializedTasks accepts the handed-over task ids. Errors
	// travel back joined for the log — the mutation itself is already
	// committed and stays untouched.
	NotifyMaterializedTasks(ctx context.Context, taskIDs []uuid.UUID) error
}

// Pagination bounds of the tasks lists: the wire default is a page of 100
// with a hard ceiling at 500.
const (
	DefaultTasksPageSize = 100
	MaxTasksPageSize     = 500
)

// TasksListQuery is the property listing request. Completed selects the
// bucket: false = the active tasks (the screen's main sections), true = the
// completed journal (the collapsible «Выполненные» section). The transport
// fills everything; PrepareTasksQuery applies the page-size default and the
// pagination bounds.
type TasksListQuery struct {
	Completed bool
	Limit     int
	Offset    int
}

// PrepareTasksQuery validates the listing request in place and applies the
// page-size default: a zero limit becomes the default page, anything out of
// range or a negative offset is ErrInvalidInput mapped to the contract's 400.
func PrepareTasksQuery(q *TasksListQuery) error {
	if q.Limit == 0 {
		q.Limit = DefaultTasksPageSize
	}
	if q.Limit < 1 || q.Limit > MaxTasksPageSize || q.Offset < 0 {
		return ErrInvalidInput
	}
	return nil
}

// TaskListItem pairs one task with its server-computed view status
// (CONTEXT.md «Просрочка», «Без срока»): the buckets are never stored — they
// are derived against the owner's current moment before the response leaves
// the use case; clients never need the owner's timezone. PropertyName is the
// global listing's row projection — the bound property's display name, empty
// on the property-scoped listings and the property-less slice.
type TaskListItem struct {
	Task         domain.Task
	Status       domain.TaskViewStatus
	PropertyName string
}

// GlobalTaskRow is one row of the global listings' read projection
// (ticket #521): the task plus the bound property's display name — the
// global screen's per-row label, empty on the property-less slice. The use
// case turns it into a TaskListItem by computing Status against the row
// owner's moment.
type GlobalTaskRow struct {
	Task         domain.Task
	PropertyName string
}

// GlobalTasksListQuery is the global listing request (ticket #521): the
// TasksListQuery pagination and bucket fields plus the property filter —
// PropertyIDs (one id — the tasks of that one property resolved through its
// view gate, ticket #521; several — the picker's multi-select over the
// merged feed, ticket #547), WithoutProperty (the property-less slice of
// the actor's own book) or their union — both set is the filter's «Общие
// задачи» plus objects (решение владельца 2026-09-07). Neither set lists
// the actor's visible merged feed.
type GlobalTasksListQuery struct {
	TasksListQuery
	PropertyIDs     []uuid.UUID
	WithoutProperty bool
}

// PrepareGlobalTasksQuery validates the global listing request in place:
// the pagination rules of PrepareTasksQuery. The property filter and the
// property-less flag combine freely — the union is the feed filter's
// «Общие задачи» + objects (решение владельца 2026-09-07).
func PrepareGlobalTasksQuery(q *GlobalTasksListQuery) error {
	return PrepareTasksQuery(&q.TasksListQuery)
}

// TasksPage is one listing page plus the total count of the same filter (the
// «Выполненные N» counter) and the data owner's today — the day boundary the
// client buckets «Сегодня»/«Завтра» sections against.
type TasksPage struct {
	Items []TaskListItem
	Total int
	Today time.Time
}

// OwnerSnapshot is the tick's read side in one interface fact: the owner's
// task rules — property-less and on non-archived properties (ADR 0052) —
// and the dedup keys of their existing tasks. How many queries build it is
// the adapter's business.
type OwnerSnapshot struct {
	Rules []domain.TaskRule
	// Existing keys the listed rules' tasks by rule: dated rows by due date
	// (the value is the completed flag), the undated row as a flag.
	Existing map[uuid.UUID]domain.TaskExistence
}

// TickStore is the persistence port of the materialization tick (ADR 0051):
// the serialization lock, the owner snapshot, and the application of a tick
// plan. Every method must run inside the tick's unit of work.
type TickStore interface {
	// LockOwnerProperties takes the tick's serialization point: FOR UPDATE on
	// the owner's active/maintenance property rows, ordered by id. Context
	// mutations take the same ordered set at the transaction's front (#546),
	// so update-vs-tick, archive-vs-tick, delete-vs-tick and
	// mutation-vs-mutation serialize on one lock order without deadlocks.
	LockOwnerProperties(ctx context.Context, ownerID uuid.UUID) error
	// LockOwner takes the property-less cut's serialization point (ADR 0052):
	// FOR UPDATE on the owner's users row. Owner-scope mutations hold it for
	// their whole transaction. The full tick takes it after the property
	// rows; bound-rule mutations never do — the slices' rules are disjoint
	// row sets, and crossed lock orders would invite deadlocks.
	LockOwner(ctx context.Context, ownerID uuid.UUID) error
	// LoadOwnerSnapshot returns the owner's task rules — the property-less
	// cut plus the rules on non-archived properties (ADR 0052) — with the
	// dedup keys of their existing tasks. The tick bodies pick their slice
	// by the rule's PropertyID.
	LoadOwnerSnapshot(ctx context.Context, ownerID uuid.UUID) (OwnerSnapshot, error)
	// ApplyTickPlan applies one rule's plan inside the caller's transaction:
	// the idempotent task inserts (due dates, the missing single future and
	// the undated task) and the future rebuild in one keep-or-none statement.
	// Ordering and the keep-or-none duality live here, not in the caller.
	ApplyTickPlan(ctx context.Context, rule domain.TaskRule, today time.Time, plan domain.TaskTickPlan) error
	WithTx(tx transaction.Tx) (TickStore, error)
}

// OwnerCalendar gives the data owner's calendar date (ADR 0048): "today" as
// the date in the property owner's timezone — users.timezone, NOT NULL with
// the Europe/Moscow default, IANA-validated on write — normalized to the
// domain's UTC-midnight convention. The tasks context shares the payments
// decision verbatim: a timezone change never rewrites history — it only
// shifts future day boundaries.
type OwnerCalendar interface {
	Today(ctx context.Context, ownerID uuid.UUID) (time.Time, error)
}

// OwnerMoment is the read-side counterpart of the calendar: the owner's
// current instant with seconds truncated to minutes (the overdue precision,
// resolution #496 — seconds never participate) plus the instant's calendar
// date under the module's UTC-midnight convention.
type OwnerMoment struct {
	// Now is the current instant in the owner's location, seconds truncated
	// to minutes (the overdue precision).
	Now time.Time
	// Today is Now's calendar date at UTC midnight (the date convention).
	Today time.Time
}

// OwnerClock resolves the data owner's current moment (the OwnerMoment
// contract) for the computed read-side views. Mutations and the tick need
// only the day boundary (OwnerCalendar); the listings need the moment.
type OwnerClock interface {
	Moment(ctx context.Context, ownerID uuid.UUID) (OwnerMoment, error)
}

// DateAtUTCMidnight is the module's date convention (ADR 0048 p.2): read
// the instant's calendar date in loc, then rebuild it at UTC midnight, so
// the result compares correctly against the UTC-midnight dates stored in
// DATE columns. The single home of the normalization both calendar paths —
// the owner lookup and the zone sweep — share.
func DateAtUTCMidnight(t time.Time, loc *time.Location) time.Time {
	zoned := t.In(loc)
	return time.Date(zoned.Year(), zoned.Month(), zoned.Day(), 0, 0, 0, 0, time.UTC)
}

// MomentAt truncates the instant to minutes in loc and pairs it with its
// calendar date — the adapter-side builder of OwnerMoment.
func MomentAt(now time.Time, loc *time.Location) OwnerMoment {
	local := now.In(loc).Truncate(time.Minute)
	return OwnerMoment{Now: local, Today: DateAtUTCMidnight(now, loc)}
}

// TickZone is one work item of the tick's hourly zone sweep (ADR 0048 p.3):
// a distinct owner timezone and the data owners in it that have task rules
// on active/maintenance properties. One "today" is computed per zone and
// every owner of the zone is materialized on it.
type TickZone struct {
	Timezone string
	Owners   []uuid.UUID
}

// TickZoneDirectory lists the hourly sweep targets of the materialization
// tick. Stateless by design: every run re-lists the zones — idempotent
// materialization makes the midnights-between runs no-ops, so no per-zone or
// per-owner tick state is kept.
type TickZoneDirectory interface {
	ListTickZones(ctx context.Context) ([]TickZone, error)
}
