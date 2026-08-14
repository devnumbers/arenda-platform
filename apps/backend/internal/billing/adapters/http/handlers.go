package http

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

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

// PaymentManager serves the user's payment views and the local fake-payment
// confirmation of issue #250.
type PaymentManager interface {
	ListPayments(ctx context.Context, userID uuid.UUID) ([]billingapp.SubscriptionPaymentView, error)
	ConfirmFakePayment(ctx context.Context, paymentID uuid.UUID) error
	ConfirmFakeCardBinding(ctx context.Context, requestKey string) error
}

// PaymentMethodManager serves the user's saved payment methods (issue #251):
// the card-binding flow, activation, deletion, sync, and the method list.
type PaymentMethodManager interface {
	ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error)
	AddPaymentMethod(ctx context.Context, userID uuid.UUID, req billingapp.AddPaymentMethodRequest) (billingapp.AddPaymentMethodResult, error)
	DeletePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error
	ActivatePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error
	SyncPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error)
}

// AdminPaymentManager serves the admin payment operations (issue #254): the
// cross-user payment views, the full-refund saga and the manual provider
// sync. Access control is the AdminOnlyMiddleware the routes are mounted
// with; the acting admin's id is attributed by the handlers.
type AdminPaymentManager interface {
	GetAdminPayment(ctx context.Context, paymentID uuid.UUID) (billingapp.AdminSubscriptionPaymentView, error)
	ListAdminPayments(ctx context.Context, filters billingapp.AdminPaymentFilters) ([]billingapp.AdminSubscriptionPaymentView, int64, error)
	RefundPayment(ctx context.Context, adminID, paymentID uuid.UUID) error
	SyncPayment(ctx context.Context, adminID, paymentID uuid.UUID) error
}

// WebhookProcessor synchronously applies payment-provider webhooks (issue #250)
// and answers with the provider's fixed acknowledgement body.
type WebhookProcessor interface {
	HandleWebhook(ctx context.Context, providerName string, payload []byte) error
	WebhookAck() []byte
}

// The synchronous webhook budget: the provider waits ~10 s for the
// acknowledgement (T-Kassa's window), so processing must finish well inside it
// — a failed or timed-out delivery is answered non-200 and the provider
// retries. See the "synchronous payment webhooks" ADR.
const webhookProcessBudget = 8 * time.Second

// maxWebhookBody caps the accepted webhook payload size. Oversized bodies are
// rejected without reading them fully.
const maxWebhookBody = 256 * 1024

// BillingHandlers implements the generated billing endpoints of the
// OpenAPI contract. The subscription lifecycle mutations of issue #249 are
// live — PATCH /subscription/auto-renew, POST /subscription/cancel and POST
// /subscription/change (downgrade scheduling; upgrades answer with a temporary
// payment-unavailable error until #250). The payment endpoints of issue #250
// are live — GET /subscription/payments, POST /webhooks/payment/{provider} and
// the local-only fake-payment confirmation. The payment-method endpoints of
// issue #251 are live — the binding flow, activation, deletion, sync and the
// local-only fake card-binding confirmation. The admin payment endpoints of
// issue #254 are live — the cross-user payment views, the full-refund saga
// and the manual provider sync. The user-facing contract is frozen, so the
// routes stay mounted.
type BillingHandlers struct {
	tariffs       TariffLister
	subscriptions SubscriptionViewer
	managers      SubscriptionManager
	payments      PaymentManager
	methods       PaymentMethodManager
	webhooks      WebhookProcessor
	adminPayments AdminPaymentManager
	// devEndpoints enables the local-only fake-payment confirmation endpoint
	// (APP_ENV=local).
	devEndpoints bool
	logger       *slog.Logger
}

// NewBillingHandlers creates HTTP handlers for the billing API.
func NewBillingHandlers(
	tariffs TariffLister,
	subscriptions SubscriptionViewer,
	managers SubscriptionManager,
	payments PaymentManager,
	methods PaymentMethodManager,
	webhooks WebhookProcessor,
	adminPayments AdminPaymentManager,
	devEndpoints bool,
	logger *slog.Logger,
) *BillingHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &BillingHandlers{
		tariffs:       tariffs,
		subscriptions: subscriptions,
		managers:      managers,
		payments:      payments,
		methods:       methods,
		webhooks:      webhooks,
		adminPayments: adminPayments,
		devEndpoints:  devEndpoints,
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

// ListSubscriptionPayments implements GET /subscription/payments (issue #250):
// the user's own subscription payments with their tariffs, newest first.
func (h *BillingHandlers) ListSubscriptionPayments(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	views, err := h.payments.ListPayments(r.Context(), ownerID)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	items := make([]openapi.SubscriptionPayment, 0, len(views))
	for _, v := range views {
		item := openapi.SubscriptionPayment{
			Id:            v.Payment.ID,
			Tariff:        httpsupport.TariffResponse(v.Tariff),
			Period:        openapi.AdminSubscriptionPaymentPeriod(v.Payment.Period),
			Status:        openapi.SubscriptionPaymentStatus(v.Payment.Status),
			AmountKopecks: int(v.Payment.AmountKopecks),
			Provider:      string(v.Payment.Provider),
			CreatedAt:     v.Payment.CreatedAt,
		}
		if v.Payment.HasPaymentURL() {
			url := *v.Payment.PaymentURL
			item.PaymentUrl = &url
		}
		items = append(items, item)
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.SubscriptionPaymentsResponse{Items: items})
}

// ListPaymentMethods implements GET /subscription/payment-methods
// (issue #251): the user's saved methods, newest first.
func (h *BillingHandlers) ListPaymentMethods(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	methods, err := h.methods.ListPaymentMethods(r.Context(), ownerID)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PaymentMethodsResponse{
		Items: paymentMethodItems(methods),
	})
}

// AddPaymentMethod implements POST /subscription/payment-methods
// (issue #251). A raw provider token creates the method synchronously; the
// bank-form flow answers with the binding form URL the payer follows.
func (h *BillingHandlers) AddPaymentMethod(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.AddPaymentMethodRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode add payment method request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	req := billingapp.AddPaymentMethodRequest{}
	if body.ProviderToken != nil {
		req.ProviderToken = *body.ProviderToken
	}
	result, err := h.methods.AddPaymentMethod(r.Context(), ownerID, req)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	resp := openapi.AddPaymentMethodResponse{}
	if result.ConfirmURL != "" {
		confirmURL := result.ConfirmURL
		resp.ConfirmUrl = &confirmURL
	}
	if result.PaymentMethod != nil {
		method := httpsupport.PaymentMethodResponse(*result.PaymentMethod)
		resp.PaymentMethod = &method
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// DeletePaymentMethod implements DELETE /subscription/payment-methods/{id}
// (issue #251). The active method answers 409 until another one is activated.
func (h *BillingHandlers) DeletePaymentMethod(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.methods.DeletePaymentMethod(r.Context(), ownerID, id); err != nil {
		h.handleBillingError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ActivatePaymentMethod implements POST
// /subscription/payment-methods/{id}/activate (issue #251): the method becomes
// the user's single active one and the subscription's charge target.
func (h *BillingHandlers) ActivatePaymentMethod(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.methods.ActivatePaymentMethod(r.Context(), ownerID, id); err != nil {
		h.handleBillingError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SyncPaymentMethods implements POST /subscription/payment-methods/sync
// (issue #251): open binding sessions are polled at the provider and resolved
// (the self-healing path when the add-card webhook was not delivered), then
// the user's up-to-date list is returned.
func (h *BillingHandlers) SyncPaymentMethods(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := httpsupport.OwnerIDFromContext(r)
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	methods, err := h.methods.SyncPaymentMethods(r.Context(), ownerID)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PaymentMethodsResponse{
		Items: paymentMethodItems(methods),
	})
}

// paymentMethodItems maps domain payment methods to the contract DTOs.
func paymentMethodItems(methods []domain.PaymentMethod) []openapi.PaymentMethod {
	items := make([]openapi.PaymentMethod, 0, len(methods))
	for _, m := range methods {
		items = append(items, httpsupport.PaymentMethodResponse(m))
	}
	return items
}

// ConfirmFakeSubscriptionPayment implements
// POST /internal/fake-subscription-payment/{id}/confirm (issue #250). Local
// only (APP_ENV=local): it completes a pending payment at the fake provider so
// the whole payment flow is drivable end-to-end on a developer machine.
func (h *BillingHandlers) ConfirmFakeSubscriptionPayment(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if !h.devEndpoints {
		h.notImplemented(w, r)
		return
	}

	if err := h.payments.ConfirmFakePayment(r.Context(), id); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, map[string]any{"status": "ok"})
}

// ConfirmFakeCardBinding implements
// POST /internal/fake-card-binding/{requestKey}/confirm (issue #251). Local
// only (APP_ENV=local): it completes a card-binding session at the fake
// provider — the counterpart of the payer completing the bank form of a real
// provider — so the binding flow is drivable end-to-end on a developer
// machine.
func (h *BillingHandlers) ConfirmFakeCardBinding(w http.ResponseWriter, r *http.Request, requestKey string) {
	if !h.devEndpoints {
		h.notImplemented(w, r)
		return
	}

	if err := h.payments.ConfirmFakeCardBinding(r.Context(), requestKey); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, map[string]any{"status": "ok"})
}

// HandlePaymentWebhook implements POST /webhooks/payment/{provider}
// (issue #250). The notification is processed synchronously within the
// provider's delivery window: the handler answers 200 with the provider's
// acknowledgement body only after the notification is fully applied (a
// repeated delivery is an idempotent no-op and also answers 200); any failure
// answers non-200 so the provider retries. See the "synchronous payment
// webhooks" ADR.
func (h *BillingHandlers) HandlePaymentWebhook(w http.ResponseWriter, r *http.Request, provider string) {
	defer func() { _ = r.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody+1))
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to read webhook body",
			slog.String("provider", provider),
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}
	if len(payload) > maxWebhookBody {
		h.logger.ErrorContext(r.Context(), "webhook body exceeds size limit",
			slog.String("provider", provider),
			slog.Int("size", len(payload)))
		httpsupport.WriteProblem(w, http.StatusRequestEntityTooLarge, httpsupport.Problem(r.Context(), "Payload too large", "Тело запроса слишком большое"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), webhookProcessBudget)
	defer cancel()

	if err := h.webhooks.HandleWebhook(ctx, provider, payload); err != nil {
		// Non-200: the provider redelivers the notification. Mismatches and
		// parse/verification failures are request defects no retry can fix, so
		// they answer 400; processing failures answer 500 for the retry.
		h.logger.ErrorContext(ctx, "payment webhook processing failed",
			slog.String("provider", provider),
			slog.String("error", httpsupport.SanitizeError(err)))
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, billingapp.ErrWebhookProviderMismatch),
			errors.Is(err, billingapp.ErrWebhookUnsupported),
			errors.Is(err, billingapp.ErrWebhookPaymentMismatch),
			errors.Is(err, billingapp.ErrWebhookRejected):
			status = http.StatusBadRequest
		case errors.Is(err, billingapp.ErrPaymentNotFound):
			status = http.StatusNotFound
		}
		httpsupport.WriteProblem(w, status, httpsupport.Problem(r.Context(), "Webhook not processed", "Уведомление не обработано"))
		return
	}

	ack := h.webhooks.WebhookAck()
	if len(ack) > 0 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(ack)
}

// GetAdminSubscriptionPayment implements GET /admin/subscription/payments/{paymentId}
// (issue #254): the payment with its tariff and the payer's phone resolved.
func (h *BillingHandlers) GetAdminSubscriptionPayment(w http.ResponseWriter, r *http.Request, paymentID uuid.UUID) {
	view, err := h.adminPayments.GetAdminPayment(r.Context(), paymentID)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, adminSubscriptionPaymentResponse(view))
}

// RefundSubscriptionPayment implements POST /admin/subscription/payments/{paymentId}/refund
// (issue #254): the full-amount refund saga. The acting admin — resolved from
// the session the AdminOnlyMiddleware already verified — is attributed in the
// transition log and the audit record.
func (h *BillingHandlers) RefundSubscriptionPayment(w http.ResponseWriter, r *http.Request, paymentID uuid.UUID) {
	adminID, ok := h.adminActor(w, r)
	if !ok {
		return
	}
	if err := h.adminPayments.RefundPayment(r.Context(), adminID, paymentID); err != nil {
		h.handleBillingError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SyncSubscriptionPayment implements POST /admin/subscription/payments/{paymentId}/sync
// (issue #254): the payment is reconciled with the provider's current status
// through the same synchronous paths the webhooks use.
func (h *BillingHandlers) SyncSubscriptionPayment(w http.ResponseWriter, r *http.Request, paymentID uuid.UUID) {
	adminID, ok := h.adminActor(w, r)
	if !ok {
		return
	}
	if err := h.adminPayments.SyncPayment(r.Context(), adminID, paymentID); err != nil {
		h.handleBillingError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListAdminSubscriptionPayments implements GET /admin/subscription/payments
// (issue #254): the cross-user payment listing with the payer's phone, the
// payment-status, subscription-status and phone filters, and the sort
// whitelist of the contract.
func (h *BillingHandlers) ListAdminSubscriptionPayments(w http.ResponseWriter, r *http.Request, params openapi.ListAdminSubscriptionPaymentsParams) {
	filters := billingapp.AdminPaymentFilters{}
	if params.UserId != nil {
		filters.UserID = params.UserId
	}
	if params.Status != nil {
		filters.Status = string(*params.Status)
	}
	if params.UserPhone != nil {
		filters.UserPhone = *params.UserPhone
	}
	if params.SubscriptionStatus != nil {
		filters.SubscriptionStatus = string(*params.SubscriptionStatus)
	}
	if params.Sort != nil {
		filters.Sort = *params.Sort
	}
	if params.Order != nil {
		filters.Order = string(*params.Order)
	}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}

	views, total, err := h.adminPayments.ListAdminPayments(r.Context(), filters)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}
	items := make([]openapi.AdminSubscriptionPayment, 0, len(views))
	for _, v := range views {
		items = append(items, adminSubscriptionPaymentResponse(v))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.AdminSubscriptionPaymentsResponse{
		Items: items,
		Total: int(total),
	})
}

// adminActor resolves the acting admin from the authenticated session. The
// AdminOnlyMiddleware guarantees the admin role on the mounted routes; this
// only extracts the id and answers unauthorized when no session is present.
func (h *BillingHandlers) adminActor(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	adminID, _, ok := httpsupport.ActorFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return uuid.Nil, false
	}
	return adminID, true
}

// adminSubscriptionPaymentResponse maps the admin payment view to the
// contract DTO.
func adminSubscriptionPaymentResponse(view billingapp.AdminSubscriptionPaymentView) openapi.AdminSubscriptionPayment {
	p := view.Payment
	resp := openapi.AdminSubscriptionPayment{
		Id:              p.ID,
		Tariff:          httpsupport.TariffResponse(view.Tariff),
		Period:          openapi.AdminSubscriptionPaymentPeriod(p.Period),
		AmountKopecks:   int(p.AmountKopecks),
		Status:          openapi.SubscriptionPaymentStatus(p.Status),
		Provider:        string(p.Provider),
		UserId:          p.UserID,
		UserPhone:       view.UserPhone,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
		SucceededAt:     p.SucceededAt,
		PaymentMethodId: p.PaymentMethodID,
	}
	if p.RefundedAmountKopecks != nil {
		v := int(*p.RefundedAmountKopecks)
		resp.RefundedAmountKopecks = &v
	}
	return resp
}

// handleBillingError maps billing sentinel errors to RFC 7807 problems.
func (h *BillingHandlers) handleBillingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, billingapp.ErrNotFound),
		errors.Is(err, billingapp.ErrTariffNotFound),
		errors.Is(err, billingapp.ErrSubscriptionNotFound),
		errors.Is(err, billingapp.ErrPaymentNotFound),
		errors.Is(err, billingapp.ErrPaymentMethodNotFound):
		httpsupport.WriteProblem(w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Ресурс не найден"))
	case errors.Is(err, billingapp.ErrPaymentMethodInUse):
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Активный способ оплаты нельзя удалить, пока не выбран другой"))
	case errors.Is(err, billingapp.ErrPaymentUnavailable):
		// Temporary answer for the flows that need a payment until #250 lands;
		// deliberately outside the frozen contract's response list because it
		// disappears with the payment flow.
		httpsupport.WriteProblem(w, http.StatusServiceUnavailable, httpsupport.Problem(r.Context(), "Payment unavailable", "Оплата временно недоступна, попробуйте позже"))
	case errors.Is(err, billingapp.ErrPaymentNotConfirmable):
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Активный провайдер не поддерживает локальное подтверждение платежа"))
	case errors.Is(err, domain.ErrAlreadyOnTariff),
		errors.Is(err, domain.ErrInvalidTariffChange),
		errors.Is(err, domain.ErrInvalidSubscriptionState),
		errors.Is(err, domain.ErrCannotEnableAutoRenew),
		errors.Is(err, domain.ErrInvalidPaymentStatus):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", detail))
	case errors.Is(err, billingapp.ErrInvalidFilter):
		// An admin listing filter outside its whitelist — a request defect,
		// not a server failure (issue #254).
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректные параметры фильтра"))
	case errors.Is(err, domain.ErrInvalidPeriod),
		errors.Is(err, domain.ErrInvalidTariff),
		errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrInvalidPayment):
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
