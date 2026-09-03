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

// fakeTaskRulesManager is the func-backed TaskRulesManager double.
type fakeTaskRulesManager struct {
	create func(ctx context.Context, actor, propertyID uuid.UUID, cmd application.CreateRuleCommand) (domain.TaskRule, error)
	get    func(ctx context.Context, actor, propertyID, ruleID uuid.UUID) (domain.TaskRule, error)
	update func(ctx context.Context, actor, propertyID, ruleID uuid.UUID, cmd application.UpdateRuleCommand) (domain.TaskRule, error)
	del    func(ctx context.Context, actor, propertyID, ruleID uuid.UUID) error
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

// fakeTasksManager is the func-backed TasksManager double.
type fakeTasksManager struct {
	list       func(ctx context.Context, actor, propertyID uuid.UUID, q application.TasksListQuery) (application.TasksPage, error)
	complete   func(ctx context.Context, actor, propertyID, taskID uuid.UUID) (domain.Task, error)
	uncomplete func(ctx context.Context, actor, propertyID, taskID uuid.UUID) (domain.Task, error)
	clear      func(ctx context.Context, actor, propertyID uuid.UUID) (int64, error)
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

// fixtureRule is the tests' fixture rule.
func fixtureRule() domain.TaskRule {
	id := uuid.Must(uuid.NewV7())
	due := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	created := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	tod := domain.TimeOfDay(15*60 + 13)
	return domain.TaskRule{
		ID: id, PropertyID: uuid.Must(uuid.NewV7()),
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
		ID: id, PropertyID: uuid.Must(uuid.NewV7()), RuleID: &ruleID,
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
		"complete":         func(w http.ResponseWriter, r *http.Request) { th.CompleteTask(w, r, propertyID, otherID) },
		"uncomplete":       func(w http.ResponseWriter, r *http.Request) { th.UncompleteTask(w, r, propertyID, otherID) },
		"delete completed": func(w http.ResponseWriter, r *http.Request) { th.DeleteCompletedTasks(w, r, propertyID) },
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
