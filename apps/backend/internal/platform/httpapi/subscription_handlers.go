package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// SubscriptionHandlers implements the generated subscription and tariff endpoints.
type SubscriptionHandlers struct {
	billing *billingapp.BillingService
	logger  *slog.Logger
	DevMode bool
}

// NewSubscriptionHandlers creates HTTP handlers for the billing API.
func NewSubscriptionHandlers(billing *billingapp.BillingService, logger *slog.Logger, devMode bool) *SubscriptionHandlers {
	return &SubscriptionHandlers{billing: billing, logger: logger, DevMode: devMode}
}

// ListTariffs implements GET /tariffs.
func (h *SubscriptionHandlers) ListTariffs(w http.ResponseWriter, r *http.Request) {
	_, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	tariffs, err := h.billing.ListTariffs(r.Context())
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	view, err := h.billing.GetSubscription(r.Context(), ownerID)
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.AutoRenewRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode auto-renew request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	if err := h.billing.ToggleAutoRenew(r.Context(), ownerID, body.Enabled); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CancelSubscription implements POST /subscription/cancel.
func (h *SubscriptionHandlers) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	if err := h.billing.CancelSubscription(r.Context(), ownerID); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ChangeTariff implements POST /subscription/change.
func (h *SubscriptionHandlers) ChangeTariff(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.ChangeTariffRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode change tariff request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	res, err := h.billing.ChangeTariff(r.Context(), ownerID, billingapp.ChangeTariffRequest{
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	views, err := h.billing.ListPayments(r.Context(), ownerID)
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	methods, err := h.billing.ListPaymentMethods(r.Context(), ownerID)
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.AddPaymentMethodRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode add payment method request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	pm, err := h.billing.AddPaymentMethod(r.Context(), ownerID, billingapp.AddPaymentMethodRequest{ProviderToken: body.ProviderToken})
	if err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, paymentMethodResponse(pm))
}

// DeletePaymentMethod implements DELETE /subscription/payment-methods/{id}.
func (h *SubscriptionHandlers) DeletePaymentMethod(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	if err := h.billing.DeletePaymentMethod(r.Context(), ownerID, id); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ActivatePaymentMethod implements POST /subscription/payment-methods/{id}/activate.
func (h *SubscriptionHandlers) ActivatePaymentMethod(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	if err := h.billing.SetActivePaymentMethod(r.Context(), ownerID, id); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ConfirmFakeSubscriptionPayment implements POST /internal/fake-subscription-payment/{id}/confirm.
func (h *SubscriptionHandlers) ConfirmFakeSubscriptionPayment(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if !h.DevMode {
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "endpoint not available"))
		return
	}

	if err := h.billing.ConfirmFakePayment(r.Context(), id); err != nil {
		h.handleBillingError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, map[string]any{"status": "ok"})
}

const maxWebhookBody = 256 * 1024

// HandlePaymentWebhook implements POST /webhooks/payment/{provider}.
// Webhooks always return 200 OK to avoid leaking payload validity to attackers.
func (h *SubscriptionHandlers) HandlePaymentWebhook(w http.ResponseWriter, r *http.Request, provider string) {
	defer func() { _ = r.Body.Close() }()

	// Read with a hard size cap, but do not use http.MaxBytesReader because it
	// writes a 413 response and prevents us from returning 200.
	payload, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody+1))
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to read webhook body",
			slog.String("provider", provider),
			slog.String("error", err.Error()))
		writeJSON(r.Context(), w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}
	if len(payload) > maxWebhookBody {
		h.logger.ErrorContext(r.Context(), "webhook body exceeds size limit",
			slog.String("provider", provider),
			slog.Int("size", len(payload)))
		writeJSON(r.Context(), w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}

	if err := h.billing.HandleWebhook(r.Context(), provider, payload); err != nil {
		h.logger.ErrorContext(r.Context(), "webhook handling failed",
			slog.String("provider", provider),
			slog.String("error", err.Error()))
	}

	writeJSON(r.Context(), w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *SubscriptionHandlers) handleBillingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, billingapp.ErrNotFound),
		errors.Is(err, billingapp.ErrTariffNotFound),
		errors.Is(err, billingapp.ErrSubscriptionNotFound),
		errors.Is(err, billingapp.ErrPaymentNotFound),
		errors.Is(err, billingapp.ErrPaymentMethodNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "resource not found"))
	case errors.Is(err, billingapp.ErrAlreadyOnTariff),
		errors.Is(err, billingapp.ErrInvalidTariffChange),
		errors.Is(err, billingapp.ErrPaymentMethodInUse),
		errors.Is(err, billingapp.ErrPaymentMethodAlreadyExists),
		errors.Is(err, domain.ErrCannotEnableAutoRenew),
		errors.Is(err, domain.ErrInvalidSubscriptionState):
		detail, _ := UserFacingDetail(err)
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", detail))
	case errors.Is(err, domain.ErrInvalidPeriod),
		errors.Is(err, domain.ErrInvalidAmount):
		detail, _ := UserFacingDetail(err)
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	case errors.Is(err, context.DeadlineExceeded):
		writeProblem(w, http.StatusGatewayTimeout, problem(r.Context(), "Gateway timeout", "request timed out"))
	case errors.Is(err, context.Canceled):
		writeProblem(w, 499, problem(r.Context(), "Client closed request", "client closed request"))
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
	if view.PendingTariff != nil {
		pt := tariffResponse(*view.PendingTariff)
		resp.PendingTariff = &pt
	}
	if sub.PendingPeriod != nil {
		period := openapi.SubscriptionPendingPeriod(*sub.PendingPeriod)
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

func subscriptionPaymentResponse(view billingapp.SubscriptionPaymentView) openapi.SubscriptionPayment {
	p := view.Payment
	return openapi.SubscriptionPayment{
		Id:            p.ID,
		Tariff:        tariffResponse(view.Tariff),
		Period:        openapi.SubscriptionPaymentPeriod(p.Period),
		AmountKopecks: int(p.AmountKopecks),
		Status:        openapi.SubscriptionPaymentStatus(p.Status),
		Provider:      string(p.Provider),
		CreatedAt:     p.CreatedAt,
	}
}
