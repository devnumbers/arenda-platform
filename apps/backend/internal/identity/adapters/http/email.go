package http

import (
	"log/slog"
	"net/http"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// parseEmail parses a required request email. It returns ok=false after
// writing a 400 problem response when the email is missing or invalid.
//
// It is the identity-local replacement for the former
// httpsupport.ParseOptionalEmail, moved here so that platform/httpsupport no
// longer parses identity/domain.Email (ADR 0034). Only the auth handlers need
// this helper.
func parseEmail(w http.ResponseWriter, r *http.Request, raw string) (domain.Email, bool) {
	if raw == "" {
		httpsupport.LoggerFromContext(r.Context()).WarnContext(r.Context(), "missing email in request body")
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Invalid email", "Некорректная почта"))
		return domain.Email{}, false
	}
	parsed, err := domain.NewEmail(raw)
	if err != nil {
		httpsupport.LoggerFromContext(r.Context()).WarnContext(r.Context(), "invalid email in request body",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Invalid email", "Некорректная почта"))
		return domain.Email{}, false
	}
	return parsed, true
}
