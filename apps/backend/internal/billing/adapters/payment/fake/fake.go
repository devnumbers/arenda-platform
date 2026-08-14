// Package fake implements a deterministic local payment provider for
// development and tests. It satisfies the same provider-neutral port as the
// real adapter, so application-layer tests drive payments through it exactly
// like production code drives the real provider (issue #248).
package fake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/logger"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// ProviderName is the provider identity payments persist as their provider
// (ADR 0038).
const ProviderName domain.PaymentProvider = "fake"

const (
	pendingTTL                  = 24 * time.Hour
	defaultErrorCode            = "fake_error"
	fakeFailTokenPrefix         = "fake_fail_"
	fakeProviderPaymentIDPrefix = "fake_"
)

type pendingEntry struct {
	event         application.WebhookEvent
	savedMethod   application.SavedMethod
	amountKopecks int64
	createdAt     time.Time
}

// Provider is a fake payment processor that keeps pending payments in memory.
type Provider struct {
	baseURL          string
	log              *slog.Logger
	clock            clock.Clock
	mu               sync.Mutex
	pending          map[string]pendingEntry
	confirmedAmounts map[string]int64
	bindingStates    map[string]application.MethodBindingState
	metrics          *payment.Metrics
}

// Compile-time assertions that Provider satisfies the aggregate provider port
// and each of its narrow capability interfaces.
var (
	_ application.PaymentProvider            = (*Provider)(nil)
	_ application.PaymentInitiator           = (*Provider)(nil)
	_ application.PaymentCharger             = (*Provider)(nil)
	_ application.PaymentStatusReader        = (*Provider)(nil)
	_ application.PaymentRefunder            = (*Provider)(nil)
	_ application.WebhookParser              = (*Provider)(nil)
	_ application.WebhookResponder           = (*Provider)(nil)
	_ application.PaymentMethodBinder        = (*Provider)(nil)
	_ application.PaymentMethodBindingReader = (*Provider)(nil)
	_ application.PaymentMethodRemover       = (*Provider)(nil)
	_ application.PaymentMethodLister        = (*Provider)(nil)
	_ application.ProviderNamer              = (*Provider)(nil)
)

// Name returns the provider identity used by the application layer.
func (p *Provider) Name() domain.PaymentProvider {
	return ProviderName
}

// NewProvider creates a fake provider for local development and tests.
func NewProvider(baseURL string, log *slog.Logger, clk clock.Clock, metrics *payment.Metrics) *Provider {
	return &Provider{
		baseURL:          strings.TrimRight(baseURL, "/"),
		log:              log,
		clock:            clk,
		pending:          make(map[string]pendingEntry),
		confirmedAmounts: make(map[string]int64),
		bindingStates:    make(map[string]application.MethodBindingState),
		metrics:          metrics,
	}
}

// InitPayment creates a pending payment and returns a confirmation URL.
// Calling InitPayment twice with the same internal payment id is idempotent:
// the existing provider payment id and confirmation URL are returned.
func (p *Provider) InitPayment(ctx context.Context, req application.InitPaymentRequest) (res application.InitPaymentResult, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "Init", status, time.Since(start))
	}()

	if req.PaymentID == uuid.Nil {
		return application.InitPaymentResult{}, errors.New("fake: payment id is required")
	}
	if req.AmountKopecks <= 0 {
		return application.InitPaymentResult{}, errors.New("fake: amount must be positive")
	}
	if !req.Initiator.Valid() {
		return application.InitPaymentResult{}, fmt.Errorf("fake: unknown operation initiator %q", req.Initiator)
	}

	log := logger.WithCorrelation(ctx, p.log)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()

	if entry, ok := p.pending[req.PaymentID.String()]; ok {
		log.InfoContext(ctx, "fake payment init is idempotent",
			"provider_payment_id", entry.event.Payment.ProviderPaymentID,
			"internal_payment_id", req.PaymentID.String(),
		)
		return application.InitPaymentResult{
			ProviderPaymentID: entry.event.Payment.ProviderPaymentID,
			PaymentURL:        p.confirmURL(entry.event.Payment.InternalPaymentID),
			Status:            domain.PaymentStatusPending,
			SavedMethod:       &entry.savedMethod,
		}, nil
	}

	providerPaymentID := fakeProviderPaymentIDPrefix + uuid.Must(uuid.NewV7()).String()
	savedMethod := application.SavedMethod{
		ChargeToken: "fake_token_" + uuid.Must(uuid.NewV7()).String(),
		CustomerRef: req.CustomerRef,
	}

	p.pending[req.PaymentID.String()] = pendingEntry{
		event: application.WebhookEvent{
			Payment: &application.PaymentNotification{
				ProviderPaymentID: providerPaymentID,
				InternalPaymentID: req.PaymentID,
				Status:            domain.PaymentStatusPending,
			},
		},
		savedMethod:   savedMethod,
		amountKopecks: req.AmountKopecks,
		createdAt:     p.clock.Now().UTC(),
	}

	log.InfoContext(ctx, "fake payment initialized",
		"provider_payment_id", providerPaymentID,
		"internal_payment_id", req.PaymentID.String(),
		"customer_ref", req.CustomerRef,
		"amount_kopecks", req.AmountKopecks,
		"period", req.Period,
	)

	// The fake provider binds the charge token to the payment synchronously —
	// the counterpart of the token-delivery webhook of a real provider.
	return application.InitPaymentResult{
		ProviderPaymentID: providerPaymentID,
		PaymentURL:        p.confirmURL(req.PaymentID),
		Status:            domain.PaymentStatusPending,
		SavedMethod:       &savedMethod,
	}, nil
}

func (p *Provider) confirmURL(internalPaymentID uuid.UUID) string {
	return fmt.Sprintf("%s/internal/fake-subscription-payment/%s/confirm", p.baseURL, internalPaymentID.String())
}

// PaymentURL returns the confirmation URL for a previously initialized fake payment.
// It is a fake-specific extra used by the local confirmation handler, not part
// of the provider port.
func (p *Provider) PaymentURL(internalPaymentID uuid.UUID) string {
	return p.confirmURL(internalPaymentID)
}

// PaymentStatus returns the provider-side status of a payment. If the payment
// is not found in the pending map it is assumed to have been completed and
// succeeded.
func (p *Provider) PaymentStatus(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (res application.PaymentStatusResult, err error) {
	start := time.Now()
	defer func() {
		recStatus := "ok"
		if err != nil {
			recStatus = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "Status", recStatus, time.Since(start))
	}()

	_ = providerPaymentID
	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()

	if entry, ok := p.pending[paymentID.String()]; ok {
		res := application.PaymentStatusResult{Status: entry.event.Payment.Status}
		if entry.event.Payment.Status == domain.PaymentStatusFailed {
			res.ErrorCode = defaultErrorCode
		}
		return res, nil
	}
	return application.PaymentStatusResult{Status: domain.PaymentStatusSucceeded}, nil
}

// ChargePayment performs a merchant-initiated charge using a saved token.
func (p *Provider) ChargePayment(ctx context.Context, req application.ChargeRequest) (res application.ChargeResult, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "Charge", status, time.Since(start))
	}()

	if req.PaymentID == uuid.Nil {
		return application.ChargeResult{}, errors.New("fake: payment id is required")
	}
	if req.AmountKopecks <= 0 {
		return application.ChargeResult{}, errors.New("fake: amount must be positive")
	}

	log := logger.WithCorrelation(ctx, p.log)

	providerPaymentID := fakeProviderPaymentIDPrefix + uuid.Must(uuid.NewV7()).String()

	if strings.HasPrefix(req.ChargeToken, fakeFailTokenPrefix) {
		log.InfoContext(ctx, "fake charge failed",
			"provider_payment_id", providerPaymentID,
			"internal_payment_id", req.PaymentID.String(),
		)
		return application.ChargeResult{
			ProviderPaymentID: providerPaymentID,
			Status:            domain.PaymentStatusFailed,
			ErrorCode:         defaultErrorCode,
		}, nil
	}

	log.InfoContext(ctx, "fake charge succeeded",
		"provider_payment_id", providerPaymentID,
		"internal_payment_id", req.PaymentID.String(),
	)
	p.mu.Lock()
	p.confirmedAmounts[providerPaymentID] = req.AmountKopecks
	p.mu.Unlock()
	return application.ChargeResult{
		ProviderPaymentID: providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
	}, nil
}

// BindPaymentMethod is not supported by the fake provider.
func (p *Provider) BindPaymentMethod(ctx context.Context, req application.BindMethodRequest) (res application.BindMethodResult, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "BindPaymentMethod", status, time.Since(start))
	}()

	_ = req
	return application.BindMethodResult{}, errors.New("fake: payment method binding is not supported")
}

// RemovePaymentMethod is a no-op for the fake provider.
func (p *Provider) RemovePaymentMethod(ctx context.Context, customerRef, providerMethodID string) (err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "RemovePaymentMethod", status, time.Since(start))
	}()

	_ = customerRef
	_ = providerMethodID
	return nil
}

// ListPaymentMethods always returns an empty list: the fake provider creates
// payment methods synchronously from tokens and has no provider-side method
// storage.
func (p *Provider) ListPaymentMethods(ctx context.Context, customerRef string) (methods []application.SavedMethod, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "ListPaymentMethods", status, time.Since(start))
	}()

	_ = customerRef
	return []application.SavedMethod{}, nil
}

// SetBindingState programs the PaymentMethodBinding result for a binding id.
// It is the fake counterpart of provider-side binding polling and is used by
// tests that drive the card-binding flow.
func (p *Provider) SetBindingState(bindingID string, state application.MethodBindingState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bindingStates[bindingID] = state
}

// PaymentMethodBinding returns the state programmed with SetBindingState.
// The fake provider has no provider-side card storage, so an unprogrammed
// binding id is an error.
func (p *Provider) PaymentMethodBinding(ctx context.Context, bindingID string) (res application.MethodBindingState, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "PaymentMethodBinding", status, time.Since(start))
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	state, ok := p.bindingStates[bindingID]
	if !ok {
		return application.MethodBindingState{}, errors.New("fake: binding state not programmed for binding id")
	}
	return state, nil
}

// RefundPayment refunds a finalized fake payment. For the fake provider every
// refund succeeds and reports the full amount back, so callers do not need to
// track it separately.
func (p *Provider) RefundPayment(ctx context.Context, req application.RefundRequest) (res application.RefundResult, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "RefundPayment", status, time.Since(start))
	}()

	if req.PaymentID == uuid.Nil {
		return application.RefundResult{}, errors.New("fake: payment id is required")
	}
	if req.ProviderPaymentID == "" {
		return application.RefundResult{}, errors.New("fake: provider payment id is required")
	}

	log := logger.WithCorrelation(ctx, p.log)

	refundedAmount := req.AmountKopecks
	if refundedAmount == 0 {
		p.mu.Lock()
		defer p.mu.Unlock()
		if amount, ok := p.confirmedAmounts[req.ProviderPaymentID]; ok {
			refundedAmount = amount
		} else if amount, ok := p.confirmedAmounts[req.PaymentID.String()]; ok {
			refundedAmount = amount
		}
	}

	log.InfoContext(ctx, "fake payment refunded",
		"provider_payment_id", req.ProviderPaymentID,
		"internal_payment_id", req.PaymentID.String(),
		"amount_kopecks", refundedAmount,
	)

	return application.RefundResult{
		ProviderPaymentID:     req.ProviderPaymentID,
		Status:                domain.PaymentStatusRefunded,
		RefundedAmountKopecks: refundedAmount,
	}, nil
}

// WebhookAck returns the fixed success response the fake provider expects
// HTTP handlers to send back after receiving a webhook.
func (p *Provider) WebhookAck() []byte {
	return []byte(`{"status":"ok"}`)
}

// ParseWebhook parses the fake provider webhook JSON payload.
func (p *Provider) ParseWebhook(_ context.Context, payload []byte) (application.WebhookEvent, error) {
	var raw struct {
		ProviderPaymentID string `json:"provider_payment_id"`
		InternalPaymentID string `json:"internal_payment_id"`
		Status            string `json:"status"`
		ErrorCode         string `json:"error_code,omitempty"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return application.WebhookEvent{}, fmt.Errorf("fake: invalid webhook payload: %w", err)
	}

	internalPaymentID, err := uuid.Parse(raw.InternalPaymentID)
	if err != nil {
		return application.WebhookEvent{}, fmt.Errorf("fake: invalid internal payment id: %w", err)
	}

	status := domain.PaymentStatus(raw.Status)
	if status != domain.PaymentStatusSucceeded && status != domain.PaymentStatusFailed {
		return application.WebhookEvent{}, fmt.Errorf("fake: unsupported webhook status %q", raw.Status)
	}

	var errorCode *string
	if status == domain.PaymentStatusFailed {
		code := raw.ErrorCode
		if code == "" {
			code = defaultErrorCode
		}
		errorCode = &code
	}

	return application.WebhookEvent{
		Payment: &application.PaymentNotification{
			ProviderPaymentID: raw.ProviderPaymentID,
			InternalPaymentID: internalPaymentID,
			Status:            status,
			ErrorCode:         errorCode,
		},
	}, nil
}

// ConfirmPayment completes a previously initialized fake payment as
// succeeded. It is used by the local fake confirmation HTTP handler and is
// not part of the provider port.
func (p *Provider) ConfirmPayment(ctx context.Context, internalPaymentID string) (res application.WebhookEvent, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "ConfirmPayment", status, time.Since(start))
	}()

	return p.confirm(internalPaymentID, false, nil)
}

// ConfirmPaymentFailed completes a previously initialized fake payment as
// failed.
func (p *Provider) ConfirmPaymentFailed(ctx context.Context, internalPaymentID string, errorCode *string) (res application.WebhookEvent, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "ConfirmPaymentFailed", status, time.Since(start))
	}()

	return p.confirm(internalPaymentID, true, errorCode)
}

func (p *Provider) confirm(internalPaymentID string, failed bool, errorCode *string) (application.WebhookEvent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()
	entry, ok := p.pending[internalPaymentID]
	if ok {
		delete(p.pending, internalPaymentID)
	}
	if !ok {
		return application.WebhookEvent{}, errors.New("fake: payment not found")
	}

	status := domain.PaymentStatusSucceeded
	if failed {
		status = domain.PaymentStatusFailed
		if errorCode == nil {
			defaultCode := defaultErrorCode
			errorCode = &defaultCode
		}
	} else {
		errorCode = nil
		p.confirmedAmounts[internalPaymentID] = entry.amountKopecks
	}

	event := application.WebhookEvent{
		Payment: &application.PaymentNotification{
			ProviderPaymentID: entry.event.Payment.ProviderPaymentID,
			InternalPaymentID: entry.event.Payment.InternalPaymentID,
			Status:            status,
			ErrorCode:         errorCode,
			AmountKopecks:     entry.amountKopecks,
		},
	}
	if !failed {
		method := entry.savedMethod
		event.Payment.SavedMethod = &method
	}
	return event, nil
}

// purgeLocked removes pending entries older than pendingTTL.
// p.mu must be held.
func (p *Provider) purgeLocked() {
	now := p.clock.Now().UTC()
	for id, entry := range p.pending {
		if now.Sub(entry.createdAt) > pendingTTL {
			delete(p.pending, id)
		}
	}
}
