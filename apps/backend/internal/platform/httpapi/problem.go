package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	billingdomain "github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	identitydomain "github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// problem builds an RFC 7807 problem detail with request ID from the context.
func problem(ctx context.Context, title, detail string) openapi.Problem {
	return openapi.Problem{
		Type:      "about:blank",
		Title:     title,
		Detail:    stringPtr(detail),
		RequestId: stringPtr(RequestIDFromContext(ctx)),
	}
}

// internalError logs an internal error and returns a generic problem response.
// The error is sanitized before logging to avoid leaking secrets or PII.
func internalError(ctx context.Context, err error) openapi.Problem {
	loggerFromContext(ctx).ErrorContext(ctx, "internal server error",
		slog.String("error", sanitizeError(err)),
	)
	return problem(ctx, "Internal Server Error", "An internal error occurred")
}

// UserFacingDetail maps known domain errors to fixed, non-sensitive messages
// suitable for RFC 7807 problem details. The second result is false when err
// is not a recognized domain error.
func UserFacingDetail(err error) (string, bool) {
	switch {
	// Identity / auth.
	case errors.Is(err, identityapp.ErrUserBlocked):
		return "user is temporarily blocked", true
	case errors.Is(err, identityapp.ErrCodeSentTooRecently):
		return "code sent too recently", true
	case errors.Is(err, identitydomain.ErrTooManyAttempts):
		return "too many attempts", true

	// Properties.
	case errors.Is(err, propertiesapp.ErrInvalidInput):
		return "invalid property input", true
	case errors.Is(err, propertiesapp.ErrInvalidTransition):
		return "invalid property status transition", true
	case errors.Is(err, propertiesapp.ErrNotFound):
		return "not found", true
	case errors.Is(err, propertiesapp.ErrLimitExceeded):
		return "active property limit exceeded", true
	case errors.Is(err, propertiesapp.ErrArchivedProperty):
		return "cannot modify an archived property", true
	case errors.Is(err, propertiesapp.ErrAlreadyArchived):
		return "property is already archived", true
	case errors.Is(err, propertiesapp.ErrNotArchived):
		return "property is not archived", true
	case errors.Is(err, propertiesapp.ErrPropertyHasOpenLease):
		return "property has an open lease", true

	// Leases.
	case errors.Is(err, leasesapp.ErrInvalidInput):
		return "invalid input", true
	case errors.Is(err, leasesapp.ErrInvalidTransition):
		return "invalid lease status transition", true
	case errors.Is(err, leasesapp.ErrNotFound):
		return "not found", true
	case errors.Is(err, leasesapp.ErrPropertyNotAvailable):
		return "property is not available for a lease", true
	case errors.Is(err, leasesapp.ErrOpenLeaseExists):
		return "property already has an open lease", true
	case errors.Is(err, leasesapp.ErrAlreadyCompleted):
		return "lease is already completed", true
	case errors.Is(err, leasesapp.ErrArchivedLease):
		return "cannot modify an archived lease", true
	case errors.Is(err, leasesapp.ErrTenantContactNotFound):
		return "tenant contact not found", true
	case errors.Is(err, leasesapp.ErrDuplicatePhone):
		return "tenant contact with this phone already exists", true

	// Billing / subscriptions.
	case errors.Is(err, billingdomain.ErrInvalidAmount):
		return "amount must be positive", true
	case errors.Is(err, billingdomain.ErrInvalidPeriod):
		return "period must be month or year", true
	case errors.Is(err, billingdomain.ErrAlreadyOnTariff):
		return "already on selected tariff", true
	case errors.Is(err, billingdomain.ErrInvalidTariffChange):
		return "invalid tariff change", true
	case errors.Is(err, billingdomain.ErrInvalidSubscriptionState):
		return "invalid subscription state", true
	case errors.Is(err, billingdomain.ErrCannotEnableAutoRenew):
		return "cannot enable auto-renew without a validity period", true
	case errors.Is(err, billingapp.ErrPaymentMethodInUse):
		return "payment method is in use", true
	case errors.Is(err, billingapp.ErrPaymentMethodAlreadyExists):
		return "payment method already exists", true

	// Notifications / reminders.
	case errors.Is(err, notificationsapp.ErrInvalidReminderDate):
		return "invalid reminder date", true
	case errors.Is(err, notificationsapp.ErrReminderNotPending):
		return "reminder is not pending", true
	case errors.Is(err, notificationsapp.ErrConcurrentUpdate):
		return "reminder changed concurrently", true
	case errors.Is(err, notificationsapp.ErrDuplicateSMSReminder):
		return "sms reminder already sent", true
	case errors.Is(err, notificationsapp.ErrNotFound):
		return "not found", true
	}

	return "An unexpected error occurred.", false
}

// writeProblem writes an RFC 7807 problem response and records the problem title
// on the response writer so that the logging middleware can include it.
func writeProblem(w http.ResponseWriter, status int, p openapi.Problem) {
	if setter, ok := w.(problemTitleSetter); ok {
		setter.SetProblemTitle(p.Title)
	}
	p.Status = status
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// openAPIErrorHandler converts OpenAPI path/header/param errors into RFC 7807 problems.
func openAPIErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	var (
		requiredParam     *openapi.RequiredParamError
		requiredHeader    *openapi.RequiredHeaderError
		invalidParamFmt   *openapi.InvalidParamFormatError
		tooManyValues     *openapi.TooManyValuesForParamError
		unescapedCookie   *openapi.UnescapedCookieParamError
		unmarshalingParam *openapi.UnmarshalingParamError
	)

	switch {
	case errors.As(err, &requiredParam),
		errors.As(err, &requiredHeader),
		errors.As(err, &invalidParamFmt),
		errors.As(err, &tooManyValues),
		errors.As(err, &unescapedCookie),
		errors.As(err, &unmarshalingParam):
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request parameter"))
	default:
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request parameter"))
	}
}

var (
	// Redact common credential patterns (case-insensitive, optional surrounding quotes).
	tokenPattern    = regexp.MustCompile(`(?i)(token|password|secret|key)\s*[:=]\s*["']?[^\s"'&]+["']?`)
	hexTokenPattern = regexp.MustCompile(`\b[0-9a-fA-F]{32,}\b`)
	b64TokenPattern = regexp.MustCompile(`\b[A-Za-z0-9+/]{40,}={0,2}\b`)
)

const maxSanitizedErrorLength = 1024

// sanitizeError redacts likely secrets and truncates an error string before
// logging. It keeps enough detail for debugging while reducing the risk of
// leaking credentials or raw upstream responses.
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	s = tokenPattern.ReplaceAllString(s, "${1}=[REDACTED]")
	s = hexTokenPattern.ReplaceAllString(s, "[REDACTED]")
	s = b64TokenPattern.ReplaceAllString(s, "[REDACTED]")
	if len(s) > maxSanitizedErrorLength {
		s = s[:maxSanitizedErrorLength] + " [truncated]"
	}
	return strings.TrimSpace(s)
}
