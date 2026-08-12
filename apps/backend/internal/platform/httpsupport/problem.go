package httpsupport

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	adminapp "github.com/nambers/arenda-planform/apps/backend/internal/admin/application"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	billingdomain "github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
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

// UserFacingDetail maps known domain errors to fixed, non-sensitive messages
// suitable for RFC 7807 problem details. The second result is false when err
// is not a recognized domain error.
func UserFacingDetail(err error) (string, bool) {
	switch {
	// Properties.
	case errors.Is(err, propertiesapp.ErrInvalidInput):
		return "Некорректные данные объекта", true
	case errors.Is(err, propertiesapp.ErrInvalidTransition):
		return "Некорректный переход статуса объекта", true
	case errors.Is(err, propertiesapp.ErrNotFound):
		return "Не найдено", true
	case errors.Is(err, propertiesapp.ErrLimitExceeded):
		return "Превышен лимит активных объектов", true
	case errors.Is(err, propertiesapp.ErrArchivedProperty):
		return "Нельзя изменить архивный объект", true
	case errors.Is(err, propertiesapp.ErrAlreadyArchived):
		return "Объект уже в архиве", true
	case errors.Is(err, propertiesapp.ErrNotArchived):
		return "Объект не в архиве", true
	case errors.Is(err, propertiesapp.ErrPropertyHasOpenLease):
		return "У объекта есть открытая аренда", true

	// Leases.
	case errors.Is(err, leasesapp.ErrInvalidInput):
		return "Некорректные данные", true
	case errors.Is(err, leasesapp.ErrInvalidTransition):
		return "Некорректный переход статуса аренды", true
	case errors.Is(err, leasesapp.ErrNotFound):
		return "Не найдено", true
	case errors.Is(err, leasesapp.ErrPropertyNotAvailable):
		return "Объект недоступен для аренды", true
	case errors.Is(err, leasesapp.ErrOpenLeaseExists):
		return "У объекта уже есть открытая аренда", true
	case errors.Is(err, leasesapp.ErrAlreadyCompleted):
		return "Аренда уже завершена", true
	case errors.Is(err, leasesapp.ErrOperationAlreadyCompleted):
		return "Операция уже завершена", true
	case errors.Is(err, leasesapp.ErrRecurringOperationLeaseCreated):
		return "Серию, созданную договором аренды, нельзя удалить", true
	case errors.Is(err, leasesapp.ErrArchivedLease):
		return "Нельзя изменить архивную аренду", true
	case errors.Is(err, leasesapp.ErrArchivedProperty):
		return "Объект в архиве", true
	case errors.Is(err, leasesapp.ErrTenantContactNotFound):
		return "Арендатор не найден", true
	case errors.Is(err, leasesapp.ErrDuplicatePhone):
		return "Арендатор с таким телефоном уже существует", true

	// Billing / subscriptions.
	case errors.Is(err, billingdomain.ErrInvalidAmount):
		return "Сумма должна быть больше нуля", true
	case errors.Is(err, billingdomain.ErrInvalidPeriod):
		return "Период должен быть месяц или год", true
	case errors.Is(err, billingdomain.ErrInvalidTariff):
		return "Некорректное название тарифа", true
	case errors.Is(err, billingdomain.ErrAlreadyOnTariff):
		return "Вы уже на выбранном тарифе", true
	case errors.Is(err, billingdomain.ErrInvalidTariffChange):
		return "Некорректная смена тарифа", true
	case errors.Is(err, billingdomain.ErrInvalidSubscriptionState):
		return "Некорректное состояние подписки", true
	case errors.Is(err, billingdomain.ErrInvalidPaymentStatus):
		return "Возврат платежа невозможен в текущем статусе", true
	case errors.Is(err, billingdomain.ErrCannotEnableAutoRenew):
		return "Нельзя включить автопродление без срока действия", true
	case errors.Is(err, billingapp.ErrPaymentMethodInUse):
		return "Способ оплаты используется", true
	case errors.Is(err, billingapp.ErrPaymentMethodAlreadyExists):
		return "Способ оплаты уже добавлен", true
	case errors.Is(err, billingapp.ErrInvalidFilter):
		return "Некорректный фильтр", true

	// Admin.
	case errors.Is(err, adminapp.ErrInvalidFilter):
		return "Некорректный параметр фильтра или сортировки", true

	// Notifications / reminders.
	case errors.Is(err, notificationsapp.ErrInvalidReminderDate):
		return "Некорректная дата напоминания", true
	case errors.Is(err, notificationsapp.ErrReminderNotPending):
		return "Напоминание не в статусе ожидания", true
	case errors.Is(err, notificationsapp.ErrConcurrentUpdate):
		return "Напоминание изменено одновременно", true
	case errors.Is(err, notificationsapp.ErrDuplicateSMSReminder):
		return "SMS-напоминание уже отправлено", true
	case errors.Is(err, notificationsapp.ErrNotFound):
		return "Не найдено", true
	}

	return "", false
}

// WriteProblem writes an RFC 7807 problem response and records the problem title
// on the response writer so that the logging middleware can include it.
func WriteProblem(w http.ResponseWriter, status int, p openapi.Problem) {
	if setter, ok := w.(problemTitleSetter); ok {
		setter.SetProblemTitle(p.Title)
	}
	p.Status = status
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.Error("failed to encode problem response", slog.String("error", SanitizeError(err))) //nolint:sloglint // WriteProblem has no request context in its signature; last-resort encoder fallback
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
	WriteProblem(w, http.StatusBadRequest, Problem(r.Context(), "Bad request", "Некорректный параметр запроса"))
}
