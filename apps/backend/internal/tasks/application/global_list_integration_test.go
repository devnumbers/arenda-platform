//go:build integration

package application_test

// The global listing family (карта #518 тикет #521, ADR 0052 + ADR 0028):
// the merged visibility over real stores and the real membership policy —
// the owner sees their bound and property-less tasks, a sharing member sees
// the shared properties' bound tasks plus their own book only, the privacy
// 404 on a foreign propertyId — plus the slice filters, the pagination, the
// completed journal and the per-owner view buckets.

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	taskspg "github.com/nambers/arenda-planform/apps/backend/internal/tasks/adapters/postgres"
	tasksapp "github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

// newTasksHarnessWithRealPolicy is the harness for the global listing's
// member scenarios: the production membership policy over the harness pool,
// so the participant tests exercise the same authorization the wire runs.
// The services are rebuilt over the same store wiring as the harness's own
// (integration_harness_test.go) — only the policy changes.
func newTasksHarnessWithRealPolicy(t *testing.T) *tasksHarness {
	t.Helper()
	h := newTasksHarness(t)
	policy := accessapp.NewMembershipPolicy(
		accesspg.NewOwnerResolver(h.pool),
		accesspg.NewMembershipRepository(h.pool),
	)
	ownerClock := taskspg.NewOwnerClock(h.pool, h.clock)
	factory := tasksapp.NewTxStoreFactory(
		taskspg.NewTickStore(h.pool),
		taskspg.NewRuleStore(h.pool),
		taskspg.NewTaskStore(h.pool),
		taskspg.NewPropertyStore(h.pool),
		auditapp.NewService(auditpg.NewWriter(h.pool), h.clock),
		pgdb.NewUoW(h.pool, slog.New(slog.DiscardHandler)),
	)
	h.rules = tasksapp.NewRuleService(factory, ownerClock, policy)
	h.tasks = tasksapp.NewTaskService(factory, ownerClock, ownerClock, policy)
	return h
}

// seedSecondOwner seeds one more user with a property, returning both ids —
// the shared-access scenarios need at least two owners in one pool.
func (h *tasksHarness) seedSecondOwner(
	t *testing.T, tz, name string,
) (ownerID, propID uuid.UUID) {
	t.Helper()
	var err error
	ownerID, err = uuid.NewV7()
	if err != nil {
		t.Fatalf("new owner id: %v", err)
	}
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, $4)`,
		ownerID, phone, actor.RoleOwner, tz,
	); err != nil {
		t.Fatalf("seed second owner: %v", err)
	}
	propID, err = uuid.NewV7()
	if err != nil {
		t.Fatalf("new property id: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, $3, 'apartment', 'Москва, Тверская 2', 'active')`,
		propID, ownerID, name,
	); err != nil {
		t.Fatalf("seed second property: %v", err)
	}
	return ownerID, propID
}

// seedMembership inserts one active viewer membership row — the only role
// the global listing scenarios need.
func (h *tasksHarness) seedMembership(t *testing.T, propertyID, userID uuid.UUID) {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new membership id: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO property_members (id, property_id, user_id, role, granted_by)
		 VALUES ($1, $2, $3, 'viewer', $4)`,
		id, propertyID, userID, h.owner,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

func TestGlobalList_OwnerSeesOwnBoundAndPropertyLess(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	if _, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd()); err != nil {
		t.Fatalf("create bound rule: %v", err)
	}
	if _, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, withoutPropertyCreateCmd()); err != nil {
		t.Fatalf("create property-less rule: %v", err)
	}

	page, err := h.tasks.ListGlobalTasks(h.ctx(), h.owner, tasksapp.GlobalTasksListQuery{})
	if err != nil {
		t.Fatalf("list global tasks: %v", err)
	}
	if page.Total != 4 || len(page.Items) != 4 {
		t.Fatalf("merged feed = %d items / total %d, want 4/4 (bound 2 + property-less 2)", len(page.Items), page.Total)
	}
	bound, unbound := 0, 0
	for _, item := range page.Items {
		if item.Task.PropertyID == nil {
			unbound++
			if item.PropertyName != "" {
				t.Fatalf("property-less row carries a label %q", item.PropertyName)
			}
			continue
		}
		bound++
		if item.PropertyName == "" {
			t.Fatalf("bound row lost its property label: %+v", item.Task)
		}
	}
	if bound != 2 || unbound != 2 {
		t.Fatalf("merged feed split bound/unbound = %d/%d, want 2/2", bound, unbound)
	}
}

// participantScenario seeds the shared-access fixture: the feed owner with
// one bound and one property-less rule, and a viewer-participant with their
// own property-less rule. Returns the harness re-scoped to the participant
// plus the shared property's id (the feed owner's).
func participantScenario(t *testing.T) (participantH *tasksHarness, ownerPropID uuid.UUID) {
	t.Helper()
	h := newTasksHarnessWithRealPolicy(t).withOwner(taskMoscowTZ)

	if _, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd()); err != nil {
		t.Fatalf("create bound rule: %v", err)
	}
	if _, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, withoutPropertyCreateCmd()); err != nil {
		t.Fatalf("create property-less rule: %v", err)
	}

	participant, participantProp := h.seedSecondOwner(t, taskMoscowTZ, "Участник")
	h.seedMembership(t, h.propID, participant)
	participantH2 := *h
	participantH2.owner = participant
	participantH2.propID = participantProp
	if _, err := participantH2.rules.CreateRuleWithoutProperty(participantH2.ctx(), participant, withoutPropertyCreateCmd()); err != nil {
		t.Fatalf("create participant property-less rule: %v", err)
	}
	return &participantH2, h.propID
}

func TestGlobalList_ParticipantMergedVisibility(t *testing.T) {
	t.Parallel()
	participantH, _ := participantScenario(t)

	page, err := participantH.tasks.ListGlobalTasks(participantH.ctx(), participantH.owner, tasksapp.GlobalTasksListQuery{})
	if err != nil {
		t.Fatalf("participant list global tasks: %v", err)
	}
	// Own property-less tasks (2) + the shared property's bound tasks (2);
	// the feed owner's property-less tasks (2) never show.
	if page.Total != 4 || len(page.Items) != 4 {
		t.Fatalf("participant merged feed = %d items / total %d, want 4/4", len(page.Items), page.Total)
	}
	for _, item := range page.Items {
		if item.Task.PropertyID == nil && item.Task.OwnerID != participantH.owner {
			t.Fatalf("the feed owner's property-less task leaked: %+v", item.Task)
		}
	}
}

func TestGlobalList_ParticipantPropertyFilter(t *testing.T) {
	t.Parallel()
	participantH, ownerPropID := participantScenario(t)

	// The shared property reads through the real policy's view gate and
	// labels every row.
	propPage, err := participantH.tasks.ListGlobalTasks(participantH.ctx(), participantH.owner,
		tasksapp.GlobalTasksListQuery{PropertyIDs: []uuid.UUID{ownerPropID}})
	if err != nil {
		t.Fatalf("participant property-filtered list: %v", err)
	}
	if propPage.Total != 2 || len(propPage.Items) != 2 {
		t.Fatalf("participant property feed = %d items / total %d, want 2/2", len(propPage.Items), propPage.Total)
	}
	for _, item := range propPage.Items {
		if item.PropertyName != "Квартира" {
			t.Fatalf("row label = %q, want Квартира", item.PropertyName)
		}
	}
}

func TestGlobalList_ParticipantWithoutPropertyFilter(t *testing.T) {
	t.Parallel()
	participantH, _ := participantScenario(t)

	// The withoutProperty filter is the participant's own book only.
	withoutPage, err := participantH.tasks.ListGlobalTasks(participantH.ctx(), participantH.owner,
		tasksapp.GlobalTasksListQuery{WithoutProperty: true})
	if err != nil {
		t.Fatalf("participant without-property list: %v", err)
	}
	if withoutPage.Total != 2 || len(withoutPage.Items) != 2 {
		t.Fatalf("participant property-less feed = %d items / total %d, want 2/2", len(withoutPage.Items), withoutPage.Total)
	}
}

func TestGlobalList_SuspendedMemberLosesSharedVisibility(t *testing.T) {
	t.Parallel()
	h := newTasksHarnessWithRealPolicy(t).withOwner(taskMoscowTZ)

	if _, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd()); err != nil {
		t.Fatalf("create bound rule: %v", err)
	}
	participant, participantProp := h.seedSecondOwner(t, taskMoscowTZ, "Участник")
	h.seedMembership(t, h.propID, participant)
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE property_members SET status = 'suspended' WHERE property_id = $1 AND user_id = $2`,
		h.propID, participant,
	); err != nil {
		t.Fatalf("suspend membership: %v", err)
	}

	participantH := *h
	participantH.owner = participant
	participantH.propID = participantProp

	page, err := participantH.tasks.ListGlobalTasks(participantH.ctx(), participant, tasksapp.GlobalTasksListQuery{})
	if err != nil {
		t.Fatalf("suspended participant list: %v", err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("suspended participant sees %d shared tasks, want none", len(page.Items))
	}
	_, err = participantH.tasks.ListGlobalTasks(participantH.ctx(), participant,
		tasksapp.GlobalTasksListQuery{PropertyIDs: []uuid.UUID{h.propID}})
	if !errors.Is(err, tasksapp.ErrNotFound) {
		t.Fatalf("suspended participant property branch = %v, want ErrNotFound", err)
	}
}

func TestGlobalList_ForeignPropertyIsPrivacy404(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	stranger, strangerProp := h.seedSecondOwner(t, taskMoscowTZ, "Чужак")
	strangerH := *h
	strangerH.owner = stranger
	strangerH.propID = strangerProp

	if _, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd()); err != nil {
		t.Fatalf("create bound rule: %v", err)
	}

	_, err := strangerH.tasks.ListGlobalTasks(strangerH.ctx(), stranger,
		tasksapp.GlobalTasksListQuery{PropertyIDs: []uuid.UUID{h.propID}})
	if !errors.Is(err, tasksapp.ErrNotFound) {
		t.Fatalf("foreign property branch = %v, want ErrNotFound", err)
	}

	// The merged feed without a filter is not an error for a stranger — it
	// just holds only their own book.
	page, err := strangerH.tasks.ListGlobalTasks(strangerH.ctx(), stranger, tasksapp.GlobalTasksListQuery{})
	if err != nil {
		t.Fatalf("stranger merged list: %v", err)
	}
	if page.Total != 0 {
		t.Fatalf("stranger merged feed total = %d, want 0", page.Total)
	}
}

func TestGlobalList_PaginationAndTotals(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	// Three weekly rules: each materializes today's and the single future
	// task — six rows in the active bucket.
	for i := range 3 {
		if _, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd()); err != nil {
			t.Fatalf("create rule %d: %v", i, err)
		}
	}

	first, err := h.tasks.ListGlobalTasks(h.ctx(), h.owner, tasksapp.GlobalTasksListQuery{
		TasksListQuery: tasksapp.TasksListQuery{Limit: 4},
	})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(first.Items) != 4 || first.Total != 6 {
		t.Fatalf("first page = %d items / total %d, want 4/6", len(first.Items), first.Total)
	}

	second, err := h.tasks.ListGlobalTasks(h.ctx(), h.owner, tasksapp.GlobalTasksListQuery{
		TasksListQuery: tasksapp.TasksListQuery{Limit: 4, Offset: 4},
	})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(second.Items) != 2 || second.Total != 6 {
		t.Fatalf("second page = %d items / total %d, want 2/6", len(second.Items), second.Total)
	}

	// The pages do not overlap.
	seen := map[uuid.UUID]bool{}
	for _, item := range append(slices.Clone(first.Items), second.Items...) {
		if seen[item.Task.ID] {
			t.Fatalf("task %s crossed the page boundary twice", item.Task.ID)
		}
		seen[item.Task.ID] = true
	}
}

func TestGlobalList_CompletedJournalFilter(t *testing.T) {
	t.Parallel()
	h := newTasksHarness(t).withOwner(taskMoscowTZ)

	if _, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd()); err != nil {
		t.Fatalf("create bound rule: %v", err)
	}
	if _, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, withoutPropertyCreateCmd()); err != nil {
		t.Fatalf("create property-less rule: %v", err)
	}
	tasks := h.loadTasks(t)
	if _, err := h.tasks.CompleteTask(h.ctx(), h.owner, h.propID, tasks[0].ID); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	active, err := h.tasks.ListGlobalTasks(h.ctx(), h.owner, tasksapp.GlobalTasksListQuery{})
	if err != nil {
		t.Fatalf("active bucket: %v", err)
	}
	if active.Total != 3 {
		t.Fatalf("active total = %d, want 3 (6 minus the completion)", active.Total)
	}

	journal, err := h.tasks.ListGlobalTasks(h.ctx(), h.owner, tasksapp.GlobalTasksListQuery{
		TasksListQuery: tasksapp.TasksListQuery{Completed: true},
	})
	if err != nil {
		t.Fatalf("completed bucket: %v", err)
	}
	if journal.Total != 1 || len(journal.Items) != 1 {
		t.Fatalf("journal = %d items / total %d, want 1/1", len(journal.Items), journal.Total)
	}
	if journal.Items[0].Task.ID != tasks[0].ID || journal.Items[0].Status != domain.ViewCompleted {
		t.Fatalf("journal row = %+v, want the completed task", journal.Items[0])
	}
	if journal.Items[0].PropertyName == "" {
		t.Fatalf("journal row lost the property label")
	}
}

func TestGlobalList_ArchivedPropertiesAreOutOfTheFeed(t *testing.T) {
	t.Parallel()
	// Решение 9 карты #522: задачи архивных объектов в глобальной ленте не
	// показываются — не-архивные объекты + безобъектные.
	h, archivedProp := archivedSharedScenario(t)

	page, err := h.tasks.ListGlobalTasks(h.ctx(), h.owner, tasksapp.GlobalTasksListQuery{})
	if err != nil {
		t.Fatalf("merged list: %v", err)
	}
	// The archived property's two tasks are out; own bound (2) + property-less (2) stay.
	if page.Total != 4 || len(page.Items) != 4 {
		t.Fatalf("merged feed = %d items / total %d, want 4/4 (archived out)", len(page.Items), page.Total)
	}
	for _, item := range page.Items {
		if item.Task.PropertyID != nil && *item.Task.PropertyID == archivedProp {
			t.Fatalf("an archived property's task leaked into the feed: %+v", item.Task)
		}
	}

	// The archived exclusion holds for the completed journal too.
	journal, err := h.tasks.ListGlobalTasks(h.ctx(), h.owner, tasksapp.GlobalTasksListQuery{
		TasksListQuery: tasksapp.TasksListQuery{Completed: true},
	})
	if err != nil {
		t.Fatalf("journal list: %v", err)
	}
	if journal.Total != 0 {
		t.Fatalf("journal total = %d, want 0 (the archived rows are out)", journal.Total)
	}

	// The participant sees the non-archived shared property, not the archived one.
	reader, readerProp := h.seedSecondOwner(t, taskMoscowTZ, "Читатель")
	h.seedMembership(t, h.propID, reader)
	readerH := *h
	readerH.owner = reader
	readerH.propID = readerProp
	readerPage, err := readerH.tasks.ListGlobalTasks(readerH.ctx(), reader, tasksapp.GlobalTasksListQuery{})
	if err != nil {
		t.Fatalf("reader merged list: %v", err)
	}
	if readerPage.Total != 2 || len(readerPage.Items) != 2 {
		t.Fatalf("reader merged feed = %d items / total %d, want 2/2", len(readerPage.Items), readerPage.Total)
	}
}

// archivedSharedScenario seeds the owner's bound and property-less rules
// plus a shared (viewable by the owner) property that then gets archived
// with two tasks of its own.
func archivedSharedScenario(t *testing.T) (*tasksHarness, uuid.UUID) {
	t.Helper()
	h := newTasksHarnessWithRealPolicy(t).withOwner(taskMoscowTZ)

	if _, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd()); err != nil {
		t.Fatalf("create bound rule: %v", err)
	}
	if _, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, withoutPropertyCreateCmd()); err != nil {
		t.Fatalf("create property-less rule: %v", err)
	}
	archivedOwner, archivedProp := h.seedSecondOwner(t, taskMoscowTZ, "Архив")
	h.seedMembership(t, archivedProp, h.owner)
	archivedH := *h
	archivedH.owner = archivedOwner
	archivedH.propID = archivedProp
	if _, err := archivedH.rules.CreateRule(archivedH.ctx(), archivedOwner, archivedProp, h.createCmd()); err != nil {
		t.Fatalf("create archived-property rule: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE properties SET status = 'archived' WHERE id = $1`, archivedProp,
	); err != nil {
		t.Fatalf("archive property: %v", err)
	}
	return h, archivedProp
}

func TestGlobalList_PerOwnerBucketsAndOrdering(t *testing.T) {
	t.Parallel()

	// The actor (Moscow, 2026-09-10 12:00 UTC → 15:00) shares two properties:
	// one with a Moscow owner, one with a Kamchatka owner (already 22:00 of
	// the 10th). The Moscow owner has a task due 23:50 of the 10th — still
	// active in their frame; the Kamchatka owner's date-only task on the 10th
	// is past the end of its day — overdue in their frame.
	h := newTasksHarnessWithRealPolicy(t).withOwner(taskMoscowTZ)
	h.clock.now = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	kamchatkaOwner, kamchatkaProp := h.seedSecondOwner(t, "Asia/Kamchatka", "Камчатка")

	reader, readerProp := h.seedSecondOwner(t, taskMoscowTZ, "Читатель")
	h.seedMembership(t, h.propID, reader)
	h.seedMembership(t, kamchatkaProp, reader)

	readerH := *h
	readerH.owner = reader
	readerH.propID = readerProp

	moscowH := *h
	moscowH.seedRule(day10, "23:50", string(domain.RepeatOnce), "Московская")
	moscowH.runTick()

	kamchatkaH := *h
	kamchatkaH.owner = kamchatkaOwner
	kamchatkaH.propID = kamchatkaProp
	kamchatkaH.seedRule(day10, "", string(domain.RepeatOnce), "Камчатская")
	kamchatkaH.runTick()

	// The actor's own undated task: the undated tail of the feed.
	if _, err := readerH.rules.CreateRuleWithoutProperty(readerH.ctx(), reader, tasksapp.CreateRuleCommand{
		Title:  boxesTitle,
		Repeat: domain.RepeatOnce,
	}); err != nil {
		t.Fatalf("create actor undated rule: %v", err)
	}

	page, err := readerH.tasks.ListGlobalTasks(readerH.ctx(), reader, tasksapp.GlobalTasksListQuery{})
	if err != nil {
		t.Fatalf("actor merged list: %v", err)
	}
	if page.Total != 3 || len(page.Items) != 3 {
		t.Fatalf("actor merged feed = %d items / total %d, want 3/3", len(page.Items), page.Total)
	}

	// Ordering: due order with the undated last — the same-day pair splits by
	// the due time (date-only NULLS FIRST, the property screen's order).
	if got := titlesOf(page.Items); !slices.Equal(got, []string{"Камчатская", "Московская", boxesTitle}) {
		t.Fatalf("feed order = %v, want [Камчатская Московская Разобрать коробки]", got)
	}
	// Buckets: each row computed in its own owner's frame.
	statusByTitle := map[string]domain.TaskViewStatus{}
	for _, item := range page.Items {
		statusByTitle[item.Task.Title] = item.Status
	}
	if statusByTitle["Московская"] != domain.ViewActive {
		t.Fatalf("moscow task = %v, want active in the Moscow owner's frame", statusByTitle["Московская"])
	}
	if statusByTitle["Камчатская"] != domain.ViewOverdue {
		t.Fatalf("kamchatka task = %v, want overdue in the Kamchatka owner's frame", statusByTitle["Камчатская"])
	}
	if statusByTitle[boxesTitle] != domain.ViewUndated {
		t.Fatalf("undated task = %v", statusByTitle[boxesTitle])
	}
	// The page's today is the reader's calendar date.
	if page.Today.Format(time.DateOnly) != day10 {
		t.Fatalf("page today = %s, want %s (the actor's Moscow frame)", page.Today.Format(time.DateOnly), day10)
	}
}

// titlesOf projects the page onto the row titles in feed order.
func titlesOf(items []tasksapp.TaskListItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Task.Title)
	}
	return out
}

// multiSelectScenario seeds the picker's multi-select fixture (ticket #547):
// the owner with two bound rules (one per property) plus one property-less,
// and a viewer-participant sharing the first property with their own bound
// rule on their own property. Returns the owner's harness, the participant
// re-scope, and both properties' ids.
func multiSelectScenario(t *testing.T) (ownerH, participantH *tasksHarness, ownerPropID, secondPropID uuid.UUID) {
	t.Helper()
	h := newTasksHarnessWithRealPolicy(t).withOwner(taskMoscowTZ)

	if _, err := h.rules.CreateRule(h.ctx(), h.owner, h.propID, h.createCmd()); err != nil {
		t.Fatalf("create rule on the first property: %v", err)
	}
	secondPropID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new second property id: %v", err)
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Дача', 'house', 'Москва, Тверская 4', 'active')`,
		secondPropID, h.owner,
	); err != nil {
		t.Fatalf("seed second property: %v", err)
	}
	if _, err := h.rules.CreateRule(h.ctx(), h.owner, secondPropID, h.createCmd()); err != nil {
		t.Fatalf("create rule on the second property: %v", err)
	}
	if _, err := h.rules.CreateRuleWithoutProperty(h.ctx(), h.owner, withoutPropertyCreateCmd()); err != nil {
		t.Fatalf("create property-less rule: %v", err)
	}

	participant, participantProp := h.seedSecondOwner(t, taskMoscowTZ, "Участник")
	h.seedMembership(t, h.propID, participant)
	participantH2 := *h
	participantH2.owner = participant
	participantH2.propID = participantProp
	if _, err := participantH2.rules.CreateRule(participantH2.ctx(), participant, participantProp, participantH2.createCmd()); err != nil {
		t.Fatalf("create participant bound rule: %v", err)
	}
	return h, &participantH2, h.propID, secondPropID
}

func TestGlobalList_OfPropertiesSelectsBothAndSkipsPropertyLess(t *testing.T) {
	t.Parallel()
	ownerH, participantH, ownerPropID, secondPropID := multiSelectScenario(t)

	// The owner picks both of their properties: bound tasks of both arrive
	// with their own labels; the property-less rule stays out of the slice.
	page, err := ownerH.tasks.ListGlobalTasks(ownerH.ctx(), ownerH.owner, tasksapp.GlobalTasksListQuery{
		PropertyIDs: []uuid.UUID{ownerPropID, secondPropID},
	})
	if err != nil {
		t.Fatalf("owner multi-property list: %v", err)
	}
	if page.Total != 4 || len(page.Items) != 4 {
		t.Fatalf("multi-property feed = %d items / total %d, want 4/4 (createCmd seeds 2 tasks per rule)", len(page.Items), page.Total)
	}
	for _, item := range page.Items {
		if item.Task.PropertyID == nil {
			t.Fatalf("property-less task leaked into the multi-property slice: %+v", item.Task)
		}
		if item.PropertyName != "Квартира" && item.PropertyName != "Дача" {
			t.Fatalf("row label = %q, want the row's own property name", item.PropertyName)
		}
	}

	// The participant picks the shared property plus their own: the merged
	// page spans both books, today is the reader's.
	page, err = participantH.tasks.ListGlobalTasks(participantH.ctx(), participantH.owner, tasksapp.GlobalTasksListQuery{
		PropertyIDs: []uuid.UUID{ownerPropID, participantH.propID},
	})
	if err != nil {
		t.Fatalf("participant multi-property list: %v", err)
	}
	if page.Total != 4 {
		t.Fatalf("participant multi-property total = %d, want 4 (2 shared + 2 own)", page.Total)
	}
	seenForeign := false
	for _, item := range page.Items {
		if item.Task.OwnerID != participantH.owner {
			seenForeign = true
		}
	}
	if !seenForeign {
		t.Fatal("the participant's own book is missing from the multi-property page")
	}
}

func TestGlobalList_OfPropertiesOneInvisibleIsPrivacy404(t *testing.T) {
	t.Parallel()
	_, strangerH0, _, ownerPropID := multiSelectScenario(t)

	h := strangerH0
	stranger, strangerProp := h.seedSecondOwner(t, taskMoscowTZ, "Чужак")
	strangerH := *h
	strangerH.owner = stranger
	strangerH.propID = strangerProp

	// The stranger's own property is visible to them; the owner's is not —
	// the whole request is the privacy 404, the valid id is not answered.
	_, err := strangerH.tasks.ListGlobalTasks(strangerH.ctx(), stranger, tasksapp.GlobalTasksListQuery{
		PropertyIDs: []uuid.UUID{strangerProp, ownerPropID},
	})
	if !errors.Is(err, tasksapp.ErrNotFound) {
		t.Fatalf("mixed-visibility list = %v, want ErrNotFound", err)
	}
}

func TestGlobalList_OfPropertiesSkipsArchived(t *testing.T) {
	t.Parallel()
	ownerH, _, ownerPropID, secondPropID := multiSelectScenario(t)

	if _, err := ownerH.pool.Exec(ownerH.ctx(),
		`UPDATE properties SET status = 'archived' WHERE id = $1`, secondPropID,
	); err != nil {
		t.Fatalf("archive second property: %v", err)
	}

	page, err := ownerH.tasks.ListGlobalTasks(ownerH.ctx(), ownerH.owner, tasksapp.GlobalTasksListQuery{
		PropertyIDs: []uuid.UUID{ownerPropID, secondPropID},
	})
	if err != nil {
		t.Fatalf("archived multi-property list: %v", err)
	}
	for _, item := range page.Items {
		if item.Task.PropertyID != nil && *item.Task.PropertyID == secondPropID {
			t.Fatalf("archived property's task leaked: %+v", item.Task)
		}
	}
	if page.Total != 2 {
		t.Fatalf("total = %d, want 2 (the active property's tasks only)", page.Total)
	}
}
