package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// Deps holds the dependencies required by the HTTP server.
type Deps struct {
	Auth                *identityapp.AuthService
	Billing             *billingapp.BillingService
	Sessions            identityapp.SessionRepository
	Properties          *propertiesapp.PropertyService
	Leases              *leasesapp.LeaseService
	TenantContacts      *leasesapp.TenantContactService
	Operations          *leasesapp.OperationService
	RecurringOperations *leasesapp.RecurringOperationService
	Reminders           *notificationsapp.ReminderService
	CookieSecure        bool
	Logger              *slog.Logger
	Clock               clock.Clock
	LogSuccessfulRequests bool
	IPRateLimiter       *RateLimiter
	PhoneSendLimiter    *RateLimiter
	PhoneVerifyLimiter  *RateLimiter
	DBPoolStats         func() DBPoolSnapshot
	DevMode             bool
}

const slowRequestThreshold = 500 * time.Millisecond

func securityHeaders(secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'none'")
			if secure {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// New builds the HTTP handler with routing and middleware wired.
func New(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestIDMiddleware)
	//nolint:staticcheck // middleware.RealIP is used here as the project-wide IP extraction strategy.
	// TODO: migrate from chi's deprecated middleware.RealIP to an explicit,
	// proxy-aware IP extraction strategy (e.g., via TRUSTED_PROXIES env).
	// Until then, client traffic must be terminated at a trusted proxy that
	// sanitises X-Forwarded-For before it reaches this middleware.
	r.Use(middleware.RealIP)
	r.Use(RequestLoggerWithOptions(deps.Logger, RequestLoggerOptions{
		LogSuccessfulRequests: deps.LogSuccessfulRequests,
		SlowRequestThreshold:  slowRequestThreshold,
	}))
	r.Use(middleware.Recoverer)
	r.Use(rateLimitMiddleware(deps.IPRateLimiter))
	r.Use(securityHeaders(deps.CookieSecure))
	r.Use(SessionMiddleware(deps.Logger, deps.Sessions, deps.CookieSecure, deps.Clock))
	r.Use(readonlyMiddleware(deps.Billing, deps.Logger, deps.Clock))

	if deps.DBPoolStats != nil {
		r.Get("/internal/perf/db-pool", dbPoolDiagnosticsHandler(deps.DBPoolStats))
	}

	authHandlers := NewAuthHandlers(deps.Auth, deps.Billing, deps.CookieSecure, deps.Logger, deps.PhoneSendLimiter, deps.PhoneVerifyLimiter)
	propertyHandlers := NewPropertyHandlers(deps.Properties, deps.Logger)
	leaseHandlers := NewLeaseHandlers(deps.Leases, deps.TenantContacts, deps.Logger)
	operationHandlers := NewOperationHandlers(deps.Operations, deps.Logger)
	recurringOperationHandlers := NewRecurringOperationHandlers(deps.RecurringOperations, deps.Logger)
	reminderHandlers := NewReminderHandlers(deps.Reminders, deps.Operations, deps.RecurringOperations, deps.Leases, deps.Logger)
	subscriptionHandlers := NewSubscriptionHandlers(deps.Billing, deps.Logger, deps.DevMode)

	handler := &composedHandler{
		AuthHandlers:               authHandlers,
		PropertyHandlers:           propertyHandlers,
		LeaseHandlers:              leaseHandlers,
		OperationHandlers:          operationHandlers,
		RecurringOperationHandlers: recurringOperationHandlers,
		ReminderHandlers:           reminderHandlers,
		SubscriptionHandlers:       subscriptionHandlers,
	}

	return openapi.HandlerWithOptions(handler, openapi.ChiServerOptions{
		BaseRouter:       r,
		ErrorHandlerFunc: openAPIErrorHandler,
	})
}

func rateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			if !limiter.Allow(ip) {
				writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too Many Requests", "rate limit exceeded"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// composedHandler groups the existing handler sets. Embedding provides the
// generated ServerInterface implementation without forwarding methods.
type composedHandler struct {
	*AuthHandlers
	*PropertyHandlers
	*LeaseHandlers
	*OperationHandlers
	*RecurringOperationHandlers
	*ReminderHandlers
	*SubscriptionHandlers
}
