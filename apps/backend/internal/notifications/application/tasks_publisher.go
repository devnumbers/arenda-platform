package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// scheduledHorizon bounds the scheduled leg's booking window (issue #750):
// the hourly pass books due-minute jobs for the timed tasks whose term falls
// into the next two days. The term further out joins on a later pass — the
// materialization tick keeps the rows standing long before their minute —
// and a term already passed inside the window is the overdue sweep's
// business.
const scheduledHorizon = 48 * time.Hour

// TaskScheduleTarget is one upcoming timed task the scheduled leg books
// (issue #750): the task id and the term's instant in the owner's timezone —
// the job's ScheduledAt, the «в минуту срока» of решение #737.
type TaskScheduleTarget struct {
	TaskID uuid.UUID
	DueAt  time.Time
}

// TaskDueTime is a wall-clock time at minute precision — minutes since
// midnight. The tasks context's TimeOfDay, kept as the publisher's own copy
// across the context boundary: seconds never enter the system, and the
// overdue copy names the term «Срок был: …, 15:13».
type TaskDueTime int

// String returns the HH:MM contract form.
func (t TaskDueTime) String() string {
	return fmt.Sprintf("%02d:%02d", int(t)/60, int(t)%60)
}

// TaskOverdueTarget is one overdue task the tasks publisher fires for: an
// active task whose term has passed in the owner's timezone (issue #750),
// with the property snapshot the publication carries (решение владельца
// 19.09.2026, #745 — the address line travels in the snapshot). A nil
// PropertyID is the task without a property (ADR 0052).
type TaskOverdueTarget struct {
	// TaskID is the payload link and the dedup key's whole entity half
	// (решение #737, тип №4: дедуп по task_id — one task, one overdue
	// notification).
	TaskID uuid.UUID
	Title  string
	// DueDate is the term's calendar date (DATE) and DueTime — the term's
	// wall-clock minute; nil means the date-only task whose copy names the
	// day alone.
	DueDate time.Time
	DueTime *TaskDueTime
	// RuleID is the producing rule's id — the screen the «переход к задаче»
	// action lands on (an active task's edit screen is its rule's screen);
	// an active task always has one (deleting the rule removes the
	// uncompleted tasks).
	RuleID          *uuid.UUID
	PropertyID      *uuid.UUID
	PropertyName    string
	PropertyAddress string
	OwnerID         uuid.UUID
	// DueAt is the term's instant in the owner's timezone — the due-minute
	// job's ScheduledAt. Only GetScheduledOverdueTask fills it (the
	// due-minute job's reload and the creation/edit seam's term-vs-now
	// decision, issue #775); the zone sweep's targets carry the term as the
	// date + wall-clock pair above and leave it zero.
	DueAt time.Time
}

// TaskOverdueSource is the tasks publisher's window into the tasks context:
// the upcoming timed tasks, the zone's overdue ones and the scheduled job's
// reload. Consumer-declared (CODING_STANDARDS), answered over the owning
// tables by the notifications postgres adapter.
type TaskOverdueSource interface {
	// ListScheduledTargets lists the active timed tasks whose term instant
	// (in the owner's timezone) falls in the window (from, until] — the
	// scheduled leg's booking list.
	ListScheduledTargets(ctx context.Context, from, until time.Time) ([]TaskScheduleTarget, error)
	// ListOverdueTargets lists the zone's active tasks whose term has passed
	// as of the sweep's instant: the timed ones by the term's minute (в
	// минуту срока, включительно), the date-only ones strictly after the
	// zone's day's end (первый скан после границы суток).
	ListOverdueTargets(ctx context.Context, zone string, today, now time.Time) ([]TaskOverdueTarget, error)
	// GetScheduledOverdueTask reloads one task at its due minute — the
	// scheduled job's delivery-time resolution. The live flag is false for a
	// task gone (the rule edit removes stale rows), completed, or turned
	// date-only: the job finishes without publishing.
	GetScheduledOverdueTask(ctx context.Context, taskID uuid.UUID) (TaskOverdueTarget, bool, error)
	// ListActiveRecipients lists the user ids of the property's active
	// members — the recipients besides the owner, «Просмотр» included
	// (решение #737: получатели объектных событий).
	ListActiveRecipients(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error)
}

// TaskOverdueScheduler books a task's due-minute job on the delivery queue
// (issue #750). Booking is idempotent — the queue's unique key keeps one
// in-flight job per task — so the hourly passes repeat their asks freely.
type TaskOverdueScheduler interface {
	ScheduleTaskOverdue(ctx context.Context, taskID uuid.UUID, dueAt time.Time) error
}

// TaskOverdueDeliverer is the scheduled jobs' call into the publisher: the
// due-minute worker reloads the task through the source and publishes.
type TaskOverdueDeliverer interface {
	DeliverTaskOverdue(ctx context.Context, taskID uuid.UUID) error
}

// TasksPublisher is the task events' publisher (issue #750) on the delivery
// pipeline (карта #734, #740): a task's term has passed — «Задача
// просрочена» goes to the owner and the active members, or to the owner
// alone for the task without a property. The hourly sweep has two legs: the
// scheduled one books the timed tasks' due-minute jobs — minute precision
// the hourly cadence cannot give (решение #737) — and the overdue one
// sweeps the zones as the backstop for everything the jobs missed (a pass
// delayed past a term, a term the booking window has not reached yet, and
// the date-only tasks whose notification lands on the first sweep after the
// day's end). A task without a term never fires (решение #737); the
// «сегодня» reminder does not exist.
//
// Idempotence is carried by the dedup key — `task_overdue:<task_id>`, the
// task id alone (решение #737, тип №4) — instead of sweep state: whichever
// leg fires first writes the row, the second one inserts nothing. A term
// moved by a rule edit removes the task and materializes a new one, whose
// id makes a fresh key.
type TasksPublisher struct {
	pipeline  *Publisher
	zones     ScanZoneDirectory
	source    TaskOverdueSource
	scheduler TaskOverdueScheduler
}

// NewTasksPublisher builds the tasks publisher over the pipeline creation
// service, the zone directory, the scan source and the job scheduler.
func NewTasksPublisher(
	pipeline *Publisher, zones ScanZoneDirectory, source TaskOverdueSource, scheduler TaskOverdueScheduler,
) *TasksPublisher {
	return &TasksPublisher{pipeline: pipeline, zones: zones, source: source, scheduler: scheduler}
}

// RunZoneScans is the hourly sweep: book the upcoming timed tasks'
// due-minute jobs first — every hour counts down to their minute — then
// sweep the zones for the already-overdue tasks. Failures are isolated — a
// broken booking, zone or task does not stop the rest; the joined error
// reports everything that failed.
func (p *TasksPublisher) RunZoneScans(ctx context.Context, now time.Time) error {
	if p.zones == nil {
		return errors.New("notifications tasks scan: zone directory must be configured")
	}
	errs := append([]error{}, p.scheduleUpcoming(ctx, now)...)
	if err := p.sweepOverdue(ctx, now); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// scheduleUpcoming books the due-minute jobs of the timed tasks whose term
// falls into the horizon window. A broken task's booking is isolated — the
// rest of the window books on.
func (p *TasksPublisher) scheduleUpcoming(ctx context.Context, now time.Time) []error {
	targets, err := p.source.ListScheduledTargets(ctx, now, now.Add(scheduledHorizon))
	if err != nil {
		return []error{fmt.Errorf("list scheduled tasks: %w", err)}
	}
	var errs []error
	for _, target := range targets {
		if err := p.scheduler.ScheduleTaskOverdue(ctx, target.TaskID, target.DueAt); err != nil {
			errs = append(errs, fmt.Errorf("schedule task %s: %w", target.TaskID, err))
		}
	}
	return errs
}

// sweepOverdue is the zone sweep (ADR 0048 p.3): one today per owner
// timezone, every overdue task published.
func (p *TasksPublisher) sweepOverdue(ctx context.Context, now time.Time) error {
	zones, err := p.zones.ListScanZones(ctx)
	if err != nil {
		return fmt.Errorf("list tasks scan zones: %w", err)
	}
	var errs []error
	for _, zone := range zones {
		today, err := zoneToday(now, zone.Timezone)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		targets, err := p.source.ListOverdueTargets(ctx, zone.Timezone, today, now)
		if err != nil {
			errs = append(errs, fmt.Errorf("list overdue tasks of zone %s: %w", zone.Timezone, err))
			continue
		}
		for _, target := range targets {
			if err := p.publish(ctx, target); err != nil {
				errs = append(errs, fmt.Errorf("publish task %s of zone %s: %w",
					target.TaskID, zone.Timezone, err))
			}
		}
	}
	return errors.Join(errs...)
}

// DeliverTaskOverdue is the due-minute job's half: reload the task through
// the source and publish if it still lives overdue. A false live flag — the
// task is gone, completed, or turned date-only — finishes the job without
// publishing; an error is returned to River for its retry ladder.
func (p *TasksPublisher) DeliverTaskOverdue(ctx context.Context, taskID uuid.UUID) error {
	target, live, err := p.source.GetScheduledOverdueTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("load task %s for the due-minute job: %w", taskID, err)
	}
	if !live {
		return nil
	}
	return p.publish(ctx, target)
}

// NotifyMaterializedTasks is the creation/edit seam (issue #775): the tasks
// context hands over the standing tasks' ids strictly after their
// materializing transaction committed. Per task the due-minute job's own
// reload decides: a live timed task books its job at the term's instant —
// or, when the term already passed (the task was born overdue), publishes
// at once, the wait for the next sweep being exactly the up-to-an-hour delay
// this seam removes; a gone, completed or date-only task is a no-op (the
// sweep leg covers the date-only ones as before). Failures are isolated per
// task and travel back joined — the caller is best-effort and only logs
// them.
func (p *TasksPublisher) NotifyMaterializedTasks(
	ctx context.Context, taskIDs []uuid.UUID, now time.Time,
) error {
	var errs []error
	for _, taskID := range taskIDs {
		if err := p.notifyMaterialized(ctx, taskID, now); err != nil {
			errs = append(errs, fmt.Errorf("notify task %s: %w", taskID, err))
		}
	}
	return errors.Join(errs...)
}

// notifyMaterialized plans one handed-over task: the live reload answers the
// target with its term instant; a term in the future books the due-minute
// job, a reached one — the boundary inclusive, the canon minute — publishes
// now.
func (p *TasksPublisher) notifyMaterialized(ctx context.Context, taskID uuid.UUID, now time.Time) error {
	target, live, err := p.source.GetScheduledOverdueTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("load task %s for the scheduling seam: %w", taskID, err)
	}
	if !live {
		return nil
	}
	if target.DueAt.After(now) {
		return p.scheduler.ScheduleTaskOverdue(ctx, taskID, target.DueAt)
	}
	return p.publish(ctx, target)
}

// publish fans one task's event out: to the owner and the active members
// for a property-bound task, to the owner alone for the task without a
// property (решение #737: безобъектная — владельца). The dedup key is the
// task id alone; a system scan and a scheduled job have no initiator — the
// actor is nobody, nobody is skipped.
func (p *TasksPublisher) publish(ctx context.Context, target TaskOverdueTarget) error {
	recipients := make([]uuid.UUID, 0, 1)
	recipients = append(recipients, target.OwnerID)
	var property *domain.EntityRef
	if target.PropertyID != nil {
		members, err := p.source.ListActiveRecipients(ctx, *target.PropertyID)
		if err != nil {
			return fmt.Errorf("list recipients of property %s: %w", *target.PropertyID, err)
		}
		recipients = append(recipients, members...)
		property = &domain.EntityRef{
			ID:      *target.PropertyID,
			Name:    target.PropertyName,
			Address: target.PropertyAddress,
		}
	}

	taskID := target.TaskID
	return p.pipeline.Publish(ctx, Publication{
		EventType: domain.EventTaskOverdue,
		DedupKey:  domain.DedupKey(taskOverdueDedupKey(target.TaskID)),
		Title:     "Задача просрочена",
		Body:      taskOverdueBody(target),
		// The line above the title is the property the task hangs on — the
		// feed's context label vocabulary (решение #737); the task without a
		// property renders without one (макет).
		ContextLabel: target.PropertyName,
		Payload: domain.Payload{
			Property:   property,
			TaskID:     &taskID,
			TaskRuleID: target.RuleID,
		},
		Recipients: recipients,
	})
}

// taskOverdueBody renders the catalog's body template (решение #737,
// verbatim): the task's name, the property line only when the task hangs on
// one, and the wall-clock «Срок был» — the day, plus the minute for the
// timed task.
func taskOverdueBody(target TaskOverdueTarget) string {
	name := fmt.Sprintf("Задача «%s»", target.Title)
	due := formatDayMonth(target.DueDate)
	if target.DueTime != nil {
		due += ", " + target.DueTime.String()
	}
	if target.PropertyID != nil {
		return fmt.Sprintf("%s по объекту «%s» просрочена. Срок был: %s", name, target.PropertyName, due)
	}
	return fmt.Sprintf("%s просрочена. Срок был: %s", name, due)
}

// taskOverdueDedupKey builds the tasks publisher's dedup key (словарь
// издателей, CONTEXT.md): the task id alone (решение #737, тип №4) — one
// task, one overdue notification, whichever leg fires first.
func taskOverdueDedupKey(taskID uuid.UUID) string {
	return "task_overdue:" + taskID.String()
}
