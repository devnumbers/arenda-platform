package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	adminapp "github.com/nambers/arenda-planform/apps/backend/internal/admin/application"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	popupsapp "github.com/nambers/arenda-planform/apps/backend/internal/popups/application"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Deps holds the dependencies required by the HTTP server.
type Deps struct {
	Auth                     identityapp.Authenticator
	PhoneChange              identityapp.PhoneChanger
	Profile                  identityapp.Profiler
	Logout                   identityapp.Logout
	Sessions                 identityapp.SessionService
	Audit                    auditapp.Recorder
	MeEnricher               MeEnricher
	Tariffs                  billingapp.Tariffer
	Subscriptions            billingapp.Subscriber
	PaymentMethods           billingapp.PaymentMethodManager
	Payments                 billingapp.PaymentProcessor
	Webhooks                 billingapp.WebhookHandler
	Admin                    *adminapp.AdminService
	Properties               *propertiesapp.PropertyService
	PropertyContacts         *propertiesapp.PropertyContactService
	AddressSuggester         propertiesapp.AddressSuggester
	Leases                   *leasesapp.LeaseService
	TenantContacts           *leasesapp.TenantContactService
	Operations               *leasesapp.OperationService
	Export                   *leasesapp.ExportService
	RecurringOperations      *leasesapp.RecurringOperationService
	Categories               *leasesapp.CategoryService
	Reminders                *notificationsapp.ReminderService
	FreeReminders            *notificationsapp.FreeReminderService
	NotificationPreferences  *notificationsapp.PreferenceService
	Popups                   *popupsapp.PopupService
	AppBaseURL               string
	CookieSecure             bool
	Logger                   *slog.Logger
	Clock                    clock.Clock
	TZResolver               sharedtz.OwnerTimezoneResolver
	LogSuccessfulRequests    bool
	IPRateLimiter            *RateLimiter
	EmailSendLimiter         *RateLimiter
	EmailVerifyLimiter       *RateLimiter
	PhoneChangeSendLimiter   *RateLimiter
	PhoneChangeVerifyLimiter *RateLimiter
	ClientErrorsLimiter      *RateLimiter
	DBPoolStats              func() DBPoolSnapshot
	DevMode                  bool
	TrustedProxies           []string
	AppVersion               string
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
	r.Use(otelhttp.NewMiddleware("arenda-api", otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return r.Method + " " + r.URL.Path })))
	r.Use(RequestIDMiddleware)
	r.Use(realIPMiddleware(deps.TrustedProxies))
	r.Use(RequestLoggerWithOptions(deps.Logger, RequestLoggerOptions{
		LogSuccessfulRequests: deps.LogSuccessfulRequests,
		SlowRequestThreshold:  slowRequestThreshold,
	}))
	r.Use(recoveryMiddleware)
	r.Use(rateLimitMiddleware(deps.IPRateLimiter))
	r.Use(clientErrorsBodyLimitMiddleware)
	r.Use(securityHeaders(deps.CookieSecure))
	r.Use(SessionMiddleware(deps.Logger, deps.Sessions, deps.CookieSecure, deps.Clock))
	r.Use(readonlyMiddleware(deps.Subscriptions, deps.Logger, deps.Clock))

	r.Get("/healthz", healthHandler(deps.AppVersion))

	if deps.DBPoolStats != nil {
		r.Get("/internal/perf/db-pool", dbPoolDiagnosticsHandler(deps.DBPoolStats))
	}

	authHandlers := NewAuthHandlers(
		deps.Auth,
		deps.PhoneChange,
		deps.Profile,
		deps.Logout,
		deps.CookieSecure,
		deps.Logger,
		deps.EmailSendLimiter,
		deps.EmailVerifyLimiter,
		deps.PhoneChangeSendLimiter,
		deps.PhoneChangeVerifyLimiter,
		deps.MeEnricher,
		deps.Audit,
	)
	propertyHandlers := NewPropertyHandlers(deps.Properties, deps.AddressSuggester, deps.TenantContacts, deps.Operations, deps.Leases, deps.Export, deps.PropertyContacts, deps.Logger, deps.Clock, deps.TZResolver)
	leaseHandlers := NewLeaseHandlers(deps.Leases, deps.TenantContacts, deps.Logger, deps.Clock, deps.TZResolver)
	operationHandlers := NewOperationHandlers(deps.Operations, deps.Categories, deps.Properties, deps.Logger)
	recurringOperationHandlers := NewRecurringOperationHandlers(deps.RecurringOperations, deps.Categories, deps.Logger)
	categoryHandlers := NewCategoryHandlers(deps.Categories, deps.Logger)
	reminderHandlers := NewReminderHandlers(deps.Reminders, deps.Operations, deps.RecurringOperations, deps.Leases, deps.Logger)
	freeReminderHandlers := NewFreeReminderHandlers(deps.FreeReminders, deps.Properties, deps.Logger)
	notificationPreferenceHandlers := NewNotificationPreferenceHandlers(deps.NotificationPreferences, deps.Logger)
	popupHandlers := NewPopupHandlers(deps.Popups, deps.Logger)
	subscriptionHandlers := NewSubscriptionHandlers(deps.Tariffs, deps.Subscriptions, deps.PaymentMethods, deps.Payments, deps.Webhooks, deps.Logger, deps.DevMode)
	financeHandlers := NewFinanceHandlers(deps.Operations)
	adminHandlers := NewAdminHandlers(deps.Admin, deps.Logger)
	clientErrorsHandlers := NewClientErrorsHandlers(deps.ClientErrorsLimiter)

	handler := &composedHandler{
		AuthHandlers:                   authHandlers,
		PropertyHandlers:               propertyHandlers,
		LeaseHandlers:                  leaseHandlers,
		OperationHandlers:              operationHandlers,
		RecurringOperationHandlers:     recurringOperationHandlers,
		ReminderHandlers:               reminderHandlers,
		FreeReminderHandlers:           freeReminderHandlers,
		NotificationPreferenceHandlers: notificationPreferenceHandlers,
		PopupHandlers:                  popupHandlers,
		SubscriptionHandlers:           subscriptionHandlers,
		FinanceHandlers:                financeHandlers,
		AdminHandlers:                  adminHandlers,
		CategoryHandlers:               categoryHandlers,
		ClientErrorsHandlers:           clientErrorsHandlers,
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
	r.With(AdminOnlyMiddleware).Get("/admin/subscription/payments/{paymentId}", wrapper.GetAdminSubscriptionPayment)
	r.With(AdminOnlyMiddleware).Post("/admin/subscription/payments/{paymentId}/refund", wrapper.RefundSubscriptionPayment)
	r.With(AdminOnlyMiddleware).Post("/admin/subscription/payments/{paymentId}/sync", wrapper.SyncSubscriptionPayment)
	r.With(AdminOnlyMiddleware).Get("/admin/users", wrapper.ListAdminUsers)
	r.With(AdminOnlyMiddleware).Get("/admin/users/{id}", wrapper.GetAdminUser)
	r.With(AdminOnlyMiddleware).Get("/admin/users/{id}/properties", wrapper.ListAdminUserProperties)
	r.With(AdminOnlyMiddleware).Get("/admin/users/{id}/leases", wrapper.ListAdminUserLeases)
	r.With(AdminOnlyMiddleware).Get("/admin/users/{id}/tenant-contacts", wrapper.ListAdminUserTenantContacts)
	r.With(AdminOnlyMiddleware).Get("/admin/users/{id}/operations", wrapper.ListAdminUserOperations)
	r.With(AdminOnlyMiddleware).Get("/admin/properties", wrapper.ListAdminProperties)
	r.With(AdminOnlyMiddleware).Get("/admin/properties/{id}", wrapper.GetAdminProperty)
	r.With(AdminOnlyMiddleware).Get("/admin/leases", wrapper.ListAdminLeases)
	r.With(AdminOnlyMiddleware).Get("/admin/leases/{id}", wrapper.GetAdminLease)
	r.With(AdminOnlyMiddleware).Get("/admin/tenant-contacts", wrapper.ListAdminTenantContacts)
	r.With(AdminOnlyMiddleware).Get("/admin/tenant-contacts/{id}", wrapper.GetAdminTenantContact)
	r.With(AdminOnlyMiddleware).Get("/admin/property-contacts", wrapper.ListAdminPropertyContacts)
	r.With(AdminOnlyMiddleware).Get("/admin/operations", wrapper.ListAdminOperations)
	r.With(AdminOnlyMiddleware).Get("/admin/operations/{id}", wrapper.GetAdminOperation)
	r.With(AdminOnlyMiddleware).Get("/admin/stats", wrapper.GetAdminStats)
	r.With(AdminOnlyMiddleware).Get("/admin/audit-logs", wrapper.ListAdminAuditLogs)
	r.With(AdminOnlyMiddleware).Get("/admin/audit-logs/{id}", wrapper.GetAdminAuditLog)
	r.With(AdminOnlyMiddleware).Get("/admin/users/{id}/audit-logs", wrapper.ListAdminUserAuditLogs)

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
				w.Header().Set("Retry-After", "60")
				writeProblem(w, http.StatusTooManyRequests, problem(r.Context(), "Too Many Requests", "Превышен лимит запросов"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientErrorsBodyLimitMiddleware caps request bodies on the public client
// error endpoint before routing, so oversized reports are rejected cheaply.
func clientErrorsBodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/client-errors" {
			r.Body = http.MaxBytesReader(w, r.Body, clientErrorBodyLimit)
		}
		next.ServeHTTP(w, r)
	})
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
	*FreeReminderHandlers
	*NotificationPreferenceHandlers
	*PopupHandlers
	*SubscriptionHandlers
	*FinanceHandlers
	*AdminHandlers
	*CategoryHandlers
	*ClientErrorsHandlers
}
