package httpapi

import (
	"log/slog"
	"net/http"
	"net/url"
	"unicode/utf8"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

const (
	// clientErrorBodyLimit caps POST /client-errors request bodies; the
	// endpoint accepts short JSON error reports only.
	clientErrorBodyLimit = 8192

	clientErrorMessageMaxLength   = 500
	clientErrorStackMaxLength     = 4000
	clientErrorURLMaxLength       = 500
	clientErrorUserAgentMaxLength = 300
)

// ClientErrorsHandlers implements the public browser error reporting endpoint.
type ClientErrorsHandlers struct {
	limiter *RateLimiter
}

// NewClientErrorsHandlers creates handlers for POST /client-errors.
func NewClientErrorsHandlers(limiter *RateLimiter) *ClientErrorsHandlers {
	return &ClientErrorsHandlers{limiter: limiter}
}

// ReportClientError implements POST /client-errors. Reports are sanitized,
// truncated, and logged to stdout for the log pipeline; nothing is persisted.
func (h *ClientErrorsHandlers) ReportClientError(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	clientIP := requestctx.ClientIPFromContext(ctx)
	if h.limiter != nil && !h.limiter.Allow(clientIP) {
		writeTooManyRequests(w, r, "Превышен лимит запросов")
		return
	}

	var body openapi.ClientErrorReport
	if err := decodeJSONBody(w, r, &body); err != nil {
		loggerFromContext(ctx).WarnContext(ctx, "failed to decode client error report", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(ctx, "Bad request", "Некорректное тело запроса"))
		return
	}
	if !body.App.Valid() || body.Message == "" {
		writeProblem(w, http.StatusBadRequest, problem(ctx, "Bad request", "Некорректное тело запроса"))
		return
	}

	var stack, pageURL string
	if body.Stack != nil {
		stack = *body.Stack
	}
	if body.Url != nil {
		pageURL = *body.Url
	}

	// Admin list filters (e.g. phone) sync into the query string, so the
	// reported page URL is logged without query and fragment.
	safeURL := truncateRunes(stripURLQuery(sanitize.String(pageURL)), clientErrorURLMaxLength)

	loggerFromContext(ctx).LogAttrs(ctx, slog.LevelError, "browser error reported",
		slog.String("source", "browser"),
		slog.String("app", string(body.App)),
		slog.String("message", sanitizeLengthLimited(body.Message, clientErrorMessageMaxLength)),
		slog.String("stack", sanitizeLengthLimited(stack, clientErrorStackMaxLength)),
		slog.String("url", safeURL),
		slog.String("user_agent", truncateRunes(r.UserAgent(), clientErrorUserAgentMaxLength)),
		slog.String("client_ip", clientIP),
	)

	w.WriteHeader(http.StatusNoContent)
}

// stripURLQuery drops the query and fragment from a page URL. On parse
// failure the already-sanitized input is returned unchanged.
func stripURLQuery(rawurl string) string {
	u, err := url.Parse(rawurl)
	if err != nil {
		return rawurl
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// sanitizeLengthLimited redacts sensitive data and then truncates s to maxLen
// runes. Sanitizing first keeps truncated secrets out of the logs.
func sanitizeLengthLimited(s string, maxLen int) string {
	return truncateRunes(sanitize.String(s), maxLen)
}

// truncateRunes cuts s to at most maxLen runes without splitting multi-byte
// UTF-8 sequences.
func truncateRunes(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	return string([]rune(s)[:maxLen])
}
