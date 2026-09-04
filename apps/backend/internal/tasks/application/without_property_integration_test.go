//go:build integration

package application_test

// The property-less slice family (ADR 0052, карта #518 тикет #520): the
// owner-book rule lifecycle, the same-tx first materialization, the
// completion toggle, the owner-only access, the tick and the zone sweep over
// the property-less rules, and the slice isolation — bound rows never mix
// into the property-less use cases, the cascade still takes the bound rows
// with the property.

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	taskspg "github.com/nambers/arenda-planform/apps/backend/internal/tasks/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

// callBookkeeperTitle is the canonical property-less rule title fixture.
const callBookkeeperTitle = "Позвонить бухгалтеру"

// withoutPropertyCreateCmd is the canonical property-less create fixture: the
// same weekly Thursday task as the harness createCmd, no property.
func withoutPropertyCreateCmd() application.CreateRuleCommand {
	return application.CreateRuleCommand{
		Title:   callBookkeeperTitle,
		Repeat:  domain.RepeatWeekly,
		DueDate: new(time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)),
	}
}

func TestWithoutPropertyRuleLifecycle(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	// Create: the first materialization happens in the same transaction.
	rule, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, withoutPropertyCreateCmd())
	if err != nil {
		t.Fatalf("create property-less rule: %v", err)
	}
	if rule.PropertyID != nil {
		t.Fatalf("created rule property = %v, want nil", *rule.PropertyID)
	}
	tasks := h.loadTasksWithoutProperty(t)
	if got := datesOf(tasks); !slices.Equal(got, []string{day10, day17}) {
		t.Fatalf("first materialization = %v, want [%s %s]", got, day10, day17)
	}

	// Read back through the property-less key.
	stored, err := h.rules.GetRuleWithoutProperty(h.ctx(), h.owner, rule.ID)
	if err != nil {
		t.Fatalf("get property-less rule: %v", err)
	}
	if stored.Title != callBookkeeperTitle {
		t.Fatalf("stored title = %q", stored.Title)
	}

	// Edit: the future task is re-stood with the fresh snapshot; today's
	// already-due task keeps its frozen snapshot.
	title := "Позвонить бухгалтеру фирмы"
	if _, err := h.rules.UpdateRuleWithoutProperty(h.ctx(), h.owner, rule.ID,
		application.UpdateRuleCommand{Title: &title}); err != nil {
		t.Fatalf("update property-less rule: %v", err)
	}
	tasks = h.loadTasksWithoutProperty(t)
	today := tasksWithDate(tasks, day10)
	future := tasksWithDate(tasks, day17)
	if len(today) != 1 || len(future) != 1 {
		t.Fatalf("tasks after edit = %v", datesOf(tasks))
	}
	if today[0].Title != callBookkeeperTitle {
		t.Fatalf("today's snapshot followed the edit: %q", today[0].Title)
	}
	if future[0].Title != title {
		t.Fatalf("future snapshot = %q, want the fresh %q", future[0].Title, title)
	}

	// Delete: the uncompleted tasks die, the completed journal would stay.
	if err := h.rules.DeleteRuleWithoutProperty(h.ctx(), h.owner, rule.ID); err != nil {
		t.Fatalf("delete property-less rule: %v", err)
	}
	if tasks := h.loadTasksWithoutProperty(t); len(tasks) != 0 {
		t.Fatalf("uncompleted tasks survived the rule deletion: %v", datesOf(tasks))
	}
	if _, err := h.rules.GetRuleWithoutProperty(h.ctx(), h.owner, rule.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("deleted rule read = %v, want ErrNotFound", err)
	}
}

func TestWithoutPropertyUndatedRule(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	rule, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, application.CreateRuleCommand{
		Title:  "Разобрать коробки",
		Repeat: domain.RepeatOnce,
	})
	if err != nil {
		t.Fatalf("create undated property-less rule: %v", err)
	}
	tasks := h.loadTasksWithoutProperty(t)
	if len(tasks) != 1 || tasks[0].DueDate != nil {
		t.Fatalf("undated materialization = %v, want the single undated task", datesOf(tasks))
	}

	// The tick keeps the undated steady state: no dated future ever.
	h.runTick()
	if tasks := h.loadTasksWithoutProperty(t); len(tasks) != 1 {
		t.Fatalf("tick disturbed the undated rule: %v", datesOf(tasks))
	}

	if err := h.rules.DeleteRuleWithoutProperty(h.ctx(), h.owner, rule.ID); err != nil {
		t.Fatalf("delete undated rule: %v", err)
	}
	if tasks := h.loadTasksWithoutProperty(t); len(tasks) != 0 {
		t.Fatalf("undated task survived the rule deletion: %v", datesOf(tasks))
	}
}

func TestWithoutPropertyCompletionToggle(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	rule, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, withoutPropertyCreateCmd())
	if err != nil {
		t.Fatalf("create property-less rule: %v", err)
	}
	today := tasksWithDate(h.loadTasksWithoutProperty(t), day10)[0]

	// «Выполнить»: the completion fact is today in the owner's timezone.
	completed, err := h.tasks.CompleteTaskWithoutProperty(h.ctx(), h.owner, today.ID)
	if err != nil {
		t.Fatalf("complete property-less task: %v", err)
	}
	if completed.CompletedDate == nil || completed.CompletedDate.UTC().Format("2006-01-02") != day10 {
		t.Fatalf("completed date = %v, want %s", completed.CompletedDate, day10)
	}
	if _, err := h.tasks.CompleteTaskWithoutProperty(h.ctx(), h.owner, today.ID); !errors.Is(err, application.ErrAlreadyCompleted) {
		t.Fatalf("second complete = %v, want ErrAlreadyCompleted", err)
	}

	// «Отменить выполнение» returns the task to the active ones.
	reopened, err := h.tasks.UncompleteTaskWithoutProperty(h.ctx(), h.owner, today.ID)
	if err != nil {
		t.Fatalf("uncomplete property-less task: %v", err)
	}
	if reopened.CompletedDate != nil {
		t.Fatalf("reopened task carries a completion fact: %v", reopened.CompletedDate)
	}

	// The journal row of a deleted rule is irreversible.
	if _, err := h.tasks.CompleteTaskWithoutProperty(h.ctx(), h.owner, today.ID); err != nil {
		t.Fatalf("complete before rule deletion: %v", err)
	}
	if err := h.rules.DeleteRuleWithoutProperty(h.ctx(), h.owner, rule.ID); err != nil {
		t.Fatalf("delete rule: %v", err)
	}
	journal := h.loadTasksWithoutProperty(t)
	if len(journal) != 1 || journal[0].CompletedDate == nil {
		t.Fatalf("journal after rule deletion = %v, want the single completed row", datesOf(journal))
	}
	if journal[0].RuleID != nil {
		t.Fatalf("journal row still points at the deleted rule: %v", *journal[0].RuleID)
	}
	if _, err := h.tasks.UncompleteTaskWithoutProperty(h.ctx(), h.owner, journal[0].ID); !errors.Is(err, application.ErrRuleDeleted) {
		t.Fatalf("journal uncomplete = %v, want ErrRuleDeleted", err)
	}
}

func TestWithoutPropertyAccessOwnerOnly(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	rule, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, withoutPropertyCreateCmd())
	if err != nil {
		t.Fatalf("create property-less rule: %v", err)
	}
	today := tasksWithDate(h.loadTasksWithoutProperty(t), day10)[0]

	// A stranger: the stub resolves full_access for any property query, but
	// the owner-book paths never even consult it — the row key is the gate.
	stranger := uuid.Must(uuid.NewV7())
	h.seedActor(stranger)

	if _, err := h.rules.GetRuleWithoutProperty(h.ctx(), stranger, rule.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("stranger read = %v, want ErrNotFound", err)
	}
	if _, err := h.rules.UpdateRuleWithoutProperty(h.ctx(), stranger, rule.ID,
		application.UpdateRuleCommand{}); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("stranger update = %v, want ErrNotFound", err)
	}
	if err := h.rules.DeleteRuleWithoutProperty(h.ctx(), stranger, rule.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("stranger delete = %v, want ErrNotFound", err)
	}
	if _, err := h.tasks.CompleteTaskWithoutProperty(h.ctx(), stranger, today.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("stranger complete = %v, want ErrNotFound", err)
	}
	if _, err := h.tasks.UncompleteTaskWithoutProperty(h.ctx(), stranger, today.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("stranger uncomplete = %v, want ErrNotFound", err)
	}

	// The stranger's own create lands in the stranger's own book.
	strangerRule, err := h.rules.CreateRuleWithoutProperty(h.ctx(), stranger, withoutPropertyCreateCmd())
	if err != nil {
		t.Fatalf("stranger create: %v", err)
	}
	rows, err := h.pool.Query(h.ctx(),
		`SELECT count(*) FROM task_rules WHERE id = $1 AND owner_id = $2 AND property_id IS NULL`,
		strangerRule.ID, stranger)
	if err != nil {
		t.Fatalf("query stranger rule: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("no count row")
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		t.Fatalf("scan count: %v", err)
	}
	if count != 1 {
		t.Fatalf("stranger rule count = %d, want 1", count)
	}
}

func TestWithoutPropertySliceIsolation(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	// A bound rule and a bound task never answer on the property-less keys.
	boundRule, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd())
	if err != nil {
		t.Fatalf("create bound rule: %v", err)
	}
	page, err := h.tasks.ListTasks(h.ctx(), h.owner, h.propID, application.TasksListQuery{})
	if err != nil {
		t.Fatalf("list bound tasks: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("bound materialization = %d tasks, want 2", len(page.Items))
	}
	boundTask := page.Items[0].Task

	if _, err := h.rules.GetRuleWithoutProperty(h.ctx(), h.owner, boundRule.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("bound rule read on the property-less key = %v, want ErrNotFound", err)
	}
	if _, err := h.tasks.CompleteTaskWithoutProperty(h.ctx(), h.owner, boundTask.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("bound task complete on the property-less key = %v, want ErrNotFound", err)
	}

	// The property deletion cascade takes the bound rows; the property-less
	// slice of the same owner is untouched (migration 000120 keeps the FK).
	withoutPropertyRule, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, withoutPropertyCreateCmd())
	if err != nil {
		t.Fatalf("create property-less rule: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(), `DELETE FROM properties WHERE id = $1`, h.propID); err != nil {
		t.Fatalf("delete property: %v", err)
	}
	var bound, withoutPropertyCount int
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT
			count(*) FILTER (WHERE property_id = $1),
			count(*) FILTER (WHERE id = $2 AND property_id IS NULL)
		 FROM task_rules`, h.propID, withoutPropertyRule.ID).Scan(&bound, &withoutPropertyCount); err != nil {
		t.Fatalf("count rules after cascade: %v", err)
	}
	if bound != 0 {
		t.Fatalf("bound rule survived the property cascade: %d", bound)
	}
	if withoutPropertyCount != 1 {
		t.Fatalf("property-less rule lost by the property cascade: %d", withoutPropertyCount)
	}
}

func TestWithoutPropertyTickMaterialization(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	// Seeded rules with backdated anchors: a missed once, a missed weekly
	// pair, and an undated one — the tick materializes them all on the
	// property-less slice.
	once := h.seedRuleWithoutProperty(day03, "", "once", "Отдать показания")
	weekly := h.seedRuleWithoutProperty(day03, "09:00", "weekly", "Проверить почту")
	undated := h.seedRuleWithoutProperty("", "", "once", "Разобрать коробки")

	h.runTick()

	tasks := h.loadTasksWithoutProperty(t)
	byRule := map[uuid.UUID][]string{}
	for _, task := range tasks {
		byRule[*task.RuleID] = append(byRule[*task.RuleID], orUndated(task.DueDate))
	}
	if got := byRule[once]; !slices.Equal(got, []string{day03}) {
		t.Fatalf("once materialization = %v, want [%s]", got, day03)
	}
	if got := byRule[weekly]; !slices.Equal(got, []string{day03, day10, day17}) {
		t.Fatalf("weekly materialization = %v, want [%s %s %s] (missed pair + single future)", got, day03, day10, day17)
	}
	if got := byRule[undated]; !slices.Equal(got, []string{""}) {
		t.Fatalf("undated materialization = %v, want the single undated task", got)
	}

	// Idempotence: the rerun converges to a no-op.
	h.runTick()
	if after := h.loadTasksWithoutProperty(t); len(after) != len(tasks) {
		t.Fatalf("rerun changed the materialization: %d → %d", len(tasks), len(after))
	}
}

func TestWithoutPropertyTickZones(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwnerWithoutProperty(taskMoscowTZ)

	// An owner with only a property-less rule is a sweep target even though
	// they own no properties at all.
	h.seedRuleWithoutProperty(day03, "", "once", "Отдать показания")

	zones := taskspg.NewTickZoneDirectory(h.pool)
	listed, err := zones.ListTickZones(h.ctx())
	if err != nil {
		t.Fatalf("list tick zones: %v", err)
	}
	found := false
	for _, zone := range listed {
		if zone.Timezone == taskMoscowTZ && slices.Contains(zone.Owners, h.owner) {
			found = true
		}
	}
	if !found {
		t.Fatalf("property-less owner missing from the zone sweep: %+v", listed)
	}

	// The worker door materializes the property-less rule end-to-end.
	if err := h.tick.RunZoneTicks(h.ctx(), tickBaseTime); err != nil {
		t.Fatalf("run zone ticks: %v", err)
	}
	tasks := h.loadTasksWithoutProperty(t)
	if len(tasks) != 1 || orUndated(tasks[0].DueDate) != day03 {
		t.Fatalf("zone tick materialization = %v, want the missed [%s]", datesOf(tasks), day03)
	}
}

func TestWithoutPropertyAuditTrail(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	rule, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, withoutPropertyCreateCmd())
	if err != nil {
		t.Fatalf("create property-less rule: %v", err)
	}
	title := "Позвонить бухгалтеру фирмы"
	if _, err := h.rules.UpdateRuleWithoutProperty(h.ctx(), h.owner, rule.ID,
		application.UpdateRuleCommand{Title: &title}); err != nil {
		t.Fatalf("update property-less rule: %v", err)
	}
	today := tasksWithDate(h.loadTasksWithoutProperty(t), day10)[0]
	if _, err := h.tasks.CompleteTaskWithoutProperty(h.ctx(), h.owner, today.ID); err != nil {
		t.Fatalf("complete property-less task: %v", err)
	}
	if err := h.rules.DeleteRuleWithoutProperty(h.ctx(), h.owner, rule.ID); err != nil {
		t.Fatalf("delete property-less rule: %v", err)
	}

	actions := h.auditActions(t, "task_rule", rule.ID)
	want := []string{"task_rule.created", "task_rule.updated", "task_rule.deleted"}
	if !slices.Equal(actions, want) {
		t.Fatalf("rule audit trail = %v, want %v", actions, want)
	}
	completedActions := h.auditActions(t, "task", today.ID)
	if !slices.Equal(completedActions, []string{"task.completed"}) {
		t.Fatalf("task audit trail = %v, want [task.completed]", completedActions)
	}
}

// orUndated projects a due date onto the shared date string, "" for undated.
func orUndated(due *string) string {
	if due == nil {
		return ""
	}
	return *due
}
