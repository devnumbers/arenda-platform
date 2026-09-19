package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// The adapters satisfy the consumer-declared ports (CODING_STANDARDS).
var (
	_ application.ScanZoneDirectory = (*TaskScanStore)(nil)
	_ application.TaskOverdueSource = (*TaskScanStore)(nil)
)

// TaskScanStore answers the tasks scan's questions (#750) over the owning
// tables directly: the sweep targets' zones, the upcoming timed tasks, the
// zone's overdue ones and the due-minute job's reload. Read-only — the scan
// publishes through the pipeline, it writes nothing here.
type TaskScanStore struct {
	db postgres.DBTX
}

// NewTaskScanStore creates the scan source adapter over the pool.
func NewTaskScanStore(db postgres.DBTX) *TaskScanStore {
	return &TaskScanStore{db: db}
}

// ListScanZones lists the distinct owner timezones having active dated tasks
// — on non-archived properties or without a property — the sweep targets
// (ADR 0048 p.3).
func (s *TaskScanStore) ListScanZones(ctx context.Context) ([]application.ScanZone, error) {
	zones, err := postgres.New(s.db).ListTaskScanZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tasks scan zones: %w", err)
	}
	result := make([]application.ScanZone, 0, len(zones))
	for _, timezone := range zones {
		result = append(result, application.ScanZone{Timezone: timezone})
	}
	return result, nil
}

// ListScheduledTargets lists the active timed tasks whose term instant (in
// the owner's timezone) falls in the window (from, until] — the scheduled
// leg's booking list (issue #750).
func (s *TaskScanStore) ListScheduledTargets(
	ctx context.Context, from, until time.Time,
) ([]application.TaskScheduleTarget, error) {
	rows, err := postgres.New(s.db).ListTaskScheduledTargets(ctx, postgres.ListTaskScheduledTargetsParams{
		Column1: pgconv.TimePtrToPgtype(&from),
		Column2: pgconv.TimePtrToPgtype(&until),
	})
	if err != nil {
		return nil, fmt.Errorf("list scheduled tasks: %w", err)
	}
	targets := make([]application.TaskScheduleTarget, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, application.TaskScheduleTarget{
			TaskID: pgconv.UUIDFromPgtype(row.TaskID),
			DueAt:  pgconv.TimestamptzToTime(row.DueAt),
		})
	}
	return targets, nil
}

// ListOverdueTargets lists the zone's overdue tasks as of the sweep's
// instant: the timed ones by their term minute, the date-only ones strictly
// after the zone's day's end (решение #737, тип №4).
func (s *TaskScanStore) ListOverdueTargets(
	ctx context.Context, zone string, today, now time.Time,
) ([]application.TaskOverdueTarget, error) {
	rows, err := postgres.New(s.db).ListTaskOverdueTargets(ctx, postgres.ListTaskOverdueTargetsParams{
		Timezone: zone,
		Column2:  pgconv.TimePtrToPgtype(&now),
		Column3:  pgconv.DateToPgtype(today),
	})
	if err != nil {
		return nil, fmt.Errorf("list overdue tasks of zone %s: %w", zone, err)
	}
	targets := make([]application.TaskOverdueTarget, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, taskOverdueTargetFromRow(
			row.TaskID, row.Title, row.DueDate, row.DueTime,
			row.PropertyID, row.PropertyName, row.PropertyAddress, row.RuleID, row.OwnerID))
	}
	return targets, nil
}

// GetScheduledOverdueTask reloads one task at its due minute — the
// scheduled job's delivery-time resolution. A gone, completed, date-only or
// archived-property task is pgx.ErrNoRows here and answers live=false: the
// job finishes without publishing.
func (s *TaskScanStore) GetScheduledOverdueTask(ctx context.Context, taskID uuid.UUID) (application.TaskOverdueTarget, bool, error) {
	row, err := postgres.New(s.db).GetScheduledOverdueTask(ctx, pgconv.UUIDToPgtype(taskID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.TaskOverdueTarget{}, false, nil
		}
		return application.TaskOverdueTarget{}, false, fmt.Errorf("load scheduled task %s: %w", taskID, err)
	}
	target := taskOverdueTargetFromRow(
		row.TaskID, row.Title, row.DueDate, row.DueTime,
		row.PropertyID, row.PropertyName, row.PropertyAddress, row.RuleID, row.OwnerID)
	return target, true, nil
}

// ListActiveRecipients lists the property's active members' user ids — the
// event's recipients besides the owner (решение #737: «Просмотр» включён,
// suspended is not an active participant).
func (s *TaskScanStore) ListActiveRecipients(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := postgres.New(s.db).ListPropertyActiveRecipients(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return nil, fmt.Errorf("list property recipients %s: %w", propertyID, err)
	}
	return pgconv.UUIDSliceFromPgtype(ids), nil
}

// taskOverdueTargetFromRow maps a scan row onto the application target: the
// term's wall-clock minutes come from the TIME column, the optional
// property snapshot travels only when the task hangs on one.
func taskOverdueTargetFromRow(
	taskID pgtype.UUID, title string, dueDate pgtype.Date, dueTime pgtype.Time,
	propertyID pgtype.UUID, propertyName, propertyAddress pgtype.Text, ruleID, ownerID pgtype.UUID,
) application.TaskOverdueTarget {
	target := application.TaskOverdueTarget{
		TaskID:  pgconv.UUIDFromPgtype(taskID),
		Title:   title,
		DueDate: pgconv.DateFromPgtype(dueDate),
		OwnerID: pgconv.UUIDFromPgtype(ownerID),
		RuleID:  pgconv.UUIDFromPgtypePtr(ruleID),
	}
	if dueTime.Valid {
		minute := application.TaskDueTime(dueTime.Microseconds / 60_000_000)
		target.DueTime = &minute
	}
	if id := pgconv.UUIDFromPgtypePtr(propertyID); id != nil {
		target.PropertyID = id
		target.PropertyName = pgconv.TextToString(propertyName)
		target.PropertyAddress = pgconv.TextToString(propertyAddress)
	}
	return target
}
