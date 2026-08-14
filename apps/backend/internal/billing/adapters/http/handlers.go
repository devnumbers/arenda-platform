package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// The two service interfaces below are consumed only by this package, so per
// the Go idiom "accept interfaces at the consumer" (ADR 0035) they live here
// next to BillingHandlers rather than in the application package.
// Conformance is checked where the handlers are wired (httpserver.Deps).

// TariffLister serves the user-facing and admin tariff views: ListTariffs the
// active plans users choose from, ListAllTariffs every plan including hidden
// ones for the admin screen (issue #247).
type TariffLister interface {
	ListTariffs(ctx context.Context) ([]domain.Tariff, error)
	ListAllTariffs(ctx context.Context) ([]domain.Tariff, error)
}

// SubscriptionViewer serves the user's own subscription view.
type SubscriptionViewer interface {
	GetSubscription(ctx context.Context, userID uuid.UUID) (billingapp.SubscriptionView, error)
}

// SubscriptionManager serves the user's own subscription lifecycle mutations
// (issue #249): cancellation, auto-renew toggling and tariff change.
type SubscriptionManager interface {
	CancelSubscription(ctx context.Context, userID uuid.UUID) error
	ToggleAutoRenew(ctx context.Context, userID uuid.UUID, enabled bool) error
	ChangeTariff(ctx context.Context, userID uuid.UUID, req billingapp.ChangeTariffRequest) (billingapp.ChangeTariffResult, error)
}

// BillingHandlers implements the generated billing endpoints of the
// OpenAPI contract. The subscription lifecycle mutations of issue #249 are
// live — PATCH /subscription/auto-renew, POST /subscription/cancel and POST
// /subscription/change (downgrade scheduling; upgrades answer with a temporary
// payment-unavailable error until #250). The remaining endpoints answer 501
// until their flows land — payments and webhooks in #250, payment methods in
// #251, refunds and admin payment views in #254. The user-facing contract is
// frozen, so the routes stay mounted.
type BillingHandlers struct {
	tariffs       TariffLister
	subscriptions SubscriptionViewer
	managers      SubscriptionManager
	logger        *slog.Logger
}

// NewBillingHandlers creates HTTP handlers for the billing API.
func NewBillingHandlers(
	tariffs TariffLister,
	subscriptions SubscriptionViewer,
	managers SubscriptionManager,
	logger *slog.Logger,
) *BillingHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &BillingHandlers{
		tariffs:       tariffs,
		subscriptions: subscriptions,
		managers:      managers,
		logger:        logger,
	}
}

// ListTariffs implements GET /tariffs.
func (h *BillingHandlers) ListTariffs(w http.ResponseWriter, r *http.Request) {
	if _, ok := httpsupport.OwnerIDFromContext(r); !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	tariffs, err := h.tariffs.ListTariffs(r.Context())
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	items := make([]openapi.Tariff, 0, len(tariffs))
	for _, t := range tariffs {
		items = append(items, httpsupport.TariffResponse(t))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.TariffsResponse{Items: items})
}

// ListAdminTariffs implements GET /admin/tariffs. Access control is the
// AdminOnlyMiddleware the route is mounted with, like every other admin
// endpoint.
func (h *BillingHandlers) ListAdminTariffs(w http.ResponseWriter, r *http.Request) {
	tariffs, err := h.tariffs.ListAllTariffs(r.Context())
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	items := make([]openapi.AdminTariff, 0, len(tariffs))
	for _, t := range tariffs {
		items = append(items, httpsupport.AdminTariffResponse(t))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.AdminTariffsResponse{Items: items})
}

// GetSubscription implements GET /subscription.
func (h *BillingHandlers) GetSubscription(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	view, err := h.subscriptions.GetSubscription(r.Context(), ownerID)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, httpsupport.SubscriptionResponse(view))
}

// notImplemented answers the endpoints whose flows return with later tickets
// of the billing rewrite (#249–#254).
func (h *BillingHandlers) notImplemented(w http.ResponseWriter, r *http.Request) {
	h.logger.WarnContext(r.Context(), "billing endpoint not implemented yet",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path))
	httpsupport.WriteProblem(w, http.StatusNotImplemented, httpsupport.Problem(r.Context(), "Not implemented", "Функционал временно недоступен"))
}

// ToggleAutoRenew implements PATCH /subscription/auto-renew (issue #249).
func (h *BillingHandlers) ToggleAutoRenew(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.AutoRenewRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode auto-renew request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	if err := h.managers.ToggleAutoRenew(r.Context(), ownerID, body.Enabled); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CancelSubscription implements POST /subscription/cancel (issue #249).
func (h *BillingHandlers) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.managers.CancelSubscription(r.Context(), ownerID); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ChangeTariff implements POST /subscription/change (issue #249): downgrade
// scheduling, same-tariff rejection and the temporary payment-unavailable
// answer for upgrades until the payment flow lands (#250).
func (h *BillingHandlers) ChangeTariff(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.ChangeTariffRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode change tariff request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	tariffName, err := domain.ParseTariffName(string(body.TariffName))
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}
	period, err := domain.ParseSubscriptionPeriod(string(body.Period))
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	result, err := h.managers.ChangeTariff(r.Context(), ownerID, billingapp.ChangeTariffRequest{
		TariffName: tariffName,
		Period:     period,
	})
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	resp := openapi.ChangeTariffResponse{}
	if result.PaymentID != uuid.Nil {
		id := result.PaymentID
		resp.PaymentId = &id
	}
	if result.ConfirmURL != "" {
		resp.ConfirmUrl = &result.ConfirmURL
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// ListSubscriptionPayments implements GET /subscription/payments (issue #250).
func (h *BillingHandlers) ListSubscriptionPayments(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, r)
}

// ListPaymentMethods implements GET /subscription/payment-methods (issue #251).
func (h *BillingHandlers) ListPaymentMethods(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, r)
}

// SyncPaymentMethods implements POST /subscription/payment-methods/sync
// (issue #251).
func (h *BillingHandlers) SyncPaymentMethods(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, r)
}

// AddPaymentMethod implements POST /subscription/payment-methods (issue #251).
func (h *BillingHandlers) AddPaymentMethod(w http.ResponseWriter, r *http.Request) {
	h.notImplemented(w, r)
}

// DeletePaymentMethod implements DELETE /subscription/payment-methods/{id}
// (issue #251).
func (h *BillingHandlers) DeletePaymentMethod(w http.ResponseWriter, r *http.Request, _ uuid.UUID) {
	h.notImplemented(w, r)
}

// ActivatePaymentMethod implements POST
// /subscription/payment-methods/{id}/activate (issue #251).
func (h *BillingHandlers) ActivatePaymentMethod(w http.ResponseWriter, r *http.Request, _ uuid.UUID) {
	h.notImplemented(w, r)
}

// ConfirmFakeSubscriptionPayment implements
// POST /internal/fake-subscription-payment/{id}/confirm (issue #250).
func (h *BillingHandlers) ConfirmFakeSubscriptionPayment(w http.ResponseWriter, r *http.Request, _ uuid.UUID) {
	h.notImplemented(w, r)
}

// HandlePaymentWebhook implements POST /webhooks/payment/{provider}. The
// synchronous-processing rewrite lands with issue #250; until then there are
// no payments to notify about.
func (h *BillingHandlers) HandlePaymentWebhook(w http.ResponseWriter, r *http.Request, _ string) {
	defer func() { _ = r.Body.Close() }()
	h.notImplemented(w, r)
}

// GetAdminSubscriptionPayment implements GET /admin/subscription/payments/{paymentId}
// (issue #254).
func (h *BillingHandlers) GetAdminSubscriptionPayment(w http.ResponseWriter, r *http.Request, _ uuid.UUID) {
	h.notImplemented(w, r)
}

// RefundSubscriptionPayment implements POST /admin/subscription/payments/{paymentId}/refund
// (issue #254).
func (h *BillingHandlers) RefundSubscriptionPayment(w http.ResponseWriter, r *http.Request, _ uuid.UUID) {
	h.notImplemented(w, r)
}

// SyncSubscriptionPayment implements POST /admin/subscription/payments/{paymentId}/sync
// (issue #254).
func (h *BillingHandlers) SyncSubscriptionPayment(w http.ResponseWriter, r *http.Request, _ uuid.UUID) {
	h.notImplemented(w, r)
}

// ListAdminSubscriptionPayments implements GET /admin/subscription/payments
// (issue #254).
func (h *BillingHandlers) ListAdminSubscriptionPayments(w http.ResponseWriter, r *http.Request, _ openapi.ListAdminSubscriptionPaymentsParams) {
	h.notImplemented(w, r)
}

// handleBillingError maps billing sentinel errors to RFC 7807 problems.
func (h *BillingHandlers) handleBillingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, billingapp.ErrNotFound),
		errors.Is(err, billingapp.ErrTariffNotFound),
		errors.Is(err, billingapp.ErrSubscriptionNotFound):
		httpsupport.WriteProblem(w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Ресурс не найден"))
	case errors.Is(err, billingapp.ErrPaymentUnavailable):
		// Temporary answer for the flows that need a payment until #250 lands;
		// deliberately outside the frozen contract's response list because it
		// disappears with the payment flow.
		httpsupport.WriteProblem(w, http.StatusServiceUnavailable, httpsupport.Problem(r.Context(), "Payment unavailable", "Оплата временно недоступна, попробуйте позже"))
	case errors.Is(err, domain.ErrAlreadyOnTariff),
		errors.Is(err, domain.ErrInvalidTariffChange),
		errors.Is(err, domain.ErrInvalidSubscriptionState),
		errors.Is(err, domain.ErrCannotEnableAutoRenew):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", detail))
	case errors.Is(err, domain.ErrInvalidPeriod),
		errors.Is(err, domain.ErrInvalidTariff):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", detail))
	default:
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}
