package http

// The contract tests of the tasks transport (ADR 0051): the create body
// folding, the PATCH tri-state resolution, the listings' parameter folding
// and the error mapping — against func-backed doubles, mirroring the
// payments handlers tests.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
)

// fakeTaskRulesManager is the func-backed TaskRulesManager double. The
// property-less slice doubles live in a nested struct (ADR 0052) — the two
// contracts stay visibly distinct and the slices never blur in a test.
type fakeTaskRulesManager struct {
	create func(ctx context.Context, actor, propertyID uuid.UUID, cmd application.CreateRuleCommand) (domain.TaskRule, error)
	get    func(ctx context.Context, actor, propertyID, ruleID uuid.UUID) (domain.TaskRule, error)
	update func(ctx context.Context, actor, propertyID, ruleID uuid.UUID, cmd application.UpdateRuleCommand) (domain.TaskRule, error)
	del    func(ctx context.Context, actor, propertyID, ruleID uuid.UUID) error

	withoutProperty struct {
		create func(ctx context.Context, actor uuid.UUID, cmd application.CreateRuleCommand) (domain.TaskRule, error)
		get    func(ctx context.Context, actor, ruleID uuid.UUID) (domain.TaskRule, error)
		update func(ctx context.Context, actor, ruleID uuid.UUID, cmd application.UpdateRuleCommand) (domain.TaskRule, error)
		del    func(ctx context.Context, actor, ruleID uuid.UUID) error
	}
}

func (f *fakeTaskRulesManager) CreateRule(
	ctx context.Context, actor, propertyID uuid.UUID, cmd application.CreateRuleCommand,
) (domain.TaskRule, error) {
	if f.create == nil {
		return domain.TaskRule{}, errors.New("unexpected CreateRule call")
	}
	return f.create(ctx, actor, propertyID, cmd)
}

func (f *fakeTaskRulesManager) GetRule(
	ctx context.Context, actor, propertyID, ruleID uuid.UUID,
) (domain.TaskRule, error) {
	if f.get == nil {
		return domain.TaskRule{}, errors.New("unexpected GetRule call")
	}
	return f.get(ctx, actor, propertyID, ruleID)
}

func (f *fakeTaskRulesManager) UpdateRule(
	ctx context.Context, actor, propertyID, ruleID uuid.UUID, cmd application.UpdateRuleCommand,
) (domain.TaskRule, error) {
	if f.update == nil {
		return domain.TaskRule{}, errors.New("unexpected UpdateRule call")
	}
	return f.update(ctx, actor, propertyID, ruleID, cmd)
}

func (f *fakeTaskRulesManager) DeleteRule(ctx context.Context, actor, propertyID, ruleID uuid.UUID) error {
	if f.del == nil {
		return errors.New("unexpected DeleteRule call")
	}
	return f.del(ctx, actor, propertyID, ruleID)
}

func (f *fakeTaskRulesManager) CreateRuleWithoutProperty(
	ctx context.Context, actor uuid.UUID, cmd application.CreateRuleCommand,
) (domain.TaskRule, error) {
	if f.withoutProperty.create == nil {
		return domain.TaskRule{}, errors.New("unexpected CreateRuleWithoutProperty call")
	}
	return f.withoutProperty.create(ctx, actor, cmd)
}

func (f *fakeTaskRulesManager) GetRuleWithoutProperty(
	ctx context.Context, actor, ruleID uuid.UUID,
) (domain.TaskRule, error) {
	if f.withoutProperty.get == nil {
		return domain.TaskRule{}, errors.New("unexpected GetRuleWithoutProperty call")
	}
	return f.withoutProperty.get(ctx, actor, ruleID)
}

func (f *fakeTaskRulesManager) UpdateRuleWithoutProperty(
	ctx context.Context, actor, ruleID uuid.UUID, cmd application.UpdateRuleCommand,
) (domain.TaskRule, error) {
	if f.withoutProperty.update == nil {
		return domain.TaskRule{}, errors.New("unexpected UpdateRuleWithoutProperty call")
	}
	return f.withoutProperty.update(ctx, actor, ruleID, cmd)
}

func (f *fakeTaskRulesManager) DeleteRuleWithoutProperty(
	ctx context.Context, actor, ruleID uuid.UUID,
) error {
	if f.withoutProperty.del == nil {
		return errors.New("unexpected DeleteRuleWithoutProperty call")
	}
	return f.withoutProperty.del(ctx, actor, ruleID)
}

// fakeTasksManager is the func-backed TasksManager double.
type fakeTasksManager struct {
	list       func(ctx context.Context, actor, propertyID uuid.UUID, q application.TasksListQuery) (application.TasksPage, error)
	listGlobal func(ctx context.Context, actor uuid.UUID, q application.GlobalTasksListQuery) (application.TasksPage, error)
	complete   func(ctx context.Context, actor, propertyID, taskID uuid.UUID) (domain.Task, error)
	uncomplete func(ctx context.Context, actor, propertyID, taskID uuid.UUID) (domain.Task, error)
	clear      func(ctx context.Context, actor, propertyID uuid.UUID) (int64, error)

	completeWithoutProperty   func(ctx context.Context, actor, taskID uuid.UUID) (domain.Task, error)
	uncompleteWithoutProperty func(ctx context.Context, actor, taskID uuid.UUID) (domain.Task, error)
	clearOwnerBook            func(ctx context.Context, actor uuid.UUID) (int64, error)
}

func (f *fakeTasksManager) ListTasks(
	ctx context.Context, actor, propertyID uuid.UUID, q application.TasksListQuery,
) (application.TasksPage, error) {
	if f.list == nil {
		return application.TasksPage{}, errors.New("unexpected ListTasks call")
	}
	return f.list(ctx, actor, propertyID, q)
}

func (f *fakeTasksManager) CompleteTask(
	ctx context.Context, actor, propertyID, taskID uuid.UUID,
) (domain.Task, error) {
	if f.complete == nil {
		return domain.Task{}, errors.New("unexpected CompleteTask call")
	}
	return f.complete(ctx, actor, propertyID, taskID)
}

func (f *fakeTasksManager) UncompleteTask(
	ctx context.Context, actor, propertyID, taskID uuid.UUID,
) (domain.Task, error) {
	if f.uncomplete == nil {
		return domain.Task{}, errors.New("unexpected UncompleteTask call")
	}
	return f.uncomplete(ctx, actor, propertyID, taskID)
}

func (f *fakeTasksManager) ClearCompletedJournal(
	ctx context.Context, actor, propertyID uuid.UUID,
) (int64, error) {
	if f.clear == nil {
		return 0, errors.New("unexpected ClearCompletedJournal call")
	}
	return f.clear(ctx, actor, propertyID)
}

func (f *fakeTasksManager) CompleteTaskWithoutProperty(
	ctx context.Context, actor, taskID uuid.UUID,
) (domain.Task, error) {
	if f.completeWithoutProperty == nil {
		return domain.Task{}, errors.New("unexpected CompleteTaskWithoutProperty call")
	}
	return f.completeWithoutProperty(ctx, actor, taskID)
}

func (f *fakeTasksManager) UncompleteTaskWithoutProperty(
	ctx context.Context, actor, taskID uuid.UUID,
) (domain.Task, error) {
	if f.uncompleteWithoutProperty == nil {
		return domain.Task{}, errors.New("unexpected UncompleteTaskWithoutProperty call")
	}
	return f.uncompleteWithoutProperty(ctx, actor, taskID)
}

func (f *fakeTasksManager) ListGlobalTasks(
	ctx context.Context, actor uuid.UUID, q application.GlobalTasksListQuery,
) (application.TasksPage, error) {
	if f.listGlobal == nil {
		return application.TasksPage{}, errors.New("unexpected ListGlobalTasks call")
	}
	return f.listGlobal(ctx, actor, q)
}

func (f *fakeTasksManager) ClearCompletedJournalOwnerBook(
	ctx context.Context, actor uuid.UUID,
) (int64, error) {
	if f.clearOwnerBook == nil {
		return 0, errors.New("unexpected ClearCompletedJournalOwnerBook call")
	}
	return f.clearOwnerBook(ctx, actor)
}

// fixtureRule is the tests' fixture rule.
func fixtureRule() domain.TaskRule {
	id := uuid.Must(uuid.NewV7())
	due := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	created := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	tod := domain.TimeOfDay(15*60 + 13)
	return domain.TaskRule{
		ID: id, PropertyID: new(uuid.Must(uuid.NewV7())),
		Title: "Проверить счётчики", DueDate: &due, DueTime: &tod,
		Repeat: domain.RepeatWeekly, CreatedAt: created, UpdatedAt: created,
	}
}

// fixtureTask is the tests' fixture task.
func fixtureTask() domain.Task {
	id := uuid.Must(uuid.NewV7())
	ruleID := uuid.Must(uuid.NewV7())
	due := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	created := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	return domain.Task{
		ID: id, PropertyID: new(uuid.Must(uuid.NewV7())), RuleID: &ruleID,
		DueDate: &due, Title: "Вынести мусор",
		CreatedAt: created, UpdatedAt: created,
	}
}

func userRequest(t *testing.T, method, target, body string) *http.Request {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), uuid.Must(uuid.NewV7())),
		method, target, reader,
	)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func TestTaskHandlers_Unauthorized(t *testing.T) {
	t.Parallel()

	rh := NewRuleHandlers(&fakeTaskRulesManager{}, nil)
	th := NewTaskHandlers(&fakeTasksManager{}, nil)
	propertyID := uuid.Must(uuid.NewV7())
	otherID := uuid.Must(uuid.NewV7())

	cases := map[string]func(w http.ResponseWriter, r *http.Request){
		"create rule": func(w http.ResponseWriter, r *http.Request) { rh.CreateTaskRule(w, r, propertyID) },
		"get rule":    func(w http.ResponseWriter, r *http.Request) { rh.GetTaskRule(w, r, propertyID, otherID) },
		"update rule": func(w http.ResponseWriter, r *http.Request) { rh.UpdateTaskRule(w, r, propertyID, otherID) },
		"delete rule": func(w http.ResponseWriter, r *http.Request) { rh.DeleteTaskRule(w, r, propertyID, otherID) },
		"list tasks": func(w http.ResponseWriter, r *http.Request) {
			th.ListPropertyTasks(w, r, propertyID, openapi.ListPropertyTasksParams{})
		},
		"list global tasks": func(w http.ResponseWriter, r *http.Request) {
			th.ListTasks(w, r, openapi.ListTasksParams{})
		},
		"complete":         func(w http.ResponseWriter, r *http.Request) { th.CompleteTask(w, r, propertyID, otherID) },
		"uncomplete":       func(w http.ResponseWriter, r *http.Request) { th.UncompleteTask(w, r, propertyID, otherID) },
		"delete completed": func(w http.ResponseWriter, r *http.Request) { th.DeleteCompletedTasks(w, r, propertyID) },

		"create rule without property": func(w http.ResponseWriter, r *http.Request) {
			rh.CreateTaskRuleWithoutProperty(w, r)
		},
		"get rule without property": func(w http.ResponseWriter, r *http.Request) {
			rh.GetTaskRuleWithoutProperty(w, r, otherID)
		},
		"update rule without property": func(w http.ResponseWriter, r *http.Request) {
			rh.UpdateTaskRuleWithoutProperty(w, r, otherID)
		},
		"delete rule without property": func(w http.ResponseWriter, r *http.Request) {
			rh.DeleteTaskRuleWithoutProperty(w, r, otherID)
		},
		"complete task without property": func(w http.ResponseWriter, r *http.Request) {
			th.CompleteTaskWithoutProperty(w, r, otherID)
		},
		"uncomplete task without property": func(w http.ResponseWriter, r *http.Request) {
			th.UncompleteTaskWithoutProperty(w, r, otherID)
		},
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			call(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", w.Code)
			}
		})
	}
}

func TestRuleHandlers_CreateTaskRule_FoldsBody(t *testing.T) {
	t.Parallel()

	var gotCmd application.CreateRuleCommand
	svc := &fakeTaskRulesManager{
		create: func(
			_ context.Context, _, _ uuid.UUID, cmd application.CreateRuleCommand,
		) (domain.TaskRule, error) {
			gotCmd = cmd
			return fixtureRule(), nil
		},
	}
	h := NewRuleHandlers(svc, nil)

	body := `{"title":"Полить цветы","comment":"Фикус","dueDate":"2026-09-10","dueTime":"15:13","repeat":"weekly"}`
	req := userRequest(t, http.MethodPost, "/tasks/rules", body)
	w := httptest.NewRecorder()
	h.CreateTaskRule(w, req, uuid.Must(uuid.NewV7()))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	if gotCmd.Title != "Полить цветы" || gotCmd.Repeat != domain.RepeatWeekly {
		t.Fatalf("command = %+v", gotCmd)
	}
	if gotCmd.Comment == nil || *gotCmd.Comment != "Фикус" {
		t.Fatalf("comment = %v", gotCmd.Comment)
	}
	if gotCmd.DueDate == nil || gotCmd.DueDate.Format(time.DateOnly) != "2026-09-10" {
		t.Fatalf("dueDate = %v", gotCmd.DueDate)
	}
	if gotCmd.DueTime == nil || gotCmd.DueTime.String() != "15:13" {
		t.Fatalf("dueTime = %v", gotCmd.DueTime)
	}
}

func TestRuleHandlers_CreateTaskRule_UndatedMinimal(t *testing.T) {
	t.Parallel()

	var gotCmd application.CreateRuleCommand
	svc := &fakeTaskRulesManager{
		create: func(
			_ context.Context, _, _ uuid.UUID, cmd application.CreateRuleCommand,
		) (domain.TaskRule, error) {
			gotCmd = cmd
			return fixtureRule(), nil
		},
	}
	h := NewRuleHandlers(svc, nil)

	req := userRequest(t, http.MethodPost, "/tasks/rules", `{"title":"Разобрать кладовку","repeat":"once"}`)
	w := httptest.NewRecorder()
	h.CreateTaskRule(w, req, uuid.Must(uuid.NewV7()))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	if gotCmd.DueDate != nil || gotCmd.DueTime != nil || gotCmd.Comment != nil {
		t.Fatalf("optional fields leaked: %+v", gotCmd)
	}
}

func TestRuleHandlers_UpdateTaskRule_TriState(t *testing.T) {
	t.Parallel()

	var gotCmd application.UpdateRuleCommand
	svc := &fakeTaskRulesManager{
		update: func(
			_ context.Context, _, _, _ uuid.UUID, cmd application.UpdateRuleCommand,
		) (domain.TaskRule, error) {
			gotCmd = cmd
			return fixtureRule(), nil
		},
	}
	h := NewRuleHandlers(svc, nil)

	// Comment cleared, dueDate set, dueTime cleared, title and repeat omitted.
	body := `{"comment":null,"dueDate":"2026-10-01","dueTime":null}`
	req := userRequest(t, http.MethodPatch, "/tasks/rules/x", body)
	w := httptest.NewRecorder()
	h.UpdateTaskRule(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotCmd.Title != nil || gotCmd.Repeat != nil {
		t.Fatalf("omitted fields must not travel: %+v", gotCmd)
	}
	if gotCmd.Comment == nil || gotCmd.Comment.Value != nil {
		t.Fatalf("comment clear lost: %+v", gotCmd.Comment)
	}
	if gotCmd.DueDate == nil || gotCmd.DueDate.Value == nil ||
		gotCmd.DueDate.Value.Format(time.DateOnly) != "2026-10-01" {
		t.Fatalf("dueDate set lost: %+v", gotCmd.DueDate)
	}
	if gotCmd.DueTime == nil || gotCmd.DueTime.Value != nil {
		t.Fatalf("dueTime clear lost: %+v", gotCmd.DueTime)
	}
}

func TestRuleHandlers_UpdateTaskRule_SetsTime(t *testing.T) {
	t.Parallel()

	var gotCmd application.UpdateRuleCommand
	svc := &fakeTaskRulesManager{
		update: func(
			_ context.Context, _, _, _ uuid.UUID, cmd application.UpdateRuleCommand,
		) (domain.TaskRule, error) {
			gotCmd = cmd
			return fixtureRule(), nil
		},
	}
	h := NewRuleHandlers(svc, nil)

	req := userRequest(t, http.MethodPatch, "/tasks/rules/x", `{"dueTime":"09:05"}`)
	w := httptest.NewRecorder()
	h.UpdateTaskRule(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotCmd.DueTime == nil || gotCmd.DueTime.Value == nil || gotCmd.DueTime.Value.String() != "09:05" {
		t.Fatalf("dueTime set lost: %+v", gotCmd.DueTime)
	}
}

func TestTaskHandlers_ListPropertyTasks_FoldsParamsAndBuckets(t *testing.T) {
	t.Parallel()

	var gotQ application.TasksListQuery
	svc := &fakeTasksManager{
		list: func(
			_ context.Context, _, _ uuid.UUID, q application.TasksListQuery,
		) (application.TasksPage, error) {
			gotQ = q
			first := fixtureTask()
			repeat := domain.RepeatWeekly
			first.Repeat = &repeat
			return application.TasksPage{
				Items: []application.TaskListItem{
					{Task: first, Status: domain.ViewOverdue},
					{Task: fixtureTask(), Status: domain.ViewCompleted},
				},
				Total: 7,
				Today: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	}
	h := NewTaskHandlers(svc, nil)

	completed := true
	limit := openapi.TasksLimit(10)
	offset := openapi.TasksOffset(5)
	req := userRequest(t, http.MethodGet, "/tasks", "")
	w := httptest.NewRecorder()
	h.ListPropertyTasks(w, req, uuid.Must(uuid.NewV7()), openapi.ListPropertyTasksParams{
		Completed: &completed,
		Limit:     &limit,
		Offset:    &offset,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !gotQ.Completed || gotQ.Limit != 10 || gotQ.Offset != 5 {
		t.Fatalf("query = %+v", gotQ)
	}

	var resp openapi.TasksResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 2 || resp.Total != 7 {
		t.Fatalf("response = %d items / total %d", len(resp.Items), resp.Total)
	}
	if resp.Items[0].Status != openapi.TaskResponseStatus(domain.ViewOverdue) {
		t.Fatalf("bucket = %q", resp.Items[0].Status)
	}
	if resp.Items[0].Repeat == nil || *resp.Items[0].Repeat != openapi.TaskRepeat(domain.RepeatWeekly) {
		t.Fatalf("repeat = %v, want weekly", resp.Items[0].Repeat)
	}
	if resp.Items[1].Repeat != nil {
		t.Fatalf("repeat without the rule projection = %q, want null", *resp.Items[1].Repeat)
	}
}

func TestTaskHandlers_ListTasks_FoldsParamsAndProjectsName(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	var gotActor uuid.UUID
	var gotQ application.GlobalTasksListQuery
	svc := &fakeTasksManager{
		listGlobal: func(_ context.Context, a uuid.UUID, q application.GlobalTasksListQuery) (application.TasksPage, error) {
			gotActor = a
			gotQ = q
			first := fixtureTask()
			return application.TasksPage{
				Items: []application.TaskListItem{
					{Task: first, Status: domain.ViewOverdue, PropertyName: "Дача"},
					{Task: fixtureTask(), Status: domain.ViewActive},
				},
				Total: 2,
				Today: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	}
	h := NewTaskHandlers(svc, nil)

	propertyID := uuid.Must(uuid.NewV7())
	propertyIDParam := propertyID.String()
	completed := true
	limit := openapi.TasksLimit(10)
	offset := openapi.TasksOffset(5)
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), actor),
		http.MethodGet, "/tasks", nil,
	)
	w := httptest.NewRecorder()
	h.ListTasks(w, req, openapi.ListTasksParams{
		PropertyId: &propertyIDParam,
		Completed:  &completed,
		Limit:      &limit,
		Offset:     &offset,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotActor != actor {
		t.Fatalf("actor = %s, want %s", gotActor, actor)
	}
	if len(gotQ.PropertyIDs) != 1 || gotQ.PropertyIDs[0] != propertyID {
		t.Fatalf("property filter = %v", gotQ.PropertyIDs)
	}
	if gotQ.WithoutProperty {
		t.Fatalf("withoutProperty leaked: %+v", gotQ)
	}
	if !gotQ.Completed || gotQ.Limit != 10 || gotQ.Offset != 5 {
		t.Fatalf("query = %+v", gotQ)
	}

	var resp openapi.TasksResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 2 || resp.Total != 2 {
		t.Fatalf("response = %d items / total %d", len(resp.Items), resp.Total)
	}
	if resp.Items[0].PropertyName == nil || *resp.Items[0].PropertyName != "Дача" {
		t.Fatalf("propertyName = %v, want Дача", resp.Items[0].PropertyName)
	}
	if resp.Items[1].PropertyName != nil {
		t.Fatalf("property-less propertyName = %q, want null", *resp.Items[1].PropertyName)
	}
}

func TestTaskHandlers_ListTasks_FoldsWithoutProperty(t *testing.T) {
	t.Parallel()

	var gotQ application.GlobalTasksListQuery
	svc := &fakeTasksManager{
		listGlobal: func(_ context.Context, _ uuid.UUID, q application.GlobalTasksListQuery) (application.TasksPage, error) {
			gotQ = q
			return application.TasksPage{Items: []application.TaskListItem{}, Total: 0, Today: time.Time{}}, nil
		},
	}
	h := NewTaskHandlers(svc, nil)

	without := true
	req := userRequest(t, http.MethodGet, "/tasks", "")
	w := httptest.NewRecorder()
	h.ListTasks(w, req, openapi.ListTasksParams{WithoutProperty: &without})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !gotQ.WithoutProperty || len(gotQ.PropertyIDs) != 0 {
		t.Fatalf("query = %+v", gotQ)
	}
}

func TestTaskHandlers_ListTasks_MultiPropertyIDs(t *testing.T) {
	t.Parallel()

	first := uuid.Must(uuid.NewV7())
	second := uuid.Must(uuid.NewV7())
	var gotQ application.GlobalTasksListQuery
	svc := &fakeTasksManager{
		listGlobal: func(_ context.Context, _ uuid.UUID, q application.GlobalTasksListQuery) (application.TasksPage, error) {
			gotQ = q
			return application.TasksPage{Items: []application.TaskListItem{}, Total: 0, Today: time.Time{}}, nil
		},
	}
	h := NewTaskHandlers(svc, nil)

	list := first.String() + ", " + second.String() + ","
	req := userRequest(t, http.MethodGet, "/tasks", "")
	w := httptest.NewRecorder()
	h.ListTasks(w, req, openapi.ListTasksParams{PropertyId: &list})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if len(gotQ.PropertyIDs) != 2 || gotQ.PropertyIDs[0] != first || gotQ.PropertyIDs[1] != second {
		t.Fatalf("property filter = %v, want both ids", gotQ.PropertyIDs)
	}
}

func TestTaskHandlers_ListTasks_MalformedPropertyID400(t *testing.T) {
	t.Parallel()

	var listed bool
	svc := &fakeTasksManager{
		listGlobal: func(context.Context, uuid.UUID, application.GlobalTasksListQuery) (application.TasksPage, error) {
			listed = true
			return application.TasksPage{}, nil
		},
	}
	h := NewTaskHandlers(svc, nil)

	garbage := "квартира"
	req := userRequest(t, http.MethodGet, "/tasks", "")
	w := httptest.NewRecorder()
	h.ListTasks(w, req, openapi.ListTasksParams{PropertyId: &garbage})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if listed {
		t.Fatal("the use case must not be reached with a malformed uuid")
	}
}

func TestTaskHandlers_ListTasks_ErrorMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"both property filters 400", application.ErrInvalidInput, http.StatusBadRequest},
		{"foreign propertyId privacy 404", application.ErrNotFound, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := &fakeTasksManager{
				listGlobal: func(
					_ context.Context, _ uuid.UUID, _ application.GlobalTasksListQuery,
				) (application.TasksPage, error) {
					return application.TasksPage{}, tt.err
				},
			}
			h := NewTaskHandlers(svc, nil)
			req := userRequest(t, http.MethodGet, "/tasks", "")
			w := httptest.NewRecorder()
			h.ListTasks(w, req, openapi.ListTasksParams{})
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}

func TestTaskHandlers_ErrorMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"privacy 404", application.ErrNotFound, http.StatusNotFound},
		{"forbidden 403", application.ErrForbidden, http.StatusForbidden},
		{"archived 409", application.ErrArchivedProperty, http.StatusConflict},
		{"already completed 409", application.ErrAlreadyCompleted, http.StatusConflict},
		{"not completed 409", application.ErrNotCompleted, http.StatusConflict},
		{"rule deleted 409", application.ErrRuleDeleted, http.StatusConflict},
		{"invalid input 400", application.ErrInvalidInput, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := &fakeTasksManager{
				complete: func(
					_ context.Context, _, _, _ uuid.UUID,
				) (domain.Task, error) {
					return domain.Task{}, tt.err
				},
			}
			h := NewTaskHandlers(svc, nil)
			req := userRequest(t, http.MethodPost, "/tasks/x/complete", "")
			w := httptest.NewRecorder()
			h.CompleteTask(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}

func TestTaskHandlers_CompleteAndUncompleteAndClear(t *testing.T) {
	t.Parallel()

	svc := &fakeTasksManager{
		complete: func(_ context.Context, _, _, _ uuid.UUID) (domain.Task, error) {
			return fixtureTask(), nil
		},
		uncomplete: func(_ context.Context, _, _, _ uuid.UUID) (domain.Task, error) {
			return fixtureTask(), nil
		},
		clear: func(_ context.Context, _, _ uuid.UUID) (int64, error) {
			return 3, nil
		},
	}
	th := NewTaskHandlers(svc, nil)

	req := userRequest(t, http.MethodPost, "/tasks/x/complete", "")
	w := httptest.NewRecorder()
	th.CompleteTask(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))
	if w.Code != http.StatusOK {
		t.Fatalf("complete status = %d, want 200", w.Code)
	}

	req = userRequest(t, http.MethodPost, "/tasks/x/uncomplete", "")
	w = httptest.NewRecorder()
	th.UncompleteTask(w, req, uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))
	if w.Code != http.StatusOK {
		t.Fatalf("uncomplete status = %d, want 200", w.Code)
	}

	req = userRequest(t, http.MethodDelete, "/tasks/completed", "")
	w = httptest.NewRecorder()
	th.DeleteCompletedTasks(w, req, uuid.Must(uuid.NewV7()))
	if w.Code != http.StatusNoContent {
		t.Fatalf("clear status = %d, want 204", w.Code)
	}
}
