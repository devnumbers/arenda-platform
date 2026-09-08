// Package httpsupport holds HTTP cross-cutting support for all contexts: the middleware chain (session,
// logging, recovery, rate limiting, admin-only, readonly gate), problem mapping and request diagnostics.
package httpsupport

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	adminapp "github.com/nambers/arenda-planform/apps/backend/internal/admin/application"
	billingdomain "github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	contactsapp "github.com/nambers/arenda-planform/apps/backend/internal/contacts/application"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	rentalsapp "github.com/nambers/arenda-planform/apps/backend/internal/rentals/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	tasksapp "github.com/nambers/arenda-planform/apps/backend/internal/tasks/application"
)

// Problem builds an RFC 7807 problem detail with request ID from the context.
func Problem(ctx context.Context, title, detail string) openapi.Problem {
	return openapi.Problem{
		Type:      "about:blank",
		Title:     title,
		Detail:    StringPtr(detail),
		RequestId: StringPtr(RequestIDFromContext(ctx)),
	}
}

// ProblemWithCode builds an RFC 7807 problem detail carrying a machine-readable
// error code for failures the client must distinguish from the generic HTTP
// status (e.g. "membership_suspended", T9).
func ProblemWithCode(ctx context.Context, title, detail, code string) openapi.Problem {
	p := Problem(ctx, title, detail)
	p.Code = StringPtr(code)
	return p
}

// ProblemWithFieldErrors builds an RFC 7807 problem detail carrying field-level
// validation errors bound to specific JSON keys.
func ProblemWithFieldErrors(ctx context.Context, title, detail string, fieldErrors []openapi.ProblemError) openapi.Problem {
	p := Problem(ctx, title, detail)
	if len(fieldErrors) > 0 {
		errs := make([]openapi.ProblemError, len(fieldErrors))
		copy(errs, fieldErrors)
		p.Errors = &errs
	}
	return p
}

// InternalError logs an internal error and returns a generic problem response.
// The error is sanitized before logging to avoid leaking secrets or PII.
func InternalError(ctx context.Context, err error) openapi.Problem {
	LoggerFromContext(ctx).ErrorContext(ctx, "internal server error",
		slog.String("error", SanitizeError(err)),
	)
	return Problem(ctx, "Internal Server Error", "Произошла внутренняя ошибка. Попробуйте позже.")
}

// detailNotFound is the RFC 7807 detail shared by the ErrNotFound mapping of
// every context.
const detailNotFound = "Не найдено"

// Shared problem titles of the fixed error mappings: the same RFC 7807 titles
// recur in every context's table, so they are named once here.
const (
	ProblemTitleConflict  = "Conflict"
	ProblemTitleForbidden = "Forbidden"
	ProblemTitleNotFound  = "Not found"
)

// ErrorProblem pairs an application error sentinel with its fixed wire
// outcome: the status, problem title and detail every occurrence of the error
// maps to. The shape is shared by every context's HTTP adapter; the tables
// themselves stay per-context — which errors map where is the context's own
// knowledge.
type ErrorProblem struct {
	Err    error
	Status int
	Title  string
	Detail string
}

// WriteErrorProblem writes the fixed outcome for err by walking the table
// with errors.Is (wrapped sentinels keep matching). It returns false when
// nothing matched, so the caller falls through to its dynamic mappings.
func WriteErrorProblem(ctx context.Context, w http.ResponseWriter, err error, table []ErrorProblem) bool {
	for _, m := range table {
		if errors.Is(err, m.Err) {
			WriteProblem(ctx, w, m.Status, Problem(ctx, m.Title, m.Detail))
			return true
		}
	}
	return false
}

// userFacingDetails maps known domain errors to fixed, non-sensitive messages
// for RFC 7807 problem details. Entries are checked in order, so the table
// preserves the previous switch's precedence exactly.
var userFacingDetails = []struct {
	err    error
	detail string
}{
	// Properties.
	{propertiesapp.ErrInvalidInput, "Некорректные данные объекта"},
	{propertiesapp.ErrInvalidTransition, "Некорректный переход статуса объекта"},
	{propertiesapp.ErrNotFound, detailNotFound},
	{propertiesapp.ErrLimitExceeded, "Превышен лимит активных объектов"},
	{propertiesapp.ErrArchivedProperty, "Нельзя изменить архивный объект"},
	{propertiesapp.ErrAlreadyArchived, "Объект уже в архиве"},
	{propertiesapp.ErrNotArchived, "Объект не в архиве"},

	// Billing / subscriptions.
	{billingdomain.ErrInvalidPeriod, "Период должен быть месяц или год"},
	{billingdomain.ErrInvalidTariff, "Некорректное название тарифа"},
	{billingdomain.ErrAlreadyOnTariff, "Вы уже на выбранном тарифе"},
	{billingdomain.ErrInvalidTariffChange, "Некорректная смена тарифа"},
	{billingdomain.ErrInvalidSubscriptionState, "Некорректное состояние подписки"},
	{billingdomain.ErrCannotEnableAutoRenew, "Нельзя включить автопродление без срока действия"},
	{billingdomain.ErrInvalidAmount, "Некорректная сумма платежа"},
	{billingdomain.ErrInvalidPayment, "Некорректный платёж"},
	{billingdomain.ErrInvalidPaymentStatus, "Некорректный статус платежа для этой операции"},
	{billingdomain.ErrInvalidTerm, "Некорректный срок служебной подписки"},
	{billingdomain.ErrInvalidGraceExtension, "Некорректное продление льготного периода"},
	{billingdomain.ErrInvalidTariffPricing, "Некорректные цены или лимит тарифа"},

	// Admin.
	{adminapp.ErrInvalidFilter, "Некорректный параметр фильтра или сортировки"},

	// Notifications (preferences, push subscriptions).
	{notificationsapp.ErrNotFound, detailNotFound},

	// Payments (payment rules, ADR 0047).
	{paymentsapp.ErrInvalidInput, "Некорректные данные платежа"},

	// Tasks (task rules and tasks, ADR 0051).
	{tasksapp.ErrInvalidInput, "Некорректные данные задачи"},

	// Contacts (the contact book, ADR 0054).
	{contactsapp.ErrInvalidInput, "Некорректные данные контакта"},

	// Rentals (ADR 0053).
	{rentalsapp.ErrInvalidInput, "Некорректные данные аренды"},
}

// UserFacingDetail maps known domain errors to fixed, non-sensitive messages
// suitable for RFC 7807 problem details. The second result is false when err
// is not a recognized domain error.
func UserFacingDetail(err error) (string, bool) {
	for _, m := range userFacingDetails {
		if errors.Is(err, m.err) {
			return m.detail, true
		}
	}
	return "", false
}

// WriteProblem writes an RFC 7807 problem response and records the problem title
// on the response writer so that the logging middleware can include it.
func WriteProblem(ctx context.Context, w http.ResponseWriter, status int, p openapi.Problem) {
	if setter, ok := w.(problemTitleSetter); ok {
		setter.SetProblemTitle(p.Title)
	}
	p.Status = status
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		LoggerFromContext(ctx).ErrorContext(ctx, "failed to encode problem response", slog.String("error", SanitizeError(err)))
	}
}

// SanitizeError is a thin wrapper around the shared sanitizer so the rest of
// the package can keep using the local helper.
func SanitizeError(err error) string {
	return sanitize.Error(err)
}

func StringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// OpenAPIErrorHandler converts OpenAPI path/header/param errors into RFC 7807 problems.
func OpenAPIErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	WriteProblem(r.Context(), w, http.StatusBadRequest, Problem(r.Context(), "Bad request", "Некорректный параметр запроса"))
}
