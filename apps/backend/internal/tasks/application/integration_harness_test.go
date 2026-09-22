//go:build integration

package application_test

// The single integration harness of the tasks context tests: the tick
// service, the rule use case service and the task use case service over one
// store factory (mirroring the wire), a mutable fake clock, an injectable
// policy and the shared seeding and projection helpers (the payments
// harness pattern, ADR 0051).

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	taskspg "github.com/nambers/arenda-planform/apps/backend/internal/tasks/adapters/postgres"
	tasksapp "github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

// tickBaseTime anchors the fake clock: 2026-09-10 12:00 UTC — Thursday,
// 15:00 in Moscow and already the 11th (00:00) in Kamchatka: the TZ split
// the clock tests rely on.
var tickBaseTime = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

// Calendar fixtures: today (in Moscow) is 2026-09-10, a Thursday.
const (
	day03        = "2026-09-03"
	day10        = "2026-09-10"
	day17        = "2026-09-17"
	day24        = "2026-09-24"
	aug27        = "2026-08-27"
	taskMoscowTZ = "Europe/Moscow"
)

// mutableClock is a fake clock.Clock whose Now can be advanced mid-test.
type mutableClock struct{ now time.Time }

func (c *mutableClock) Now() time.Time { return c.now }

// stubPropertyPolicy resolves every actor to one configured role — the tasks
// role-matrix double. The real membership policy is the access context's own
// seam; here only the role resolution contract matters.
type stubPropertyPolicy struct{ role sharedpolicy.Role }

func (p stubPropertyPolicy) Role(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleNone, nil
}

func (p stubPropertyPolicy) RoleForProperty(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return p.role, nil
}

// tasksHarness wires the whole tasks context — the tick service, the rule
// use cases and the task use cases over one store factory with the real
// audit recorder — to real PostgreSQL with a mutable clock and an injectable
// policy (testcontainers PostgreSQL 18 or TEST_DATABASE_URL).
type tasksHarness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	clock  *mutableClock
	tick   *tasksapp.TickService
	rules  *tasksapp.RuleService
	tasks  *tasksapp.TaskService
	seam   *fakeSeam
	owner  uuid.UUID
	propID uuid.UUID
}

func newTasksHarness(t *testing.T) *tasksHarness {
	t.Helper()
	return newTasksHarnessWithPolicy(t, nil)
}

func newTasksHarnessWithPolicy(t *testing.T, policy sharedpolicy.Policy) *tasksHarness {
	t.Helper()

	pool := testdb.Setup(t)
	clk := &mutableClock{now: tickBaseTime}
	logger := slog.New(slog.DiscardHandler)

	tickStore := taskspg.NewTickStore(pool)
	ruleStore := taskspg.NewRuleStore(pool)
	taskStore := taskspg.NewTaskStore(pool)
	propertyStore := taskspg.NewPropertyStore(pool)
	audit := auditapp.NewService(auditpg.NewWriter(pool), clk)
	uow := pgdb.NewUoW(pool, logger)
	ownerClock := taskspg.NewOwnerClock(pool, clk)
	zones := taskspg.NewTickZoneDirectory(pool)
	factory := tasksapp.NewTxStoreFactory(
		tickStore, ruleStore, taskStore, propertyStore, audit, uow,
	)

	return &tasksHarness{
		t:     t,
		pool:  pool,
		clock: clk,
		tick:  tasksapp.NewTickService(factory, zones, ownerClock, nil),
		rules: tasksapp.NewRuleService(factory, ownerClock, policy, logger),
		tasks: tasksapp.NewTaskService(factory, ownerClock, ownerClock, policy),
	}
}

// withSeam wires a recording scheduling-seam fake into the rule service —
// the scheduling-seam tests' door (issue #775); the other families keep the
// pre-#775 silence.
func (h *tasksHarness) withSeam(seam *fakeSeam) *tasksHarness {
	h.t.Helper()
	h.rules.SetOverdueSeam(seam)
	h.seam = seam
	return h
}

// fakeSeam records the handovers the rule flows make to the notifications
// scheduling seam and can fail on demand — the best-effort contract's
// double.
type fakeSeam struct {
	handovers [][]uuid.UUID
	err       error
}

func (f *fakeSeam) NotifyMaterializedTasks(_ context.Context, taskIDs []uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.handovers = append(f.handovers, taskIDs)
	return nil
}

// withOwner seeds a user (the data owner) with the given timezone and one
// active property, and returns the harness scoped to them.
func (h *tasksHarness) withOwner(tz string) *tasksHarness {
	t := h.t
	t.Helper()

	owner, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, $4)`,
		owner, phone, actor.RoleOwner, tz,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	propID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Квартира', 'apartment', 'Москва, Тверская 1', 'active')`,
		propID, owner,
	); err != nil {
		t.Fatalf("seed property: %v", err)
	}
	h.owner = owner
	h.propID = propID
	return h
}

// withOwnerWithoutProperty seeds a user with the given timezone and no
// properties at all — the pure property-less book (ADR 0052). The propID
// field stays the zero UUID: the property-less use cases never touch it.
func (h *tasksHarness) withOwnerWithoutProperty(tz string) *tasksHarness {
	t := h.t
	t.Helper()

	owner, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, $4)`,
		owner, phone, actor.RoleOwner, tz,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	h.owner = owner
	return h
}

// seedActor seeds a bare user row so audit_log's actor FK holds for members
// and strangers of the role-matrix tests.
func (h *tasksHarness) seedActor(id uuid.UUID) {
	t := h.t
	t.Helper()
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, 'Europe/Moscow')`,
		id, phone, actor.RoleOwner,
	); err != nil {
		t.Fatalf("seed actor: %v", err)
	}
}

// seedRule inserts a task rule directly (raw SQL — the tick tests drive
// materialization over pre-seeded rules with past anchors, which the use
// case validator would reject as backdated).
func (h *tasksHarness) seedRule(anchor, dueTime, repeat, title string) uuid.UUID {
	t := h.t
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	var dueDate any
	if anchor != "" {
		dueDate = anchor
	}
	var timeOfDay any
	if dueTime != "" {
		timeOfDay = dueTime
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO task_rules (id, owner_id, property_id, title, comment, due_date, due_time, repeat)
		 VALUES ($1, $2, $3, $4, NULL, $5, $6, $7)`,
		id, h.owner, h.propID, title, dueDate, timeOfDay, repeat,
	); err != nil {
		t.Fatalf("seed task rule: %v", err)
	}
	return id
}

// seedRuleWithoutProperty inserts a task rule without a property directly (raw
// SQL — the same backdated-anchor driver as seedRule, on the property-less
// slice of ADR 0052).
func (h *tasksHarness) seedRuleWithoutProperty(anchor, dueTime, repeat, title string) uuid.UUID {
	t := h.t
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	var dueDate any
	if anchor != "" {
		dueDate = anchor
	}
	var timeOfDay any
	if dueTime != "" {
		timeOfDay = dueTime
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO task_rules (id, owner_id, property_id, title, comment, due_date, due_time, repeat)
		 VALUES ($1, $2, NULL, $3, NULL, $4, $5, $6)`,
		id, h.owner, title, dueDate, timeOfDay, repeat,
	); err != nil {
		t.Fatalf("seed property-less task rule: %v", err)
	}
	return id
}

// createCmd is the canonical valid create fixture: a weekly Thursday task.
func (h *tasksHarness) createCmd() tasksapp.CreateRuleCommand {
	due := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	title := "Проверить счётчики"
	return tasksapp.CreateRuleCommand{
		Title:   title,
		DueDate: &due,
		Repeat:  domain.RepeatWeekly,
	}
}

func (h *tasksHarness) ctx() context.Context { return context.Background() }

// runTick runs the worker-path tick over the harness owner.
func (h *tasksHarness) runTick() {
	h.t.Helper()
	if err := h.tick.RunOwnerTick(h.ctx(), h.owner); err != nil {
		h.t.Fatalf("run owner tick: %v", err)
	}
}

// taskRow is the projection-free row of the shared tasks loader: everything
// any test family needs, read once.
type taskRow struct {
	ID            uuid.UUID
	RuleID        *uuid.UUID
	DueDate       *string
	DueTime       *string
	Title         string
	CompletedDate *string
}

// loadTasks reads the property's ordered tasks straight from the table — the
// single SQL home of all test families' fixtures inspection.
func (h *tasksHarness) loadTasks(t *testing.T) []taskRow {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(), `
		SELECT id, rule_id, due_date::text, due_time::text, title, completed_date::text
		FROM tasks WHERE property_id = $1
		ORDER BY created_at, id`, h.propID)
	if err != nil {
		t.Fatalf("query tasks: %v", err)
	}
	defer rows.Close()

	out := []taskRow{}
	for rows.Next() {
		var r taskRow
		if err := rows.Scan(&r.ID, &r.RuleID, &r.DueDate, &r.DueTime, &r.Title, &r.CompletedDate); err != nil {
			t.Fatalf("scan task: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tasks: %v", err)
	}
	return out
}

// loadTasksWithoutProperty reads the owner's property-less tasks — the
// property-less counterpart of loadTasks (ADR 0052).
func (h *tasksHarness) loadTasksWithoutProperty(t *testing.T) []taskRow {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(), `
		SELECT id, rule_id, due_date::text, due_time::text, title, completed_date::text
		FROM tasks WHERE owner_id = $1 AND property_id IS NULL
		ORDER BY created_at, id`, h.owner)
	if err != nil {
		t.Fatalf("query property-less tasks: %v", err)
	}
	defer rows.Close()

	out := []taskRow{}
	for rows.Next() {
		var r taskRow
		if err := rows.Scan(&r.ID, &r.RuleID, &r.DueDate, &r.DueTime, &r.Title, &r.CompletedDate); err != nil {
			t.Fatalf("scan task: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tasks: %v", err)
	}
	return out
}

// datesOf projects the loaded tasks onto their due dates ("" for undated).
func datesOf(tasks []taskRow) []string {
	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.DueDate == nil {
			out = append(out, "")
		} else {
			out = append(out, *task.DueDate)
		}
	}
	return out
}

// tasksWithDate filters the loaded tasks by due date ("" = undated).
func tasksWithDate(tasks []taskRow, date string) []taskRow {
	out := make([]taskRow, 0)
	for _, task := range tasks {
		got := ""
		if task.DueDate != nil {
			got = *task.DueDate
		}
		if got == date {
			out = append(out, task)
		}
	}
	return out
}

// ruleDatesOf groups the loaded tasks by their rule.
func ruleDatesOf(tasks []taskRow, ruleID uuid.UUID) []string {
	out := make([]string, 0)
	for _, task := range tasks {
		if task.RuleID != nil && *task.RuleID == ruleID {
			if task.DueDate == nil {
				out = append(out, "")
			} else {
				out = append(out, *task.DueDate)
			}
		}
	}
	return out
}

// auditActions loads the entity's audit trail actions in recording order.
func (h *tasksHarness) auditActions(t *testing.T, entityType string, entityID uuid.UUID) []string {
	t.Helper()
	rows, err := h.pool.Query(h.ctx(),
		`SELECT action FROM audit_log WHERE entity_type = $1 AND entity_id = $2 ORDER BY created_at, id`,
		entityType, entityID)
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	defer rows.Close()
	actions := []string{}
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			t.Fatalf("scan audit: %v", err)
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate audit: %v", err)
	}
	return actions
}
