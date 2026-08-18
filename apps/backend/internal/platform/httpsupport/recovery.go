package httpsupport

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// maxPanicStackLength caps the stack trace attached to panic recovery logs.
const maxPanicStackLength = 8192

// RecoveryMiddleware recovers from panics in downstream handlers, logs the
// sanitized panic value with a truncated stack trace, and responds with an
// RFC 7807 problem. It deliberately avoids InternalError to keep a single log
// entry per panic; WriteProblem still records the error code for the request
// logger.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func(ctx context.Context) {
			if rec := recover(); rec != nil {
				// Sanitize before truncating: debug.Stack includes function
				// arguments that may contain secrets or PII.
				stack := sanitizeLengthLimited(string(debug.Stack()), maxPanicStackLength)
				LoggerFromContext(ctx).ErrorContext(ctx, "http panic recovered",
					slog.String("error", SanitizeError(fmt.Errorf("%v", rec))),
					slog.String("stack", stack),
				)
				WriteProblem(ctx, w, http.StatusInternalServerError, Problem(ctx, "Internal Server Error", "Произошла внутренняя ошибка. Попробуйте позже."))
			}
		}(r.Context())
		next.ServeHTTP(w, r)
	})
}
