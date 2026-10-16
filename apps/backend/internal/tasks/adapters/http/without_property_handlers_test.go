package http

// The property-less transport tests (ADR 0052): the global paths wire the
// actor straight to the owner-scope use cases — no property parameter, the
// propertyId rides the responses as null, and the tasks error table maps the
// application vocabulary (privacy 404, 409 conflicts).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wireRule is the JSON view of a rule response, unmarshalled loosely so the
// null propertyId is observable.
type wireRule struct {
	ID         string  `json:"id"`
	PropertyID *string `json:"propertyId"`
	Title      string  `json:"title"`
	DueDate    *string `json:"dueDate"`
	Repeat     string  `json:"repeat"`
}

func TestRuleHandlers_CreateTaskRuleWithoutProperty(t *testing.T) {
	t.Parallel()

	var gotActor uuid.UUID
	var gotCmd application.CreateRuleCommand
	svc := &fakeTaskRulesManager{}
	svc.withoutProperty.create = func(
		_ context.Context, actor uuid.UUID, cmd application.CreateRuleCommand,
	) (domain.TaskRule, error) {
		gotActor = actor
		gotCmd = cmd
		rule := fixtureRule()
		rule.PropertyID = nil
		return rule, nil
	}
	rh := NewRuleHandlers(svc, nil)

	body := `{"title":"Позвонить бухгалтеру","repeat":"once"}`
	w := httptest.NewRecorder()
	rh.CreateTaskRuleWithoutProperty(w, userRequest(t, http.MethodPost, "/tasks/rules", body))

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.NotEqual(t, uuid.Nil, gotActor)
	assert.Equal(t, "Позвонить бухгалтеру", gotCmd.Title)
	assert.Equal(t, domain.RepeatOnce, gotCmd.Repeat)

	var wire wireRule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wire))
	assert.Nil(t, wire.PropertyID, "propertyId must be null on the property-less slice")
}

func TestRuleHandlers_GetTaskRuleWithoutProperty_PrivacyNotFound(t *testing.T) {
	t.Parallel()

	svc := &fakeTaskRulesManager{}
	svc.withoutProperty.get = func(
		_ context.Context, _, _ uuid.UUID,
	) (domain.TaskRule, error) {
		return domain.TaskRule{}, application.ErrNotFound
	}
	rh := NewRuleHandlers(svc, nil)

	w := httptest.NewRecorder()
	rh.GetTaskRuleWithoutProperty(w, userRequest(t, http.MethodGet, "/tasks/rules/x", ""),
		openapiUUID(t))

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestRuleHandlers_UpdateTaskRuleWithoutProperty_FoldsPatch(t *testing.T) {
	t.Parallel()

	var gotCmd application.UpdateRuleCommand
	svc := &fakeTaskRulesManager{}
	svc.withoutProperty.update = func(
		_ context.Context, _, _ uuid.UUID, cmd application.UpdateRuleCommand,
	) (domain.TaskRule, error) {
		gotCmd = cmd
		rule := fixtureRule()
		rule.PropertyID = nil
		return rule, nil
	}
	rh := NewRuleHandlers(svc, nil)

	// Tri-state patch: the title is renamed, the comment is cleared.
	body := `{"title":"Другое дело","comment":null}`
	w := httptest.NewRecorder()
	rh.UpdateTaskRuleWithoutProperty(w, userRequest(t, http.MethodPatch, "/tasks/rules/x", body),
		openapiUUID(t))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, gotCmd.Title, "a present title must travel")
	assert.Equal(t, "Другое дело", *gotCmd.Title)
	require.NotNil(t, gotCmd.Comment, "an explicit null comment must travel as a set field")
	assert.Nil(t, gotCmd.Comment.Value, "null clears the comment")
	assert.Nil(t, gotCmd.DueDate, "an omitted dueDate stays unset")

	var wire wireRule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wire))
	assert.Nil(t, wire.PropertyID)
}

func TestRuleHandlers_DeleteTaskRuleWithoutProperty(t *testing.T) {
	t.Parallel()

	var gotRuleID uuid.UUID
	svc := &fakeTaskRulesManager{}
	svc.withoutProperty.del = func(_ context.Context, _, ruleID uuid.UUID) error {
		gotRuleID = ruleID
		return nil
	}
	rh := NewRuleHandlers(svc, nil)

	ruleID := uuid.Must(uuid.NewV7())
	w := httptest.NewRecorder()
	rh.DeleteTaskRuleWithoutProperty(w, userRequest(t, http.MethodDelete, "/tasks/rules/x", ""), ruleID)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, ruleID, gotRuleID)
}

func TestTaskHandlers_CompleteTaskWithoutProperty(t *testing.T) {
	t.Parallel()

	completedDay := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	svc := &fakeTasksManager{
		completeWithoutProperty: func(
			_ context.Context, _, taskID uuid.UUID,
		) (domain.Task, error) {
			task := fixtureTask()
			task.ID = taskID
			task.PropertyID = nil
			task.CompletedDate = &completedDay
			return task, nil
		},
	}
	th := NewTaskHandlers(svc, nil)

	taskID := uuid.Must(uuid.NewV7())
	w := httptest.NewRecorder()
	th.CompleteTaskWithoutProperty(w, userRequest(t, http.MethodPost, "/tasks/x/complete", ""), taskID)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var wire struct {
		ID         string  `json:"id"`
		PropertyID *string `json:"propertyId"`
		Status     string  `json:"status"`
		Completed  *string `json:"completedDate"`
		RuleID     *string `json:"ruleId"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wire))
	assert.Equal(t, taskID.String(), wire.ID)
	assert.Nil(t, wire.PropertyID)
	assert.Equal(t, "completed", wire.Status)
	require.NotNil(t, wire.Completed)
	assert.Equal(t, "2026-09-10", *wire.Completed)
}

func TestTaskHandlers_UncompleteTaskWithoutProperty_MapsConflicts(t *testing.T) {
	t.Parallel()

	svc := &fakeTasksManager{
		uncompleteWithoutProperty: func(
			_ context.Context, _, _ uuid.UUID,
		) (domain.Task, error) {
			return domain.Task{}, application.ErrRuleDeleted
		},
	}
	th := NewTaskHandlers(svc, nil)

	w := httptest.NewRecorder()
	th.UncompleteTaskWithoutProperty(w, userRequest(t, http.MethodPost, "/tasks/x/uncomplete", ""),
		uuid.Must(uuid.NewV7()))

	require.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "Отменить выполнение можно, пока правило живо")
}

func TestRuleHandlers_CreateTaskRuleWithoutProperty_MapsInvalidInput(t *testing.T) {
	t.Parallel()

	svc := &fakeTaskRulesManager{}
	svc.withoutProperty.create = func(
		_ context.Context, _ uuid.UUID, _ application.CreateRuleCommand,
	) (domain.TaskRule, error) {
		return domain.TaskRule{}, application.ErrInvalidInput
	}
	rh := NewRuleHandlers(svc, nil)

	w := httptest.NewRecorder()
	rh.CreateTaskRuleWithoutProperty(w, userRequest(t, http.MethodPost, "/tasks/rules", `{"title":"","repeat":"once"}`))

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTaskHandlers_DeleteCompletedTasksWithoutProperty(t *testing.T) {
	t.Parallel()

	var gotActor uuid.UUID
	svc := &fakeTasksManager{
		clearOwnerBook: func(_ context.Context, actor uuid.UUID) (int64, error) {
			gotActor = actor
			return 4, nil
		},
	}
	th := NewTaskHandlers(svc, nil)

	w := httptest.NewRecorder()
	th.DeleteCompletedTasksWithoutProperty(w, userRequest(t, http.MethodDelete, "/tasks/completed", ""))

	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.NotEqual(t, uuid.Nil, gotActor, "the actor rides straight into the owner-scope use case")
}

func TestTaskHandlers_DeleteCompletedTasksWithoutProperty_MapsStoreFailure(t *testing.T) {
	t.Parallel()

	svc := &fakeTasksManager{
		clearOwnerBook: func(context.Context, uuid.UUID) (int64, error) {
			return 0, errors.New("boom")
		},
	}
	th := NewTaskHandlers(svc, nil)

	w := httptest.NewRecorder()
	th.DeleteCompletedTasksWithoutProperty(w, userRequest(t, http.MethodDelete, "/tasks/completed", ""))

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Internal Server Error")
}

// openapiUUID mints a path-parameter id; openapi_types.UUID is an alias of
// uuid.UUID, so no conversion is involved.
func openapiUUID(t *testing.T) openapi_types.UUID {
	t.Helper()
	return uuid.Must(uuid.NewV7())
}
