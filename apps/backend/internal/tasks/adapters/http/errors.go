// Package http holds the tasks transport adapters: the rule CRUD handlers,
// the task handlers (listings, completion toggle, journal clear) and the
// shared error mapping. Generated OpenAPI DTOs stay at this edge and are
// mapped explicitly onto the application commands (ADR 0002); the property
// access matrix is enforced in the application layer over the policy port
// (ADR 0028).
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
)

// staticTaskProblems maps the tasks application errors with a fixed outcome
// onto their wire status, title and detail (the shared ErrorProblem table
// shape). Invalid input is handled dynamically through the shared
// user-facing table like in every context.
var staticTaskProblems = []httpsupport.ErrorProblem{
	{
		Err: application.ErrNotFound, Status: http.StatusNotFound,
		Title: httpsupport.ProblemTitleNotFound, Detail: "Не найдено",
	},
	{
		Err: application.ErrForbidden, Status: http.StatusForbidden,
		Title: httpsupport.ProblemTitleForbidden, Detail: "Недостаточно прав для этого действия",
	},
	{
		Err: application.ErrArchivedProperty, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "Нельзя изменить архивный объект",
	},
	{
		Err: application.ErrAlreadyCompleted, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "Задача уже выполнена",
	},
	{
		Err: application.ErrNotCompleted, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "Задача ещё не выполнена",
	},
	{
		Err: application.ErrRuleDeleted, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "Отменить выполнение можно, пока правило живо",
	},
}

// writeTasksError maps an error with a fixed outcome onto the wire via the
// tasks problem table, then handles invalid input through its user-facing
// detail. The bool verdict lets each handler turn anything unrecognized into
// its own opaque 500.
func writeTasksError(ctx context.Context, w http.ResponseWriter, err error) bool {
	if httpsupport.WriteErrorProblem(ctx, w, err, staticTaskProblems) {
		return true
	}
	if errors.Is(err, application.ErrInvalidInput) {
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			return false
		}
		httpsupport.WriteProblem(ctx, w, http.StatusBadRequest, httpsupport.Problem(ctx, "Bad request", detail))
		return true
	}
	return false
}

// writeInternal logs the unexpected error and answers the opaque 500.
func writeInternal(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	logger.ErrorContext(r.Context(), "tasks use case failed",
		slog.String("error", httpsupport.SanitizeError(err)))
	httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError,
		httpsupport.InternalError(r.Context(), err))
}

// handleTaskError is the tasks handlers' shared error mapper.
func handleTaskError(logger *slog.Logger, w http.ResponseWriter, r *http.Request, err error) {
	if writeTasksError(r.Context(), w, err) {
		return
	}
	writeInternal(logger, w, r, err)
}
