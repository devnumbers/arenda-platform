package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the adapters to the consumer-declared ports.
var (
	_ application.TickStore         = (*TickStore)(nil)
	_ application.OwnerCalendar     = (*OwnerClock)(nil)
	_ application.OwnerClock        = (*OwnerClock)(nil)
	_ application.TickZoneDirectory = (*TickZoneDirectory)(nil)
)

// TickStore is the postgres adapter of the materialization tick port
// (ADR 0051).
type TickStore struct {
	db postgres.DBTX
}

// NewTickStore creates a tick store over the given connection or pool.
func NewTickStore(db postgres.DBTX) *TickStore {
	return &TickStore{db: db}
}

func (s *TickStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *TickStore) WithTx(tx transaction.Tx) (application.TickStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("tasks.TickStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewTickStore(dbtx), nil
}

// LockOwnerProperties takes the tick's serialization point: FOR UPDATE on
// the owner's active/maintenance property rows.
func (s *TickStore) LockOwnerProperties(ctx context.Context, ownerID uuid.UUID) error {
	if _, err := s.q().LockTaskOwnerProperties(ctx, pgconv.UUIDToPgtype(ownerID)); err != nil {
		return fmt.Errorf("lock owner tick properties: %w", err)
	}
	return nil
}

// LockOwner takes the property-less cut's serialization point (ADR 0052):
// FOR UPDATE on the owner's users row. Owner-scope mutations hold it for
// their whole transaction, so mutation-vs-tick on the property-less slice
// serializes on one point. A missing users row surfaces through the :one
// contract as pgx.ErrNoRows.
func (s *TickStore) LockOwner(ctx context.Context, ownerID uuid.UUID) error {
	if _, err := s.q().LockTaskOwner(ctx, pgconv.UUIDToPgtype(ownerID)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return fmt.Errorf("lock owner row: %w", err)
	}
	return nil
}

// LoadOwnerSnapshot returns the owner's task rules on non-archived
// properties with the dedup keys of their existing tasks — the tick's read
// side in one call.
func (s *TickStore) LoadOwnerSnapshot(ctx context.Context, ownerID uuid.UUID) (application.OwnerSnapshot, error) {
	ruleRows, err := s.q().ListTickTaskRulesByOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return application.OwnerSnapshot{}, fmt.Errorf("list tick task rules: %w", err)
	}
	snapshot := application.OwnerSnapshot{
		Rules:    make([]domain.TaskRule, 0, len(ruleRows)),
		Existing: make(map[uuid.UUID]domain.TaskExistence, len(ruleRows)),
	}
	ids := make([]uuid.UUID, 0, len(ruleRows))
	for _, row := range ruleRows {
		rule := mapRuleRow(taskRuleFields{
			ID:            pgconv.UUIDFromPgtype(row.ID),
			OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
			PropertyID:    row.PropertyID,
			Title:         row.Title,
			Comment:       row.Comment,
			DueDate:       row.DueDate,
			DueTime:       row.DueTime,
			Repeat:        row.Repeat,
			HistoryBefore: row.HistoryBefore,
		})
		snapshot.Rules = append(snapshot.Rules, rule)
		ids = append(ids, rule.ID)
	}
	if len(ids) == 0 {
		return snapshot, nil
	}
	keyRows, err := s.q().ListTickTaskKeys(ctx, pgconv.UUIDSliceToPgtype(ids))
	if err != nil {
		return application.OwnerSnapshot{}, fmt.Errorf("list tick task keys: %w", err)
	}
	for _, key := range keyRows {
		ruleID := pgconv.UUIDFromPgtype(key.RuleID)
		existing := snapshot.Existing[ruleID]
		if existing.Dated == nil {
			existing.Dated = make(map[time.Time]bool)
		}
		if key.DueDate.Valid {
			existing.Dated[pgconv.DateFromPgtype(key.DueDate)] = key.Completed
		} else {
			existing.Undated = true
		}
		snapshot.Existing[ruleID] = existing
	}
	return snapshot, nil
}

// ApplyTickPlan applies one rule's tick plan inside the caller's
// transaction: the idempotent task inserts (due dates, the missing single
// future, the undated task) and the future rebuild in one keep-or-none
// statement. The call order and the keep-or-none duality are this
// implementation's business.
func (s *TickStore) ApplyTickPlan(
	ctx context.Context, rule domain.TaskRule, today time.Time, plan domain.TaskTickPlan,
) error {
	for _, day := range plan.Materialize {
		if err := s.insertTask(ctx, rule, &day); err != nil {
			return err
		}
	}
	if plan.MaterializeUndated {
		if err := s.insertUndatedTask(ctx, rule); err != nil {
			return err
		}
	}
	if plan.InsertFuture != nil {
		if err := s.insertTask(ctx, rule, plan.InsertFuture); err != nil {
			return err
		}
	}
	var keep pgtype.Date
	if plan.KeepFuture != nil {
		keep = pgconv.DateToPgtype(*plan.KeepFuture)
	}
	if _, err := s.q().DeleteFutureTasksExcept(ctx, postgres.DeleteFutureTasksExceptParams{
		RuleID:  pgconv.UUIDToPgtype(rule.ID),
		DueDate: pgconv.DateToPgtype(today),
		Column3: keep,
	}); err != nil {
		return fmt.Errorf("rebuild future tasks of rule %s: %w", rule.ID, err)
	}
	return nil
}

// insertTask materializes one dated task of the rule; the insert is
// idempotent through the (rule_id, due_date) partial unique index.
func (s *TickStore) insertTask(ctx context.Context, rule domain.TaskRule, day *time.Time) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("mint task id: %w", err)
	}
	task := domain.NewMaterializedTask(rule, day)
	task.ID = id
	if task.RuleID == nil {
		return fmt.Errorf("materialize task %s of rule %s: rule id is required",
			day.Format(time.DateOnly), rule.ID)
	}
	if err := s.q().InsertMaterializedTask(ctx, postgres.InsertMaterializedTaskParams{
		ID:         pgconv.UUIDToPgtype(task.ID),
		OwnerID:    pgconv.UUIDToPgtype(task.OwnerID),
		PropertyID: pgconv.UUIDToPgtypePtr(task.PropertyID),
		RuleID:     pgconv.UUIDToPgtype(*task.RuleID),
		DueDate:    pgconv.DateToPgtype(*day),
		DueTime:    timeOfDayToPgtype(task.DueTime),
		Title:      task.Title,
		Comment:    pgconv.StringPtrToPgtype(task.Comment),
	}); err != nil {
		return fmt.Errorf("materialize task %s of rule %s: %w", day.Format(time.DateOnly), rule.ID, err)
	}
	return nil
}

// insertUndatedTask materializes the undated task of an undated rule;
// idempotent through the (rule_id) partial unique index over the undated
// rows.
func (s *TickStore) insertUndatedTask(ctx context.Context, rule domain.TaskRule) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("mint task id: %w", err)
	}
	task := domain.NewMaterializedTask(rule, nil)
	task.ID = id
	if task.RuleID == nil {
		return fmt.Errorf("materialize undated task of rule %s: rule id is required", rule.ID)
	}
	if err := s.q().InsertMaterializedUndatedTask(ctx, postgres.InsertMaterializedUndatedTaskParams{
		ID:         pgconv.UUIDToPgtype(task.ID),
		OwnerID:    pgconv.UUIDToPgtype(task.OwnerID),
		PropertyID: pgconv.UUIDToPgtypePtr(task.PropertyID),
		RuleID:     pgconv.UUIDToPgtype(*task.RuleID),
		Title:      task.Title,
		Comment:    pgconv.StringPtrToPgtype(task.Comment),
	}); err != nil {
		return fmt.Errorf("materialize undated task of rule %s: %w", rule.ID, err)
	}
	return nil
}

// OwnerClock is the postgres adapter of both time ports (ADR 0048):
// users.timezone, NOT NULL with the Europe/Moscow default, IANA-validated on
// write, plus the injected clock. Today serves the mutations and the tick
// (the day boundary); Moment serves the listings' computed buckets (the
// owner's current instant at minute precision). A row that fails to load as
// a location is a data integrity error, not a fallback case.
type OwnerClock struct {
	db    postgres.DBTX
	clock clock.Clock
}

// NewOwnerClock creates the owner time adapter over the given connection or
// pool.
func NewOwnerClock(db postgres.DBTX, clk clock.Clock) *OwnerClock {
	return &OwnerClock{db: db, clock: clk}
}

func (c *OwnerClock) q() *postgres.Queries {
	return postgres.New(c.db)
}

// Today returns the owner's calendar date at the clock's now: the date in
// the owner's timezone, rebuilt at UTC midnight so it compares correctly
// against the UTC-midnight dates stored in DATE columns (ADR 0048 p.2 —
// computed in Go, no AT TIME ZONE in SQL).
func (c *OwnerClock) Today(ctx context.Context, ownerID uuid.UUID) (time.Time, error) {
	moment, err := c.Moment(ctx, ownerID)
	if err != nil {
		return time.Time{}, err
	}
	return moment.Today, nil
}

// Moment returns the owner's current instant with seconds truncated to
// minutes (the overdue precision, resolution #496) paired with its calendar
// date.
func (c *OwnerClock) Moment(ctx context.Context, ownerID uuid.UUID) (application.OwnerMoment, error) {
	name, err := c.q().GetTaskOwnerTimezone(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.OwnerMoment{}, fmt.Errorf("load owner timezone: owner %s has no users row: %w", ownerID, err)
		}
		return application.OwnerMoment{}, fmt.Errorf("load owner timezone: %w", err)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return application.OwnerMoment{}, fmt.Errorf("load owner timezone %q as location: %w", name, err)
	}
	return application.MomentAt(c.clock.Now(), loc), nil
}

// TickZoneDirectory is the postgres adapter of the hourly sweep port
// (ADR 0048 p.3): the distinct owner timezones with task rules on
// active/maintenance properties, each with its owners. The grouping of the
// flat DISTINCT rows into zones is this adapter's business.
type TickZoneDirectory struct {
	db postgres.DBTX
}

// NewTickZoneDirectory creates a zone directory over the given connection or
// pool.
func NewTickZoneDirectory(db postgres.DBTX) *TickZoneDirectory {
	return &TickZoneDirectory{db: db}
}

// ListTickZones returns the sweep targets grouped by timezone, in the
// query's timezone order; owners within a zone keep the query's owner order.
func (d *TickZoneDirectory) ListTickZones(ctx context.Context) ([]application.TickZone, error) {
	rows, err := postgres.New(d.db).ListTaskTickZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tick zones: %w", err)
	}
	zones := make([]application.TickZone, 0, len(rows))
	for _, row := range rows {
		if len(zones) == 0 || zones[len(zones)-1].Timezone != row.Timezone {
			zones = append(zones, application.TickZone{Timezone: row.Timezone})
		}
		last := &zones[len(zones)-1]
		last.Owners = append(last.Owners, pgconv.UUIDFromPgtype(row.OwnerID))
	}
	return zones, nil
}
