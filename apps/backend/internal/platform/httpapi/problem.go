package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
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
func internalError(ctx context.Context, err error) openapi.Problem {
	loggerFromContext(ctx).ErrorContext(ctx, "internal server error",
		slog.String("error", err.Error()),
		slog.String("request_id", RequestIDFromContext(ctx)),
	)
	return problem(ctx, "Internal Server Error", "An internal error occurred")
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
