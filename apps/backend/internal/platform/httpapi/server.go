package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
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
	Auth                  *identityapp.AuthService
	Billing               *billingapp.BillingService
	Sessions              identityapp.SessionRepository
	Properties            *propertiesapp.PropertyService
	AddressSuggester      propertiesapp.AddressSuggester
	Leases                *leasesapp.LeaseService
	TenantContacts        *leasesapp.TenantContactService
	Operations            *leasesapp.OperationService
	RecurringOperations   *leasesapp.RecurringOperationService
	Reminders             *notificationsapp.ReminderService
	AppBaseURL            string
	CookieSecure          bool
	Logger                *slog.Logger
	Clock                 clock.Clock
	LogSuccessfulRequests bool
	IPRateLimiter         *RateLimiter
	PhoneSendLimiter      *RateLimiter
	PhoneVerifyLimiter    *RateLimiter
	EmailSendLimiter      *RateLimiter
	EmailVerifyLimiter    *RateLimiter
	DBPoolStats           func() DBPoolSnapshot
	DevMode               bool
	TrustedProxies        []string
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
	r.Use(realIPMiddleware(deps.TrustedProxies))
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

	authHandlers := NewAuthHandlers(deps.Auth, deps.Billing, deps.CookieSecure, deps.Logger, deps.PhoneSendLimiter, deps.PhoneVerifyLimiter, deps.EmailSendLimiter, deps.EmailVerifyLimiter)
	propertyHandlers := NewPropertyHandlers(deps.Properties, deps.AddressSuggester, deps.TenantContacts, deps.Operations, deps.Logger)
	leaseHandlers := NewLeaseHandlers(deps.Leases, deps.TenantContacts, deps.Logger)
	operationHandlers := NewOperationHandlers(deps.Operations, deps.Logger)
	recurringOperationHandlers := NewRecurringOperationHandlers(deps.RecurringOperations, deps.Logger)
	reminderHandlers := NewReminderHandlers(deps.Reminders, deps.Operations, deps.RecurringOperations, deps.Leases, deps.Logger)
	subscriptionHandlers := NewSubscriptionHandlers(deps.Billing, deps.Logger, deps.DevMode)
	financeHandlers := NewFinanceHandlers(deps.Operations)

	handler := &composedHandler{
		AuthHandlers:               authHandlers,
		PropertyHandlers:           propertyHandlers,
		LeaseHandlers:              leaseHandlers,
		OperationHandlers:          operationHandlers,
		RecurringOperationHandlers: recurringOperationHandlers,
		ReminderHandlers:           reminderHandlers,
		SubscriptionHandlers:       subscriptionHandlers,
		FinanceHandlers:            financeHandlers,
	}

	// The generated OpenAPI router has no per-route middleware support, so we
	// let it register all routes first, then override the admin refund route
	// with one that applies AdminOnlyMiddleware. Chi matches the last
	// registered route, and delegating to the generated wrapper keeps path
	// parameter binding and error handling consistent.
	generated := openapi.HandlerWithOptions(handler, openapi.ChiServerOptions{
		BaseRouter:       r,
		ErrorHandlerFunc: openAPIErrorHandler,
	})

	wrapper := openapi.ServerInterfaceWrapper{
		Handler:          handler,
		ErrorHandlerFunc: openAPIErrorHandler,
	}
	r.With(AdminOnlyMiddleware).Get("/admin/subscription/payments", wrapper.ListAdminSubscriptionPayments)
	r.With(AdminOnlyMiddleware).Post("/admin/subscription/payments/{paymentId}/refund", wrapper.RefundSubscriptionPayment)
	r.With(AdminOnlyMiddleware).Post("/admin/subscription/payments/{paymentId}/sync", wrapper.SyncSubscriptionPayment)

	// T-Kassa redirects the user here after the add-card bank form. Redirect them
	// back to the frontend payment-methods page with a query flag so the UI can
	// refresh the list and show the appropriate toast.
	r.Get("/subscription/payment-methods/add-card/success", addCardReturnHandler(deps.AppBaseURL, true))
	r.Get("/subscription/payment-methods/add-card/fail", addCardReturnHandler(deps.AppBaseURL, false))

	return generated
}

func addCardReturnHandler(baseURL string, success bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := baseURL
		if path, err := url.JoinPath(baseURL, "/profile/tariff/payment-methods"); err == nil {
			target = path
		}
		q := url.Values{}
		if success {
			q.Set("addCard", "success")
		} else {
			q.Set("addCard", "fail")
		}
		http.Redirect(w, r, target+"?"+q.Encode(), http.StatusFound)
	}
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
	*FinanceHandlers
}
