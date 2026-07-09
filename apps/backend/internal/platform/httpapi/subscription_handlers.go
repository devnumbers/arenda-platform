package httpapi

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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// SubscriptionHandlers implements the generated subscription and tariff endpoints.
type SubscriptionHandlers struct {
	tariffs        billingapp.Tariffer
	subscriptions  billingapp.Subscriber
	paymentMethods billingapp.PaymentMethodManager
	payments       billingapp.PaymentProcessor
	webhooks       billingapp.WebhookHandler
	logger         *slog.Logger
	DevMode        bool
}

// NewSubscriptionHandlers creates HTTP handlers for the billing API.
func NewSubscriptionHandlers(
	tariffs billingapp.Tariffer,
	subscriptions billingapp.Subscriber,
	paymentMethods billingapp.PaymentMethodManager,
	payments billingapp.PaymentProcessor,
	webhooks billingapp.WebhookHandler,
	logger *slog.Logger,
	devMode bool,
) *SubscriptionHandlers {
	return &SubscriptionHandlers{
		tariffs:        tariffs,
		subscriptions:  subscriptions,
		paymentMethods: paymentMethods,
		payments:       payments,
		webhooks:       webhooks,
		logger:         logger,
		DevMode:        devMode,
	}
}

// ListTariffs implements GET /tariffs.
func (h *SubscriptionHandlers) ListTariffs(w http.ResponseWriter, r *http.Request) {
	_, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	tariffs, err := h.tariffs.ListTariffs(r.Context())
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	items := make([]openapi.Tariff, 0, len(tariffs))
	for _, t := range tariffs {
		items = append(items, tariffResponse(t))
	}
	writeJSON(r.Context(), w, http.StatusOK, openapi.TariffsResponse{Items: items})
}

// GetSubscription implements GET /subscription.
func (h *SubscriptionHandlers) GetSubscription(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	view, err := h.subscriptions.GetSubscription(r.Context(), ownerID)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, subscriptionResponse(view))
}

// ToggleAutoRenew implements PATCH /subscription/auto-renew.
func (h *SubscriptionHandlers) ToggleAutoRenew(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.AutoRenewRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode auto-renew request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	if err := h.subscriptions.ToggleAutoRenew(r.Context(), ownerID, body.Enabled); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CancelSubscription implements POST /subscription/cancel.
func (h *SubscriptionHandlers) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.subscriptions.CancelSubscription(r.Context(), ownerID); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ChangeTariff implements POST /subscription/change.
func (h *SubscriptionHandlers) ChangeTariff(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.ChangeTariffRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode change tariff request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	res, err := h.subscriptions.ChangeTariff(r.Context(), ownerID, billingapp.ChangeTariffRequest{
		TariffName: string(body.TariffName),
		Period:     domain.SubscriptionPeriod(body.Period),
	})
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	resp := openapi.ChangeTariffResponse{}
	if res.PaymentID != uuid.Nil {
		id := res.PaymentID
		resp.PaymentId = &id
	}
	if res.ConfirmURL != "" {
		resp.ConfirmUrl = &res.ConfirmURL
	}
	writeJSON(r.Context(), w, http.StatusOK, resp)
}

// ListSubscriptionPayments implements GET /subscription/payments.
func (h *SubscriptionHandlers) ListSubscriptionPayments(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	views, err := h.payments.ListPayments(r.Context(), ownerID)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	items := make([]openapi.SubscriptionPayment, 0, len(views))
	for _, v := range views {
		items = append(items, subscriptionPaymentResponse(v))
	}
	writeJSON(r.Context(), w, http.StatusOK, openapi.SubscriptionPaymentsResponse{Items: items})
}

// ListPaymentMethods implements GET /subscription/payment-methods.
func (h *SubscriptionHandlers) ListPaymentMethods(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	methods, err := h.paymentMethods.ListPaymentMethods(r.Context(), ownerID)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	items := make([]openapi.PaymentMethod, 0, len(methods))
	for _, m := range methods {
		items = append(items, *paymentMethodResponse(m))
	}
	writeJSON(r.Context(), w, http.StatusOK, openapi.PaymentMethodsResponse{Items: items})
}

// AddPaymentMethod implements POST /subscription/payment-methods.
func (h *SubscriptionHandlers) AddPaymentMethod(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.AddPaymentMethodRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode add payment method request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	req := billingapp.AddPaymentMethodRequest{}
	if body.ProviderToken != nil {
		req.ProviderToken = *body.ProviderToken
	}
	resp, err := h.paymentMethods.AddPaymentMethod(r.Context(), ownerID, req)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	apiResp := openapi.AddPaymentMethodResponse{}
	if resp.PaymentMethod != nil {
		apiResp.PaymentMethod = paymentMethodResponse(*resp.PaymentMethod)
	}
	if resp.ConfirmURL != "" {
		apiResp.ConfirmUrl = &resp.ConfirmURL
	}
	writeJSON(r.Context(), w, http.StatusOK, apiResp)
}

// DeletePaymentMethod implements DELETE /subscription/payment-methods/{id}.
func (h *SubscriptionHandlers) DeletePaymentMethod(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.paymentMethods.DeletePaymentMethod(r.Context(), ownerID, id); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ActivatePaymentMethod implements POST /subscription/payment-methods/{id}/activate.
func (h *SubscriptionHandlers) ActivatePaymentMethod(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.paymentMethods.SetActivePaymentMethod(r.Context(), ownerID, id); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ConfirmFakeSubscriptionPayment implements POST /internal/fake-subscription-payment/{id}/confirm.
func (h *SubscriptionHandlers) ConfirmFakeSubscriptionPayment(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if !h.DevMode {
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "Метод недоступен"))
		return
	}

	if err := h.payments.ConfirmFakePayment(r.Context(), id); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, map[string]any{"status": "ok"})
}

const (
	maxWebhookBody        = 256 * 1024
	webhookProcessTimeout = 30 * time.Second
)

// HandlePaymentWebhook implements POST /webhooks/payment/{provider}.
// Webhooks always return 200 OK to avoid leaking payload validity to attackers.
// The actual business processing is done asynchronously so that slow or
// temporarily unavailable downstream dependencies cannot trigger the provider's
// webhook timeout (e.g. T-Kassa's 10-second window).
func (h *SubscriptionHandlers) HandlePaymentWebhook(w http.ResponseWriter, r *http.Request, provider string) {
	defer func() { _ = r.Body.Close() }()

	// Read with a hard size cap, but do not use http.MaxBytesReader because it
	// writes a 413 response and prevents us from returning 200.
	payload, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody+1))
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to read webhook body",
			slog.String("provider", provider),
			slog.String("error", sanitizeError(err)))
		writeWebhookResponse(w, h.webhooks.WebhookResponse())
		return
	}
	if len(payload) > maxWebhookBody {
		h.logger.ErrorContext(r.Context(), "webhook body exceeds size limit",
			slog.String("provider", provider),
			slog.Int("size", len(payload)))
		writeWebhookResponse(w, h.webhooks.WebhookResponse())
		return
	}

	h.logger.InfoContext(r.Context(), "payment webhook received",
		slog.String("provider", provider),
		slog.String("remote_addr", r.RemoteAddr),
		slog.Int("body_size", len(payload)))

	// Respond immediately to satisfy the provider's timeout window, then process
	// the webhook in a background goroutine with its own timeout and recovery.
	writeWebhookResponse(w, h.webhooks.WebhookResponse())

	reqCtx := r.Context()
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				h.logger.ErrorContext(reqCtx, "panic processing payment webhook",
					slog.String("provider", provider),
					slog.Any("recover", rec))
			}
		}()

		ctx, cancel := context.WithTimeout(context.WithoutCancel(reqCtx), webhookProcessTimeout)
		defer cancel()

		if err := h.webhooks.HandleWebhook(ctx, provider, payload); err != nil {
			h.logger.ErrorContext(ctx, "payment webhook processing failed",
				slog.String("provider", provider),
				slog.String("error", sanitizeError(err)))
			return
		}
		h.logger.InfoContext(ctx, "payment webhook processed",
			slog.String("provider", provider))
	}()
}

func writeWebhookResponse(w http.ResponseWriter, responseBody []byte) {
	if len(responseBody) > 0 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(responseBody)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *SubscriptionHandlers) handleBillingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, billingapp.ErrNotFound),
		errors.Is(err, billingapp.ErrTariffNotFound),
		errors.Is(err, billingapp.ErrSubscriptionNotFound),
		errors.Is(err, billingapp.ErrPaymentNotFound),
		errors.Is(err, billingapp.ErrPaymentMethodNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "Ресурс не найден"))
	case errors.Is(err, billingapp.ErrAlreadyOnTariff),
		errors.Is(err, billingapp.ErrInvalidTariffChange),
		errors.Is(err, billingapp.ErrPaymentMethodInUse),
		errors.Is(err, billingapp.ErrPaymentMethodAlreadyExists),
		errors.Is(err, domain.ErrCannotEnableAutoRenew),
		errors.Is(err, domain.ErrInvalidSubscriptionState),
		errors.Is(err, domain.ErrInvalidPaymentStatus):
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", detail))
	case errors.Is(err, domain.ErrInvalidPeriod),
		errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, billingapp.ErrInvalidFilter):
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	case errors.Is(err, context.DeadlineExceeded):
		writeProblem(w, http.StatusGatewayTimeout, problem(r.Context(), "Gateway timeout", "Время ожидания запроса истекло"))
	case errors.Is(err, context.Canceled):
		writeProblem(w, 499, problem(r.Context(), "Client closed request", "Запрос отменён клиентом"))
	default:
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
	}
}

func tariffResponse(t domain.Tariff) openapi.Tariff {
	return openapi.Tariff{
		Name:                openapi.TariffName(t.Name),
		ActivePropertyLimit: t.ActivePropertyLimit,
		MonthlyPriceKopecks: int(t.MonthlyPriceKopecks),
		YearlyPriceKopecks:  int(t.YearlyPriceKopecks),
	}
}

func subscriptionResponse(view billingapp.SubscriptionView) openapi.Subscription {
	sub := view.Subscription
	resp := openapi.Subscription{
		Tariff:           tariffResponse(view.Tariff),
		Status:           openapi.SubscriptionStatus(sub.Status),
		ValidUntil:       sub.ValidUntil,
		AutoRenewEnabled: sub.AutoRenewEnabled,
		PendingChangeAt:  sub.PendingChangeAt,
	}
	if sub.CurrentPeriod != nil {
		period := openapi.AdminSubscriptionPaymentPeriod(*sub.CurrentPeriod)
		resp.CurrentPeriod = &period
	}
	if view.PendingTariff != nil {
		pt := tariffResponse(*view.PendingTariff)
		resp.PendingTariff = &pt
	}
	if sub.PendingPeriod != nil {
		period := openapi.AdminSubscriptionPaymentPeriod(*sub.PendingPeriod)
		resp.PendingPeriod = &period
	}
	if view.ActivePaymentMethod != nil {
		resp.ActivePaymentMethod = paymentMethodResponse(*view.ActivePaymentMethod)
	}
	return resp
}

func paymentMethodResponse(pm domain.PaymentMethod) *openapi.PaymentMethod {
	return &openapi.PaymentMethod{
		Id:          pm.ID,
		Provider:    string(pm.Provider),
		DisplayMask: pm.DisplayMask,
		IsActive:    pm.IsActive,
		CreatedAt:   pm.CreatedAt,
	}
}

// GetAdminSubscriptionPayment implements GET /admin/subscription/payments/{paymentId}.
func (h *SubscriptionHandlers) GetAdminSubscriptionPayment(w http.ResponseWriter, r *http.Request, paymentId uuid.UUID) {
	view, err := h.payments.GetPayment(r.Context(), paymentId)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, adminSubscriptionPaymentResponse(view))
}

// RefundSubscriptionPayment implements POST /admin/subscription/payments/{paymentId}/refund.
func (h *SubscriptionHandlers) RefundSubscriptionPayment(w http.ResponseWriter, r *http.Request, paymentId uuid.UUID) {
	if err := h.payments.RefundPayment(r.Context(), paymentId); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func subscriptionPaymentResponse(view billingapp.SubscriptionPaymentView) openapi.SubscriptionPayment {
	p := view.Payment
	return openapi.SubscriptionPayment{
		Id:            p.ID,
		Tariff:        tariffResponse(view.Tariff),
		Period:        openapi.AdminSubscriptionPaymentPeriod(p.Period),
		AmountKopecks: int(p.AmountKopecks),
		Status:        openapi.SubscriptionPaymentStatus(p.Status),
		Provider:      string(p.Provider),
		CreatedAt:     p.CreatedAt,
	}
}

func adminSubscriptionPaymentResponse(view billingapp.AdminSubscriptionPaymentView) openapi.AdminSubscriptionPayment {
	p := view.Payment
	resp := openapi.AdminSubscriptionPayment{
		Id:              p.ID,
		Tariff:          tariffResponse(view.Tariff),
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

// SyncSubscriptionPayment implements POST /admin/subscription/payments/{paymentId}/sync.
func (h *SubscriptionHandlers) SyncSubscriptionPayment(w http.ResponseWriter, r *http.Request, paymentId uuid.UUID) {
	if err := h.payments.SyncPendingPayment(r.Context(), paymentId); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListAdminSubscriptionPayments implements GET /admin/subscription/payments.
func (h *SubscriptionHandlers) ListAdminSubscriptionPayments(w http.ResponseWriter, r *http.Request, params openapi.ListAdminSubscriptionPaymentsParams) {
	filters := billingapp.ListAllPaymentsFilters{
		Limit:  20,
		Offset: 0,
	}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}
	if params.Status != nil {
		filters.Status = string(*params.Status)
	}
	if params.UserId != nil {
		filters.UserID = *params.UserId
	}

	views, total, err := h.payments.ListAllPayments(r.Context(), filters)
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	items := make([]openapi.AdminSubscriptionPayment, 0, len(views))
	for _, v := range views {
		items = append(items, adminSubscriptionPaymentResponse(v))
	}
	writeJSON(r.Context(), w, http.StatusOK, openapi.AdminSubscriptionPaymentsResponse{
		Items: items,
		Total: int(total),
	})
}
