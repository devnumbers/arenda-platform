package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

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

// The route patterns of the local confirmation endpoints, registered by
// MountRoutes only under the fake provider. The fake adapter builds the
// payer-facing confirmation URLs from the same paths (confirmURL and
// bindingConfirmURL) — keep both sides in sync.
const (
	// FakePaymentConfirmRoute is /internal/fake-subscription-payment/{id}/confirm
	// — POST for scripts, and in the auto simulator mode also the GET the
	// browser lands on when it returns from the «bank».
	FakePaymentConfirmRoute = "/internal/fake-subscription-payment/{id}/confirm"
	// FakeCardBindingConfirmRoute is /internal/fake-card-binding/{requestKey}/confirm.
	FakeCardBindingConfirmRoute = "/internal/fake-card-binding/{requestKey}/confirm"
	// FakePaymentFailRoute is POST /internal/fake-subscription-payment/{id}/fail
	// — the manual simulator mode's decline endpoint; auto mode never mounts it.
	FakePaymentFailRoute = "/internal/fake-subscription-payment/{id}/fail"
)

// FakeConfirmProvider is the fake adapter's local confirmation capability —
// the dev-only counterpart of the payer completing the provider's form — plus
// the status read the lost-entry fallback resolves from. Declared here at the
// consumer (ADR 0035); only the fake adapter implements it.
type FakeConfirmProvider interface {
	ConfirmPayment(ctx context.Context, internalPaymentID string) (billingapp.WebhookEvent, error)
	ConfirmCardBinding(ctx context.Context, requestKey string) (billingapp.WebhookEvent, error)
	ConfirmPaymentFailed(ctx context.Context, internalPaymentID string, errorCode *string) (billingapp.WebhookEvent, error)
	PaymentStatus(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (billingapp.PaymentStatusResult, error)
}

// FakeConfirmBackend is the application side of the local confirmation flow:
// reading the persisted payment and applying a provider event through the same
// synchronous paths a webhook delivery uses.
type FakeConfirmBackend interface {
	GetPayment(ctx context.Context, paymentID uuid.UUID) (domain.SubscriptionPayment, error)
	ApplyProviderEvent(ctx context.Context, event billingapp.WebhookEvent) error
}

// FakeConfirmHandlers implements the local confirmation endpoints of the fake
// provider: the completion of a pending payment or card-binding session, so
// the whole flow is drivable end-to-end on a developer machine. The confirmed
// event always flows through the production webhook path, so local runs
// exercise production behaviour. Two simulator modes (issue #663):
//
//   - auto (default) — the confirm URLs double as the browser's bank-return:
//     GET completes the operation and answers 303 into the frontend, while
//     POST stays scriptable;
//   - manual (off) — POST-only confirms, plus the fail endpoint that
//     completes a payment as declined for pending/TTL/decline scenarios.
type FakeConfirmHandlers struct {
	provider    FakeConfirmProvider
	backend     FakeConfirmBackend
	logger      *slog.Logger
	webOrigin   string
	autoConfirm bool
}

// NewFakeConfirmHandlers creates the local confirmation handlers over the
// fake adapter and the payment service. The webOrigin argument is the
// frontend origin the GET bank-return redirects into (without a trailing
// slash); the autoConfirm flag enables the GET confirm routes of the auto
// simulator mode.
func NewFakeConfirmHandlers(
	provider FakeConfirmProvider, backend FakeConfirmBackend, logger *slog.Logger,
	webOrigin string, autoConfirm bool,
) *FakeConfirmHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &FakeConfirmHandlers{
		provider:    provider,
		backend:     backend,
		logger:      logger,
		webOrigin:   strings.TrimRight(webOrigin, "/"),
		autoConfirm: autoConfirm,
	}
}

// MountRoutes registers the local confirmation endpoints on r according to
// the simulator mode — the single source of truth the HTTP wiring uses:
// auto adds the GET bank-return routes that complete and redirect, manual
// (off) adds the fail endpoint; POST confirms exist in both modes.
func (h *FakeConfirmHandlers) MountRoutes(r chi.Router) {
	r.Post(FakePaymentConfirmRoute, h.ConfirmPayment)
	r.Post(FakeCardBindingConfirmRoute, h.ConfirmCardBinding)
	if h.autoConfirm {
		r.Get(FakePaymentConfirmRoute, h.ConfirmPaymentRedirect)
		r.Get(FakeCardBindingConfirmRoute, h.ConfirmCardBindingRedirect)
	} else {
		r.Post(FakePaymentFailRoute, h.FailPayment)
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
	h.writeConfirmAck(r, w)
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
	h.writeConfirmAck(r, w)
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

// ConfirmPaymentRedirect implements the GET side of
// /internal/fake-subscription-payment/{id}/confirm — the browser's return from
// the fake «bank» (issue #663): it completes the payment through the same
// webhook path as the POST and answers 303 into the frontend payment screen,
// whose polling flips the UI to the actual outcome. Failure paths keep the
// POST contract (problem responses) — only the completed operation redirects.
func (h *FakeConfirmHandlers) ConfirmPaymentRedirect(w http.ResponseWriter, r *http.Request) {
	paymentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.badRequest(w, r, "Некорректный идентификатор платежа")
		return
	}
	if err := h.confirmPayment(r.Context(), paymentID); err != nil {
		writeBillingError(w, r, err)
		return
	}
	http.Redirect(w, r, h.paymentReturnURL(paymentID), http.StatusSeeOther)
}

// ConfirmCardBindingRedirect implements the GET side of
// /internal/fake-card-binding/{requestKey}/confirm — the browser's return from
// the fake bank's add-card form (issue #663): it completes the binding session
// and answers 303 into «Способы оплаты», whose addCardResult effect on mount
// refreshes the list.
func (h *FakeConfirmHandlers) ConfirmCardBindingRedirect(w http.ResponseWriter, r *http.Request) {
	requestKey := chi.URLParam(r, "requestKey")
	if requestKey == "" {
		h.badRequest(w, r, "Некорректный идентификатор сессии")
		return
	}
	if err := h.confirmCardBinding(r.Context(), requestKey); err != nil {
		writeBillingError(w, r, err)
		return
	}
	http.Redirect(w, r, h.bindingReturnURL(), http.StatusSeeOther)
}

// FailPayment implements POST /internal/fake-subscription-payment/{id}/fail —
// the manual simulator mode's decline endpoint (issue #663): it completes a
// pending payment as failed through the webhook path, so decline scenarios
// stay drivable by scripts. The auto mode never mounts it — declines there
// are encoded by the sentinel amounts.
func (h *FakeConfirmHandlers) FailPayment(w http.ResponseWriter, r *http.Request) {
	paymentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.badRequest(w, r, "Некорректный идентификатор платежа")
		return
	}
	event, err := h.provider.ConfirmPaymentFailed(r.Context(), paymentID.String(), nil)
	if err != nil {
		if errors.Is(err, billingapp.ErrProviderPaymentNotFound) {
			writeBillingError(w, r, billingapp.ErrNotFound)
			return
		}
		writeBillingError(w, r, fmt.Errorf("fail fake payment: %w", err))
		return
	}
	if err := h.backend.ApplyProviderEvent(r.Context(), event); err != nil {
		writeBillingError(w, r, err)
		return
	}
	h.writeConfirmAck(r, w)
}

// writeConfirmAck answers the JSON acknowledgement the script-facing POST
// endpoints share.
func (h *FakeConfirmHandlers) writeConfirmAck(r *http.Request, w http.ResponseWriter) {
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, map[string]any{"status": "ok"})
}

// The frontend return route the GET bank-redirects land on (issue #663).
// It mirrors an existing frontend route (ROUTES in apps/frontend): the
// change success screen polls the payment status by its paymentId query
// param. The card-binding return shares httpsupport.AddCardReturnURL with
// the T-Kassa add-card return — the same entry point by design.
const paymentReturnPath = "/profile/tariff/change/success"

// paymentReturnURL builds the frontend redirect target of a completed form
// payment: the payment screen keyed by paymentId — its polling resolves the
// actual outcome (succeeded, failed, still pending).
func (h *FakeConfirmHandlers) paymentReturnURL(paymentID uuid.UUID) string {
	q := url.Values{"paymentId": {paymentID.String()}}
	return h.webOrigin + paymentReturnPath + "?" + q.Encode()
}

// bindingReturnURL builds the frontend redirect target of a completed card
// binding: «Способы оплаты» with the addCardResult flag its mount effect
// consumes (the same entry point the T-Kassa add-card return uses).
func (h *FakeConfirmHandlers) bindingReturnURL() string {
	return httpsupport.AddCardReturnURL(h.webOrigin, httpsupport.AddCardResultSuccess)
}

// badRequest answers a malformed path parameter with a 400 problem.
func (h *FakeConfirmHandlers) badRequest(w http.ResponseWriter, r *http.Request, detail string) {
	h.logger.WarnContext(r.Context(), "fake confirm request rejected", slog.String("detail", detail))
	httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", detail))
}
