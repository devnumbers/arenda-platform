package httpserver

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	accesshttp "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/http"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	adminhttp "github.com/nambers/arenda-planform/apps/backend/internal/admin/adapters/http"
	adminapp "github.com/nambers/arenda-planform/apps/backend/internal/admin/application"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	billinghttp "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/http"
	identityhttp "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/http"
	leaseshttp "github.com/nambers/arenda-planform/apps/backend/internal/leases/adapters/http"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	notificationshttp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/http"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	popupshttp "github.com/nambers/arenda-planform/apps/backend/internal/popups/adapters/http"
	popupsapp "github.com/nambers/arenda-planform/apps/backend/internal/popups/application"
	propertieshttp "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/http"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Deps holds the dependencies required by the HTTP server.
type Deps struct {
	Auth                     identityhttp.Authenticator
	PhoneChange              identityhttp.PhoneChanger
	Profile                  identityhttp.Profiler
	Logout                   identityhttp.Logout
	Sessions                 httpsupport.SessionLoader
	Audit                    auditapp.Recorder
	MeEnricher               identityhttp.MeEnricher
	Tariffs                  billinghttp.TariffLister
	Subscriptions            billinghttp.SubscriptionViewer
	ReadonlyGate             httpsupport.SubscriptionMutationChecker
	Admin                    *adminapp.AdminService
	Properties               *propertiesapp.PropertyService
	PropertyContacts         *propertiesapp.PropertyContactService
	AddressSuggester         propertiesapp.AddressSuggester
	Access                   *accessapp.AccessService
	Invitations              *accessapp.InvitationService
	Leases                   *leasesapp.LeaseService
	TenantContacts           *leasesapp.TenantContactService
	Operations               *leasesapp.OperationService
	Export                   *leasesapp.ExportService
	RecurringOperations      *leasesapp.RecurringOperationService
	Categories               *leasesapp.CategoryService
	Reminders                *notificationsapp.ReminderService
	Calendar                 *notificationsapp.CalendarService
	FreeReminders            *notificationsapp.FreeReminderService
	NotificationPreferences  *notificationsapp.PreferenceService
	PushSubscriptions        *notificationsapp.PushSubscriptionService
	VAPIDPublicKey           string
	Popups                   *popupsapp.PopupService
	AppBaseURL               string
	CookieSecure             bool
	Logger                   *slog.Logger
	Clock                    clock.Clock
	TZResolver               sharedtz.OwnerTimezoneResolver
	LogSuccessfulRequests    bool
	IPRateLimiter            *httpsupport.RateLimiter
	EmailSendLimiter         *httpsupport.RateLimiter
	EmailVerifyLimiter       *httpsupport.RateLimiter
	PhoneChangeSendLimiter   *httpsupport.RateLimiter
	PhoneChangeVerifyLimiter *httpsupport.RateLimiter
	ClientErrorsLimiter      *httpsupport.RateLimiter
	DBPoolStats              func() httpsupport.DBPoolSnapshot
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
	r.Use(httpsupport.RequestIDMiddleware)
	r.Use(httpsupport.RealIPMiddleware(deps.TrustedProxies))
	r.Use(httpsupport.RequestLoggerWithOptions(deps.Logger, httpsupport.RequestLoggerOptions{
		LogSuccessfulRequests: deps.LogSuccessfulRequests,
		SlowRequestThreshold:  slowRequestThreshold,
	}))
	r.Use(httpsupport.RecoveryMiddleware)
	r.Use(rateLimitMiddleware(deps.IPRateLimiter))
	r.Use(clientErrorsBodyLimitMiddleware)
	r.Use(securityHeaders(deps.CookieSecure))
	r.Use(httpsupport.SessionMiddleware(deps.Logger, deps.Sessions, deps.CookieSecure, deps.Clock))
	r.Use(httpsupport.ReadonlyMiddleware(deps.ReadonlyGate, deps.Logger))

	r.Get("/healthz", httpsupport.HealthHandler(deps.AppVersion))

	if deps.DBPoolStats != nil {
		r.Get("/internal/perf/db-pool", httpsupport.DBPoolDiagnosticsHandler(deps.DBPoolStats))
	}

	authHandlers := identityhttp.NewAuthHandlers(
		deps.Auth,
		deps.PhoneChange,
		deps.Profile,
		deps.Logout,
		deps.CookieSecure,
		deps.Logger,
		identityhttp.AuthRateLimits{
			Send:              deps.EmailSendLimiter,
			Verify:            deps.EmailVerifyLimiter,
			PhoneChangeSend:   deps.PhoneChangeSendLimiter,
			PhoneChangeVerify: deps.PhoneChangeVerifyLimiter,
		},
		deps.MeEnricher,
	)
	propertyHandlers := propertieshttp.NewPropertyHandlers(deps.Properties, deps.AddressSuggester, deps.TenantContacts, deps.Operations, deps.Leases, deps.Export, deps.PropertyContacts, deps.Logger, deps.Clock, deps.TZResolver)
	accessMemberHandlers := accesshttp.NewMemberHandlers(deps.Access, deps.Logger)
	accessInvitationHandlers := accesshttp.NewInvitationHandlers(deps.Invitations, deps.Logger)
	leaseHandlers := leaseshttp.NewLeaseHandlers(deps.Leases, deps.TenantContacts, deps.Logger, deps.Clock, deps.TZResolver)
	operationHandlers := leaseshttp.NewOperationHandlers(deps.Operations, deps.Categories, deps.Properties, deps.Logger)
	recurringOperationHandlers := leaseshttp.NewRecurringOperationHandlers(deps.RecurringOperations, deps.Categories, deps.Logger)
	categoryHandlers := leaseshttp.NewCategoryHandlers(deps.Categories, deps.Logger)
	reminderHandlers := notificationshttp.NewReminderHandlers(deps.Reminders, deps.Calendar, deps.Operations, deps.RecurringOperations, deps.Leases, deps.Logger)
	freeReminderHandlers := notificationshttp.NewFreeReminderHandlers(deps.FreeReminders, deps.Reminders, deps.Properties, deps.Clock, deps.Logger)
	notificationPreferenceHandlers := notificationshttp.NewNotificationPreferenceHandlers(deps.NotificationPreferences, deps.Logger)
	pushSubscriptionHandlers := notificationshttp.NewPushSubscriptionHandlers(deps.PushSubscriptions, deps.VAPIDPublicKey, deps.Logger)
	popupHandlers := popupshttp.NewPopupHandlers(deps.Popups, deps.Logger)
	billingHandlers := billinghttp.NewBillingHandlers(deps.Tariffs, deps.Subscriptions, deps.Logger)
	financeHandlers := leaseshttp.NewFinanceHandlers(deps.Operations)
	adminHandlers := adminhttp.NewAdminHandlers(deps.Admin, deps.Logger)
	clientErrorsHandlers := httpsupport.NewClientErrorsHandlers(deps.ClientErrorsLimiter)

	handler := &composedHandler{
		AuthHandlers:                   authHandlers,
		PropertyHandlers:               propertyHandlers,
		MemberHandlers:                 accessMemberHandlers,
		InvitationHandlers:             accessInvitationHandlers,
		LeaseHandlers:                  leaseHandlers,
		OperationHandlers:              operationHandlers,
		RecurringOperationHandlers:     recurringOperationHandlers,
		ReminderHandlers:               reminderHandlers,
		FreeReminderHandlers:           freeReminderHandlers,
		NotificationPreferenceHandlers: notificationPreferenceHandlers,
		PushSubscriptionHandlers:       pushSubscriptionHandlers,
		PopupHandlers:                  popupHandlers,
		BillingHandlers:                billingHandlers,
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
		ErrorHandlerFunc: httpsupport.OpenAPIErrorHandler,
	})

	wrapper := openapi.ServerInterfaceWrapper{
		Handler:          handler,
		ErrorHandlerFunc: httpsupport.OpenAPIErrorHandler,
	}
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/tariffs", wrapper.ListAdminTariffs)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/subscription/payments", wrapper.ListAdminSubscriptionPayments)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/subscription/payments/{paymentId}", wrapper.GetAdminSubscriptionPayment)
	r.With(httpsupport.AdminOnlyMiddleware).Post("/admin/subscription/payments/{paymentId}/refund", wrapper.RefundSubscriptionPayment)
	r.With(httpsupport.AdminOnlyMiddleware).Post("/admin/subscription/payments/{paymentId}/sync", wrapper.SyncSubscriptionPayment)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users", wrapper.ListAdminUsers)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users/{id}", wrapper.GetAdminUser)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users/{id}/properties", wrapper.ListAdminUserProperties)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users/{id}/leases", wrapper.ListAdminUserLeases)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users/{id}/tenant-contacts", wrapper.ListAdminUserTenantContacts)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users/{id}/operations", wrapper.ListAdminUserOperations)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/properties", wrapper.ListAdminProperties)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/properties/{id}", wrapper.GetAdminProperty)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/leases", wrapper.ListAdminLeases)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/leases/{id}", wrapper.GetAdminLease)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/tenant-contacts", wrapper.ListAdminTenantContacts)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/tenant-contacts/{id}", wrapper.GetAdminTenantContact)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/property-contacts", wrapper.ListAdminPropertyContacts)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/operations", wrapper.ListAdminOperations)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/operations/{id}", wrapper.GetAdminOperation)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/stats", wrapper.GetAdminStats)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/audit-logs", wrapper.ListAdminAuditLogs)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/audit-logs/{id}", wrapper.GetAdminAuditLog)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users/{id}/audit-logs", wrapper.ListAdminUserAuditLogs)

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

func rateLimitMiddleware(limiter *httpsupport.RateLimiter) func(http.Handler) http.Handler {
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
				httpsupport.WriteProblem(w, http.StatusTooManyRequests, httpsupport.Problem(r.Context(), "Too Many Requests", "Превышен лимит запросов"))
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
			r.Body = http.MaxBytesReader(w, r.Body, httpsupport.ClientErrorBodyLimit)
		}
		next.ServeHTTP(w, r)
	})
}

// composedHandler groups the existing handler sets. Embedding provides the
// generated ServerInterface implementation without forwarding methods.
type composedHandler struct {
	*identityhttp.AuthHandlers
	*propertieshttp.PropertyHandlers
	*accesshttp.MemberHandlers
	*accesshttp.InvitationHandlers
	*leaseshttp.LeaseHandlers
	*leaseshttp.OperationHandlers
	*leaseshttp.RecurringOperationHandlers
	*notificationshttp.ReminderHandlers
	*notificationshttp.FreeReminderHandlers
	*notificationshttp.NotificationPreferenceHandlers
	*notificationshttp.PushSubscriptionHandlers
	*popupshttp.PopupHandlers
	*billinghttp.BillingHandlers
	*leaseshttp.FinanceHandlers
	*adminhttp.AdminHandlers
	*leaseshttp.CategoryHandlers
	*httpsupport.ClientErrorsHandlers
}
