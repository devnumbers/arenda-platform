package http

// The task endpoints of the tasks context (ADR 0051): the property-wide
// listing (the list screen's sections), the completion toggle behind the row
// circle tap and the «Удалить все выполненные» journal clear (resolution
// #497). The shared package doc lives in errors.go.

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// TasksManager is the consumer-side port of the task endpoints (ADR 0035):
// the listing, the completion toggle and the journal clear. The concrete
// application service satisfies it; the handler tests run against
// func-backed fakes.
type TasksManager interface {
	ListTasks(ctx context.Context, actor, propertyID uuid.UUID, q application.TasksListQuery) (application.TasksPage, error)
	CompleteTask(ctx context.Context, actor, propertyID, taskID uuid.UUID) (domain.Task, error)
	UncompleteTask(ctx context.Context, actor, propertyID, taskID uuid.UUID) (domain.Task, error)
	ClearCompletedJournal(ctx context.Context, actor, propertyID uuid.UUID) (int64, error)
	CompleteTaskWithoutProperty(ctx context.Context, actor, taskID uuid.UUID) (domain.Task, error)
	UncompleteTaskWithoutProperty(ctx context.Context, actor, taskID uuid.UUID) (domain.Task, error)
	ListGlobalTasks(ctx context.Context, actor uuid.UUID, q application.GlobalTasksListQuery) (application.TasksPage, error)
	ClearCompletedJournalOwnerBook(ctx context.Context, actor uuid.UUID) (int64, error)
}

// TaskHandlers implements the generated task endpoints.
type TaskHandlers struct {
	svc    TasksManager
	logger *slog.Logger
}

// NewTaskHandlers creates HTTP handlers for the task API.
func NewTaskHandlers(svc TasksManager, logger *slog.Logger) *TaskHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &TaskHandlers{svc: svc, logger: logger}
}

// ListPropertyTasks implements GET /properties/{propertyId}/tasks. The view
// buckets are computed server-side (minute-precision overdue included); the
// response carries the owner's today for the client's section bucketing.
func (h *TaskHandlers) ListPropertyTasks(
	w http.ResponseWriter, r *http.Request, propertyID openapi_types.UUID,
	params openapi.ListPropertyTasksParams,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	q := application.TasksListQuery{Completed: params.Completed != nil && *params.Completed}
	if params.Limit != nil {
		q.Limit = *params.Limit
	}
	if params.Offset != nil {
		q.Offset = *params.Offset
	}
	page, err := h.svc.ListTasks(r.Context(), actor, propertyID, q)
	if err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}
	writeTasksPage(r, w, page)
}

// writeTasksPage maps one listing page onto the wire's TasksResponse: the
// items through taskResponse (status stays the server-computed bucket,
// propertyName the global listing's row projection), the filter's total and
// the today boundary for the client's date sections.
func writeTasksPage(r *http.Request, w http.ResponseWriter, page application.TasksPage) {
	items := make([]openapi.TaskResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, taskResponse(item))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.TasksResponse{
		Items: items,
		Total: page.Total,
		Today: openapi_types.Date{Time: page.Today},
	})
}

// ListTasks implements GET /tasks — the global «Задачи» screen (ticket
// #521). The property filters are mutually exclusive; both set is the
// contract's 400 via the use case's ErrInvalidInput. The buckets are
// computed server-side against each item's data owner's moment; the page
// carries the reading actor's today for the section bucketing.
func (h *TaskHandlers) ListTasks(w http.ResponseWriter, r *http.Request, params openapi.ListTasksParams) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	q := application.GlobalTasksListQuery{
		TasksListQuery: application.TasksListQuery{
			Completed: params.Completed != nil && *params.Completed,
		},
		PropertyID:      params.PropertyId,
		WithoutProperty: params.WithoutProperty != nil && *params.WithoutProperty,
	}
	if params.Limit != nil {
		q.Limit = *params.Limit
	}
	if params.Offset != nil {
		q.Offset = *params.Offset
	}
	page, err := h.svc.ListGlobalTasks(r.Context(), actor, q)
	if err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}
	writeTasksPage(r, w, page)
}

// CompleteTask implements POST /properties/{propertyId}/tasks/{taskId}/complete:
// the completion fact is stamped with today in the owner's timezone; a
// repeated completion is the contract's 409.
func (h *TaskHandlers) CompleteTask(
	w http.ResponseWriter, r *http.Request, propertyID, taskID openapi_types.UUID,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	task, err := h.svc.CompleteTask(r.Context(), actor, propertyID, taskID)
	if err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, taskResponse(application.TaskListItem{
		Task:   task,
		Status: domain.ViewCompleted,
	}))
}

// UncompleteTask implements POST
// /properties/{propertyId}/tasks/{taskId}/uncomplete: the completion fact is
// cleared — only while the rule lives; a journal row of a deleted rule is
// the contract's 409.
func (h *TaskHandlers) UncompleteTask(
	w http.ResponseWriter, r *http.Request, propertyID, taskID openapi_types.UUID,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	task, err := h.svc.UncompleteTask(r.Context(), actor, propertyID, taskID)
	if err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, taskResponse(application.TaskListItem{
		Task:   task,
		Status: domain.ViewActive,
	}))
}

// DeleteCompletedTasks implements DELETE
// /properties/{propertyId}/tasks/completed: the completed tasks of deleted
// rules are removed forever; rules and active tasks are untouched.
func (h *TaskHandlers) DeleteCompletedTasks(w http.ResponseWriter, r *http.Request, propertyID openapi_types.UUID) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	if _, err := h.svc.ClearCompletedJournal(r.Context(), actor, propertyID); err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CompleteTaskWithoutProperty implements POST /tasks/{taskId}/complete — the
// property-less slice (ADR 0052): the completion fact is stamped with today
// in the owner's timezone; a repeated completion is the contract's 409. A
// bound task is invisible here: the slices never mix.
func (h *TaskHandlers) CompleteTaskWithoutProperty(w http.ResponseWriter, r *http.Request, taskID openapi_types.UUID) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	task, err := h.svc.CompleteTaskWithoutProperty(r.Context(), actor, taskID)
	if err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, taskResponse(application.TaskListItem{
		Task:   task,
		Status: domain.ViewCompleted,
	}))
}

// UncompleteTaskWithoutProperty implements POST /tasks/{taskId}/uncomplete —
// the property-less slice (ADR 0052): the completion fact is cleared — only
// while the rule lives; a journal row of a deleted rule is the contract's
// 409.
func (h *TaskHandlers) UncompleteTaskWithoutProperty(w http.ResponseWriter, r *http.Request, taskID openapi_types.UUID) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	task, err := h.svc.UncompleteTaskWithoutProperty(r.Context(), actor, taskID)
	if err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, taskResponse(application.TaskListItem{
		Task:   task,
		Status: domain.ViewActive,
	}))
}

// DeleteCompletedTasksWithoutProperty implements DELETE /tasks/completed —
// the global «Задачи» screen's kebab (ticket #536): the book-wide journal
// clear of the actor's own book. The shared-to and archived journals are out
// of its scope by the use case's contract; a book without clearable rows
// still answers 204.
func (h *TaskHandlers) DeleteCompletedTasksWithoutProperty(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	if _, err := h.svc.ClearCompletedJournalOwnerBook(r.Context(), actor); err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// taskResponse maps one listed or just-mutated task onto the wire response:
// status is always the server-computed view bucket; a nil property maps to
// the wire's null (the property-less slice, ADR 0052). The propertyName row
// projection comes from the global listing; the property-scoped endpoints
// and the property-less slice carry no name — the wire's null.
func taskResponse(item application.TaskListItem) openapi.TaskResponse {
	task := item.Task
	return openapi.TaskResponse{
		Id:            task.ID,
		PropertyId:    openAPIUUIDPtr(task.PropertyID),
		PropertyName:  stringPtrOrNil(item.PropertyName),
		RuleId:        openAPIUUIDPtr(task.RuleID),
		DueDate:       httpsupport.DatePtrToOpenAPI(task.DueDate),
		DueTime:       dueTimeToWire(task.DueTime),
		Title:         task.Title,
		Comment:       copyStringPtr(task.Comment),
		Repeat:        repeatToWire(task.Repeat),
		CompletedDate: httpsupport.DatePtrToOpenAPI(task.CompletedDate),
		Status:        openapi.TaskResponseStatus(item.Status),
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
	}
}

// stringPtrOrNil maps the optional row projection onto the wire's nullable
// string: the empty projection is the wire's null.
func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// repeatToWire converts the read projection of the rule's repeat; nil (rule
// deleted or the writer-built task) stays the wire's null.
func repeatToWire(repeat *domain.RepeatKind) *openapi.TaskRepeat {
	if repeat == nil {
		return nil
	}
	wire := openapi.TaskRepeat(*repeat)
	return &wire
}

// openAPIUUIDPtr converts an optional domain UUID into the wire pointer form.
func openAPIUUIDPtr(u *uuid.UUID) *openapi_types.UUID {
	if u == nil {
		return nil
	}
	copied := *u
	return &copied
}

// copyStringPtr copies an optional string without aliasing the domain value.
func copyStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}
