package http

// The task rule endpoints of the tasks context (ADR 0051): the CRUD behind
// the two-step create and the single-screen edit of the screen map
// (resolutions #497/#500/#502). The shared package doc lives in errors.go.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/domain"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// TaskRulesManager is the consumer-side port of the rule endpoints (ADR
// 0035): the rule CRUD use cases. The concrete application service satisfies
// it; the handler tests run against func-backed fakes.
type TaskRulesManager interface {
	CreateRule(ctx context.Context, actor, propertyID uuid.UUID, cmd application.CreateRuleCommand) (domain.TaskRule, error)
	GetRule(ctx context.Context, actor, propertyID, ruleID uuid.UUID) (domain.TaskRule, error)
	UpdateRule(ctx context.Context, actor, propertyID, ruleID uuid.UUID, cmd application.UpdateRuleCommand) (domain.TaskRule, error)
	DeleteRule(ctx context.Context, actor, propertyID, ruleID uuid.UUID) error
}

// RuleHandlers implements the generated task rule endpoints.
type RuleHandlers struct {
	svc    TaskRulesManager
	logger *slog.Logger
}

// NewRuleHandlers creates HTTP handlers for the task rule API.
func NewRuleHandlers(svc TaskRulesManager, logger *slog.Logger) *RuleHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &RuleHandlers{svc: svc, logger: logger}
}

// CreateTaskRule implements POST /properties/{propertyId}/tasks/rules. The
// first materialization (the undated task, today's tasks and the single
// future) happens inside the use case's transaction.
func (h *RuleHandlers) CreateTaskRule(w http.ResponseWriter, r *http.Request, propertyID openapi_types.UUID) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	var body openapi.TaskRuleCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create task rule request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}
	cmd, err := createCommand(body)
	if err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}
	rule, err := h.svc.CreateRule(r.Context(), actor, propertyID, cmd)
	if err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, taskRuleResponse(rule))
}

// GetTaskRule implements GET /properties/{propertyId}/tasks/rules/{ruleId} —
// the edit screen's load. A stranger or a foreign rule is the privacy 404.
func (h *RuleHandlers) GetTaskRule(w http.ResponseWriter, r *http.Request, propertyID, ruleID openapi_types.UUID) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	rule, err := h.svc.GetRule(r.Context(), actor, propertyID, ruleID)
	if err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, taskRuleResponse(rule))
}

// UpdateTaskRule implements PATCH /properties/{propertyId}/tasks/rules/{ruleId}.
// The change invalidates the not-yet-due uncompleted tasks; the tick stands
// the single future again with fresh snapshots.
func (h *RuleHandlers) UpdateTaskRule(w http.ResponseWriter, r *http.Request, propertyID, ruleID openapi_types.UUID) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	var body openapi.TaskRuleUpdateRequest
	cmd, err := h.decodeUpdateBody(w, r, &body)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update task rule request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}
	rule, err := h.svc.UpdateRule(r.Context(), actor, propertyID, ruleID, cmd)
	if err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, taskRuleResponse(rule))
}

// DeleteTaskRule implements DELETE
// /properties/{propertyId}/tasks/rules/{ruleId}: the hard deletion — the
// uncompleted tasks die, the completed journal survives. The v1 UI does not
// call it (resolution #497); the contract is ready.
func (h *RuleHandlers) DeleteTaskRule(w http.ResponseWriter, r *http.Request, propertyID, ruleID openapi_types.UUID) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	if err := h.svc.DeleteRule(r.Context(), actor, propertyID, ruleID); err != nil {
		handleTaskError(h.logger, w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// decodeUpdateBody reads the update body once and decodes it two ways: the
// strict contract decode (bounded, unknown fields rejected) and a shadow
// pass that preserves the tri-state comment/dueDate/dueTime. The shadow
// exists because encoding/json collapses absent and null onto the same nil
// pointer for every optional field, while a non-pointer json.RawMessage
// receives the raw "null" bytes — that distinction is exactly the PATCH
// semantics (omit keeps, null clears, a value sets).
func (h *RuleHandlers) decodeUpdateBody(
	w http.ResponseWriter, r *http.Request, body *openapi.TaskRuleUpdateRequest,
) (application.UpdateRuleCommand, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpsupport.MaxRequestBodySize))
	if err != nil {
		return application.UpdateRuleCommand{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(body); err != nil {
		return application.UpdateRuleCommand{}, err
	}
	var shadow struct {
		Comment json.RawMessage `json:"comment"`
		DueDate json.RawMessage `json:"dueDate"`
		DueTime json.RawMessage `json:"dueTime"`
	}
	if err := json.Unmarshal(raw, &shadow); err != nil {
		return application.UpdateRuleCommand{}, err
	}

	cmd := application.UpdateRuleCommand{}
	if body.Title != nil {
		cmd.Title = body.Title
	}
	if body.Repeat != nil {
		repeat := domain.RepeatKind(*body.Repeat)
		cmd.Repeat = &repeat
	}
	// A nil raw message means omitted — no change; "null" clears; a value
	// sets.
	comment, commentSet, err := commentUpdateFromWire(shadow.Comment)
	if err != nil {
		return application.UpdateRuleCommand{}, err
	}
	if commentSet {
		cmd.Comment = &comment
	}
	dueDate, dueDateSet, err := dueDateUpdateFromWire(shadow.DueDate)
	if err != nil {
		return application.UpdateRuleCommand{}, err
	}
	if dueDateSet {
		cmd.DueDate = &dueDate
	}
	dueTime, dueTimeSet, err := dueTimeUpdateFromWire(shadow.DueTime)
	if err != nil {
		return application.UpdateRuleCommand{}, err
	}
	if dueTimeSet {
		cmd.DueTime = &dueTime
	}
	return cmd, nil
}

// createCommand folds the create body onto the application command. The
// contract rules live in the rule module (validateRule); the transport only
// builds.
func createCommand(body openapi.TaskRuleCreateRequest) (application.CreateRuleCommand, error) {
	var dueTime *domain.TimeOfDay
	if body.DueTime != nil {
		parsed, err := parseTimeOfDay(*body.DueTime)
		if err != nil {
			return application.CreateRuleCommand{}, err
		}
		dueTime = parsed
	}
	return application.CreateRuleCommand{
		Title:   body.Title,
		Comment: body.Comment,
		DueDate: wireDateToDomain(body.DueDate),
		DueTime: dueTime,
		Repeat:  domain.RepeatKind(body.Repeat),
	}, nil
}

// jsonNull is the wire literal of an explicit PATCH clear.
const jsonNull = "null"

// commentUpdateFromWire resolves the tri-state comment from its raw wire
// bytes: nil raw = omitted (present=false), "null" = clear (present=true,
// nil value), a string = set. The bool is the "the command field is set"
// verdict; the update value is zero for a clear.
func commentUpdateFromWire(raw json.RawMessage) (application.CommentUpdate, bool, error) {
	if len(raw) == 0 {
		return application.CommentUpdate{}, false, nil
	}
	if string(raw) == jsonNull {
		return application.CommentUpdate{}, true, nil
	}
	var value *string
	if err := json.Unmarshal(raw, &value); err != nil {
		return application.CommentUpdate{}, false, application.ErrInvalidInput
	}
	if value != nil && utf8Len(*value) > application.MaxCommentLength {
		return application.CommentUpdate{}, false, application.ErrInvalidInput
	}
	return application.CommentUpdate{Value: value}, true, nil
}

// dueDateUpdateFromWire resolves the tri-state dueDate the same way.
func dueDateUpdateFromWire(raw json.RawMessage) (application.DueDateUpdate, bool, error) {
	if len(raw) == 0 {
		return application.DueDateUpdate{}, false, nil
	}
	if string(raw) == jsonNull {
		return application.DueDateUpdate{}, true, nil
	}
	var value *openapi_types.Date
	if err := json.Unmarshal(raw, &value); err != nil {
		return application.DueDateUpdate{}, false, application.ErrInvalidInput
	}
	return application.DueDateUpdate{Value: wireDateToDomain(value)}, true, nil
}

// dueTimeUpdateFromWire resolves the tri-state dueTime the same way.
func dueTimeUpdateFromWire(raw json.RawMessage) (application.DueTimeUpdate, bool, error) {
	if len(raw) == 0 {
		return application.DueTimeUpdate{}, false, nil
	}
	if string(raw) == jsonNull {
		return application.DueTimeUpdate{}, true, nil
	}
	var value *openapi.TaskTime
	if err := json.Unmarshal(raw, &value); err != nil {
		return application.DueTimeUpdate{}, false, application.ErrInvalidInput
	}
	var parsed *domain.TimeOfDay
	if value != nil {
		p, err := parseTimeOfDay(*value)
		if err != nil {
			return application.DueTimeUpdate{}, false, err
		}
		parsed = p
	}
	return application.DueTimeUpdate{Value: parsed}, true, nil
}

// parseTimeOfDay converts the wire HH:MM into the domain's minutes-since-
// midnight.
func parseTimeOfDay(t openapi.TaskTime) (*domain.TimeOfDay, error) {
	tod, err := domain.ParseTimeOfDay(t)
	if err != nil {
		return nil, application.ErrInvalidInput
	}
	return &tod, nil
}

// wireDateToDomain converts the wire date pointer onto the domain's calendar
// date pointer.
func wireDateToDomain(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}
	return new(d.Time)
}

// utf8Len counts runes — the comment limit is a product copy limit, not a
// byte limit (the application validator re-checks authoritatively).
func utf8Len(s string) int {
	return utf8.RuneCountInString(s)
}

// taskRuleResponse maps one rule onto the wire response shape.
func taskRuleResponse(rule domain.TaskRule) openapi.TaskRuleResponse {
	return openapi.TaskRuleResponse{
		Id:         rule.ID,
		PropertyId: rule.PropertyID,
		Title:      rule.Title,
		Comment:    copyStringPtr(rule.Comment),
		DueDate:    httpsupport.DatePtrToOpenAPI(rule.DueDate),
		DueTime:    dueTimeToWire(rule.DueTime),
		Repeat:     openapi.TaskRepeat(rule.Repeat),
		CreatedAt:  rule.CreatedAt,
		UpdatedAt:  rule.UpdatedAt,
	}
}

// dueTimeToWire converts the optional domain time-of-day into the wire
// HH:MM pointer form.
func dueTimeToWire(t *domain.TimeOfDay) *openapi.TaskTime {
	if t == nil {
		return nil
	}
	s := t.String()
	return &s
}
