// Package httpserver is the HTTP composition root: it builds the chi handler that wires every context's
// endpoints, middleware and routes.
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
	notificationshttp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/http"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	paymentshttp "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/http"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	popupshttp "github.com/nambers/arenda-planform/apps/backend/internal/popups/adapters/http"
	popupsapp "github.com/nambers/arenda-planform/apps/backend/internal/popups/application"
	propertieshttp "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/http"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Deps holds the dependencies required by the HTTP server.
type Deps struct {
	Auth                 identityhttp.Authenticator
	PhoneChange          identityhttp.PhoneChanger
	Profile              identityhttp.Profiler
	Logout               identityhttp.Logout
	Sessions             httpsupport.SessionLoader
	Audit                auditapp.Recorder
	MeEnricher           identityhttp.MeEnricher
	Tariffs              billinghttp.TariffLister
	AdminTariffs         billinghttp.AdminTariffManager
	Subscriptions        billinghttp.SubscriptionViewer
	SubscriptionManagers billinghttp.SubscriptionManager
	Payments             billinghttp.PaymentManager
	PaymentMethods       billinghttp.PaymentMethodManager
	Webhooks             billinghttp.WebhookProcessor
	AdminPayments        billinghttp.AdminPaymentManager
	AdminSubscriptions   billinghttp.AdminSubscriptionManager
	// BillingFakeConfirms serves the local-only fake confirmation endpoints;
	// non-nil only when the fake provider is active (the billing wiring
	// constructs it there), so the routes are mounted only in that case —
	// a production build has no such endpoints at all (issue #287).
	BillingFakeConfirms      *billinghttp.FakeConfirmHandlers
	ReadonlyGate             httpsupport.SubscriptionMutationChecker
	Admin                    *adminapp.AdminService
	Properties               *propertiesapp.PropertyService
	PropertyContacts         *propertiesapp.PropertyContactService
	AddressSuggester         propertiesapp.AddressSuggester
	PropertyPayments         *paymentsapp.PaymentService
	Access                   *accessapp.AccessService
	Invitations              *accessapp.InvitationService
	NotificationPreferences  *notificationsapp.PreferenceService
	PushSubscriptions        *notificationsapp.PushSubscriptionService
	VAPIDPublicKey           string
	Popups                   *popupsapp.PopupService
	AppBaseURL               string
	CookieSecure             bool
	Logger                   *slog.Logger
	Clock                    clock.Clock
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
	r.Use(otelhttp.NewMiddleware("arenda-api",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return r.Method + " " + r.URL.Path })))
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
	propertyHandlers := propertieshttp.NewPropertyHandlers(
		deps.Properties,
		deps.AddressSuggester,
		deps.PropertyContacts,
		deps.Logger,
		deps.Clock,
	)
	accessMemberHandlers := accesshttp.NewMemberHandlers(deps.Access, deps.Logger)
	accessInvitationHandlers := accesshttp.NewInvitationHandlers(deps.Invitations, deps.Logger)
	notificationPreferenceHandlers := notificationshttp.NewNotificationPreferenceHandlers(deps.NotificationPreferences, deps.Logger)
	pushSubscriptionHandlers := notificationshttp.NewPushSubscriptionHandlers(deps.PushSubscriptions, deps.VAPIDPublicKey, deps.Logger)
	popupHandlers := popupshttp.NewPopupHandlers(deps.Popups, deps.Logger)
	billingHandlers := billinghttp.NewBillingHandlers(
		deps.Tariffs, deps.AdminTariffs, deps.Subscriptions, deps.SubscriptionManagers,
		deps.Payments, deps.PaymentMethods, deps.Webhooks, deps.AdminPayments,
		deps.AdminSubscriptions, deps.Logger)
	paymentHandlers := paymentshttp.NewPaymentHandlers(deps.PropertyPayments, deps.Logger)
	adminHandlers := adminhttp.NewAdminHandlers(deps.Admin, deps.Logger)
	clientErrorsHandlers := httpsupport.NewClientErrorsHandlers(deps.ClientErrorsLimiter)

	handler := &composedHandler{
		AuthHandlers:                   authHandlers,
		PropertyHandlers:               propertyHandlers,
		MemberHandlers:                 accessMemberHandlers,
		InvitationHandlers:             accessInvitationHandlers,
		NotificationPreferenceHandlers: notificationPreferenceHandlers,
		PushSubscriptionHandlers:       pushSubscriptionHandlers,
		PopupHandlers:                  popupHandlers,
		BillingHandlers:                billingHandlers,
		PaymentHandlers:                paymentHandlers,
		AdminHandlers:                  adminHandlers,
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
	r.With(httpsupport.AdminOnlyMiddleware).Post("/admin/tariffs", wrapper.CreateAdminTariff)
	r.With(httpsupport.AdminOnlyMiddleware).Put("/admin/tariffs/{tariffId}", wrapper.UpdateAdminTariff)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/subscription/payments", wrapper.ListAdminSubscriptionPayments)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/subscription/payments/{paymentId}", wrapper.GetAdminSubscriptionPayment)
	r.With(httpsupport.AdminOnlyMiddleware).Post("/admin/subscription/payments/{paymentId}/refund", wrapper.RefundSubscriptionPayment)
	r.With(httpsupport.AdminOnlyMiddleware).Post("/admin/subscription/payments/{paymentId}/sync", wrapper.SyncSubscriptionPayment)
	r.With(httpsupport.AdminOnlyMiddleware).Post("/admin/users/{id}/subscription/service", wrapper.AssignAdminServiceSubscription)
	r.With(httpsupport.AdminOnlyMiddleware).Post("/admin/users/{id}/subscription/force-change", wrapper.ForceChangeAdminSubscriptionTariff)
	r.With(httpsupport.AdminOnlyMiddleware).Post("/admin/users/{id}/subscription/grace-extension", wrapper.ExtendAdminSubscriptionGrace)
	r.With(httpsupport.AdminOnlyMiddleware).Post("/admin/users/{id}/subscription/cancel", wrapper.CancelAdminSubscription)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users/{id}/subscription/transitions", wrapper.ListAdminSubscriptionTransitions)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users", wrapper.ListAdminUsers)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users/{id}", wrapper.GetAdminUser)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users/{id}/properties", wrapper.ListAdminUserProperties)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/properties", wrapper.ListAdminProperties)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/properties/{id}", wrapper.GetAdminProperty)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/property-contacts", wrapper.ListAdminPropertyContacts)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/stats", wrapper.GetAdminStats)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/audit-logs", wrapper.ListAdminAuditLogs)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/audit-logs/{id}", wrapper.GetAdminAuditLog)
	r.With(httpsupport.AdminOnlyMiddleware).Get("/admin/users/{id}/audit-logs", wrapper.ListAdminUserAuditLogs)

	// T-Kassa redirects the user here after the add-card bank form. Redirect them
	// back to the frontend payment-methods page with a query flag so the UI can
	// refresh the list and show the appropriate toast.
	r.Get("/subscription/payment-methods/add-card/success", addCardReturnHandler(deps.AppBaseURL, true))
	r.Get("/subscription/payment-methods/add-card/fail", addCardReturnHandler(deps.AppBaseURL, false))

	// The local-only fake confirmation endpoints (issues #250/#251) live
	// outside the generated contract: the billing wiring constructs their
	// handlers only under the fake provider, so a production build mounts no
	// such routes at all (issue #287).
	if deps.BillingFakeConfirms != nil {
		r.Post(billinghttp.FakePaymentConfirmRoute, deps.BillingFakeConfirms.ConfirmPayment)
		r.Post(billinghttp.FakeCardBindingConfirmRoute, deps.BillingFakeConfirms.ConfirmCardBinding)
	}

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
				httpsupport.WriteProblem(r.Context(), w, http.StatusTooManyRequests,
					httpsupport.Problem(r.Context(), "Too Many Requests", "Превышен лимит запросов"))
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
	*notificationshttp.NotificationPreferenceHandlers
	*notificationshttp.PushSubscriptionHandlers
	*popupshttp.PopupHandlers
	*billinghttp.BillingHandlers
	*paymentshttp.PaymentHandlers
	*adminhttp.AdminHandlers
	*httpsupport.ClientErrorsHandlers
}
