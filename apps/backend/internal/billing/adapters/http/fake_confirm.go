package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// The local fake-payment confirmation endpoints (issues #250 and #251). They
// exist only when the fake provider is active: the composition root mounts the
// routes only then (issue #287), so a production build has no such endpoints
// at all. The application layer stays provider-neutral — these handlers talk
// to the fake adapter directly and feed the confirmed event through the same
// synchronous application path a webhook takes.

// The route patterns of the two local confirmation endpoints, registered by
// the HTTP wiring only under the fake provider. The fake adapter builds the
// payer-facing confirmation URLs from the same paths (confirmURL and
// bindingConfirmURL) — keep both sides in sync.
const (
	// FakePaymentConfirmRoute is POST /internal/fake-subscription-payment/{id}/confirm.
	FakePaymentConfirmRoute = "/internal/fake-subscription-payment/{id}/confirm"
	// FakeCardBindingConfirmRoute is POST /internal/fake-card-binding/{requestKey}/confirm.
	FakeCardBindingConfirmRoute = "/internal/fake-card-binding/{requestKey}/confirm"
)

// FakeConfirmProvider is the fake adapter's local confirmation capability —
// the dev-only counterpart of the payer completing the provider's form — plus
// the status read the lost-entry fallback resolves from. Declared here at the
// consumer (ADR 0035); only the fake adapter implements it.
type FakeConfirmProvider interface {
	ConfirmPayment(ctx context.Context, internalPaymentID string) (billingapp.WebhookEvent, error)
	ConfirmCardBinding(ctx context.Context, requestKey string) (billingapp.WebhookEvent, error)
	PaymentStatus(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (billingapp.PaymentStatusResult, error)
}

// FakeConfirmBackend is the application side of the local confirmation flow:
// reading the persisted payment and applying a provider event through the same
// synchronous paths a webhook delivery uses.
type FakeConfirmBackend interface {
	GetPayment(ctx context.Context, paymentID uuid.UUID) (domain.SubscriptionPayment, error)
	ApplyProviderEvent(ctx context.Context, event billingapp.WebhookEvent) error
}

// FakeConfirmHandlers implements POST /internal/fake-subscription-payment/{id}/confirm
// and POST /internal/fake-card-binding/{requestKey}/confirm: the local-only
// completion of a pending payment or card-binding session at the fake provider,
// so the whole flow is drivable end-to-end on a developer machine. The
// confirmed event flows through the production webhook path, so local runs
// exercise production behaviour.
type FakeConfirmHandlers struct {
	provider FakeConfirmProvider
	backend  FakeConfirmBackend
	logger   *slog.Logger
}

// NewFakeConfirmHandlers creates the local confirmation handlers over the fake
// adapter and the payment service.
func NewFakeConfirmHandlers(provider FakeConfirmProvider, backend FakeConfirmBackend, logger *slog.Logger) *FakeConfirmHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &FakeConfirmHandlers{
		provider: provider,
		backend:  backend,
		logger:   logger,
	}
}

// ConfirmPayment implements
// POST /internal/fake-subscription-payment/{id}/confirm (issue #250): it
// completes a pending payment at the fake provider. Confirming a finalized
// payment is an idempotent no-op; an unknown payment answers 404.
func (h *FakeConfirmHandlers) ConfirmPayment(w http.ResponseWriter, r *http.Request) {
	paymentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.badRequest(w, r, "Некорректный идентификатор платежа")
		return
	}
	if err := h.confirmPayment(r.Context(), paymentID); err != nil {
		writeBillingError(w, r, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, map[string]any{"status": "ok"})
}

// ConfirmCardBinding implements
// POST /internal/fake-card-binding/{requestKey}/confirm (issue #251): it
// completes a card-binding session at the fake provider — the counterpart of
// the payer completing the bank form of a real provider. Confirming an
// already-resolved binding is an idempotent no-op; an unknown request key
// answers 404.
func (h *FakeConfirmHandlers) ConfirmCardBinding(w http.ResponseWriter, r *http.Request) {
	requestKey := chi.URLParam(r, "requestKey")
	if requestKey == "" {
		h.badRequest(w, r, "Некорректный идентификатор сессии")
		return
	}
	if err := h.confirmCardBinding(r.Context(), requestKey); err != nil {
		writeBillingError(w, r, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, map[string]any{"status": "ok"})
}

// confirmPayment completes a pending payment at the fake provider and applies
// the confirmed event. The provider no longer tracking the entry (a TTL purge
// or an earlier confirmation) resolves from the provider status — the source
// of truth (ADR 0010).
func (h *FakeConfirmHandlers) confirmPayment(ctx context.Context, paymentID uuid.UUID) error {
	payment, err := h.backend.GetPayment(ctx, paymentID)
	if err != nil {
		return err
	}
	if payment.IsFinalized() {
		return nil
	}

	event, err := h.provider.ConfirmPayment(ctx, paymentID.String())
	if err != nil {
		if errors.Is(err, billingapp.ErrProviderPaymentNotFound) && payment.HasProviderReference() {
			return h.confirmFromProviderStatus(ctx, payment)
		}
		return fmt.Errorf("confirm fake payment: %w", err)
	}
	return h.backend.ApplyProviderEvent(ctx, event)
}

// confirmFromProviderStatus finalizes a payment from the provider's current
// status after its local confirmation could not resolve the entry.
func (h *FakeConfirmHandlers) confirmFromProviderStatus(ctx context.Context, payment domain.SubscriptionPayment) error {
	status, err := h.provider.PaymentStatus(ctx, payment.ID, *payment.ProviderPaymentID)
	if err != nil {
		return fmt.Errorf("provider status after lost confirm: %w", err)
	}
	if status.Status != domain.PaymentStatusSucceeded && status.Status != domain.PaymentStatusFailed {
		return nil
	}
	var errorCode *string
	if status.Status == domain.PaymentStatusFailed && status.ErrorCode != "" {
		errorCode = &status.ErrorCode
	}
	return h.backend.ApplyProviderEvent(ctx, billingapp.WebhookEvent{Payment: &billingapp.PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            status.Status,
		ErrorCode:         errorCode,
		AmountKopecks:     payment.AmountKopecks,
	}})
}

// confirmCardBinding completes a binding session at the fake provider and
// applies the confirmed method-bound event.
func (h *FakeConfirmHandlers) confirmCardBinding(ctx context.Context, requestKey string) error {
	event, err := h.provider.ConfirmCardBinding(ctx, requestKey)
	if err != nil {
		if errors.Is(err, billingapp.ErrProviderBindingNotFound) {
			return billingapp.ErrNotFound
		}
		return fmt.Errorf("confirm fake card binding: %w", err)
	}
	return h.backend.ApplyProviderEvent(ctx, event)
}

// badRequest answers a malformed path parameter with a 400 problem.
func (h *FakeConfirmHandlers) badRequest(w http.ResponseWriter, r *http.Request, detail string) {
	h.logger.WarnContext(r.Context(), "fake confirm request rejected", slog.String("detail", detail))
	httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", detail))
}
