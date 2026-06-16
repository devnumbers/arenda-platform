package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

// Deps holds the dependencies required by the HTTP server.
type Deps struct {
	Auth           *identityapp.AuthService
	Sessions       identityapp.SessionRepository
	Properties     *propertiesapp.PropertyService
	Leases         *leasesapp.LeaseService
	TenantContacts *leasesapp.TenantContactService
	CookieSecure   bool
	Logger         *slog.Logger
	Clock          identityapp.Clock
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
	propertyHandlers := NewPropertyHandlers(deps.Properties, deps.Logger)
	leaseHandlers := NewLeaseHandlers(deps.Leases, deps.TenantContacts, deps.Logger)

	handler := &composedHandler{
		AuthHandlers:     authHandlers,
		PropertyHandlers: propertyHandlers,
		LeaseHandlers:    leaseHandlers,
	}

	return openapi.HandlerFromMux(handler, r)
}

// composedHandler groups the existing handler sets. Embedding provides the
// generated ServerInterface implementation without forwarding methods.
type composedHandler struct {
	*AuthHandlers
	*PropertyHandlers
	*LeaseHandlers
}
