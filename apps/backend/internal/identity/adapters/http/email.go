package http

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// parseOptionalEmail parses an optional request email. It returns ok=false
// after writing a 400 problem response when the provided email is invalid.
// A nil or blank email is treated as absent and yields (nil, true).
//
// It is the identity-local replacement for the former
// httpsupport.ParseOptionalEmail, moved here so that platform/httpsupport no
// longer parses identity/domain.Email (ADR 0034). Only the auth handlers need
// this helper.
func parseOptionalEmail(w http.ResponseWriter, r *http.Request, raw *string) (*domain.Email, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, true
	}
	parsed, err := domain.NewEmail(*raw)
	if err != nil {
		httpsupport.LoggerFromContext(r.Context()).WarnContext(r.Context(), "invalid email in request body", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Invalid email", "Некорректная почта"))
		return nil, false
	}
	return &parsed, true
}
