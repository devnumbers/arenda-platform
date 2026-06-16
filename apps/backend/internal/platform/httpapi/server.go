package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// Deps holds the dependencies required by the HTTP server.
type Deps struct {
	Auth         *application.AuthService
	Sessions     application.SessionRepository
	CookieSecure bool
	Logger       *slog.Logger
	Clock        application.Clock
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		next.ServeHTTP(w, r)
	})
}

// New builds the HTTP handler with routing and middleware wired.
func New(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestIDMiddleware)
	r.Use(RequestLogger(deps.Logger))
	r.Use(securityHeaders)
	r.Use(middleware.Recoverer)
	r.Use(SessionMiddleware(deps.Logger, deps.Sessions, deps.CookieSecure, deps.Clock))

	authHandlers := NewAuthHandlers(deps.Auth, deps.CookieSecure, deps.Logger)
	return openapi.HandlerFromMux(authHandlers, r)
}
