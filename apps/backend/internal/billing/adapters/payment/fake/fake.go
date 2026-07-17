// Package fake implements a deterministic local payment provider for development and tests.
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

const (
	pendingTTL                  = 24 * time.Hour
	defaultErrorCode            = "fake_error"
	fakeFailTokenPrefix         = "fake_fail_"
	fakeProviderPaymentIDPrefix = "fake_"
)

type pendingEntry struct {
	payload       application.WebhookPayload
	savedToken    string
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
	addCardStates    map[string]application.CardBindingState
	metrics          *payment.Metrics
}

// Compile-time assertions that Provider satisfies the aggregate Provider port
// and each of its narrow capability interfaces.
var (
	_ application.Provider                = (*Provider)(nil)
	_ application.PaymentInitiator        = (*Provider)(nil)
	_ application.PaymentCharger          = (*Provider)(nil)
	_ application.PaymentCanceler         = (*Provider)(nil)
	_ application.PaymentStatusChecker    = (*Provider)(nil)
	_ application.WebhookParser           = (*Provider)(nil)
	_ application.CardManager             = (*Provider)(nil)
	_ application.CardLister              = (*Provider)(nil)
	_ application.CardBindingStateChecker = (*Provider)(nil)
	_ application.WebhookResponder        = (*Provider)(nil)
)

// Name returns the provider identity used by the application layer.
func (p *Provider) Name() domain.PaymentProvider {
	return domain.ProviderFake
}

// NewProvider creates a fake provider for local development and Bruno tests.
func NewProvider(baseURL string, log *slog.Logger, clk clock.Clock, metrics *payment.Metrics) *Provider {
	return &Provider{
		baseURL:          strings.TrimRight(baseURL, "/"),
		log:              log,
		clock:            clk,
		pending:          make(map[string]pendingEntry),
		confirmedAmounts: make(map[string]int64),
		addCardStates:    make(map[string]application.CardBindingState),
		metrics:          metrics,
	}
}

// Init creates a pending payment and returns a confirmation URL.
// Calling Init twice with the same internal payment id is idempotent: the
// existing provider payment id and confirmation URL are returned.
func (p *Provider) Init(ctx context.Context, req application.InitRequest) (res application.InitResult, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "Init", status, time.Since(start))
	}()

	if req.PaymentID == uuid.Nil {
		return application.InitResult{}, errors.New("fake: payment id is required")
	}
	if req.AmountKopecks <= 0 {
		return application.InitResult{}, errors.New("fake: amount must be positive")
	}

	log := logger.WithCorrelation(ctx, p.log)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()

	if entry, ok := p.pending[req.PaymentID.String()]; ok {
		log.InfoContext(ctx, "fake payment init is idempotent",
			"provider_payment_id", entry.payload.ProviderPaymentID,
			"internal_payment_id", req.PaymentID.String(),
		)
		return application.InitResult{
			ProviderPaymentID: entry.payload.ProviderPaymentID,
			PaymentURL:        p.confirmURL(entry.payload.InternalPaymentID),
			SavedToken:        entry.savedToken,
			Status:            domain.PaymentStatusPending,
		}, nil
	}

	providerPaymentID := fakeProviderPaymentIDPrefix + uuid.Must(uuid.NewV7()).String()
	savedToken := "fake_token_" + uuid.Must(uuid.NewV7()).String()

	p.pending[req.PaymentID.String()] = pendingEntry{
		payload: application.WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: req.PaymentID,
			Status:            domain.PaymentStatusPending,
		},
		savedToken:    savedToken,
		amountKopecks: req.AmountKopecks,
		createdAt:     p.clock.Now().UTC(),
	}

	log.InfoContext(ctx, "fake payment initialized",
		"provider_payment_id", providerPaymentID,
		"internal_payment_id", req.PaymentID.String(),
		"user_id", req.UserID.String(),
		"amount_kopecks", req.AmountKopecks,
		"period", req.Period,
		"description", req.Description,
	)

	return application.InitResult{
		ProviderPaymentID: providerPaymentID,
		PaymentURL:        p.confirmURL(req.PaymentID),
		SavedToken:        savedToken,
		Status:            domain.PaymentStatusPending,
	}, nil
}

func (p *Provider) confirmURL(internalPaymentID uuid.UUID) string {
	return fmt.Sprintf("%s/internal/fake-subscription-payment/%s/confirm", p.baseURL, internalPaymentID.String())
}

// PaymentURL returns the confirmation URL for a previously initialized fake payment.
func (p *Provider) PaymentURL(ctx context.Context, paymentID uuid.UUID) (string, error) {
	_ = ctx
	return p.confirmURL(paymentID), nil
}

// Status returns the provider-side status of a payment. If the payment is not
// found in the pending map it is assumed to have been completed and succeeded.
func (p *Provider) Status(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (res application.PaymentStatusResult, err error) {
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
		return application.PaymentStatusResult{Status: entry.payload.Status}, nil
	}
	return application.PaymentStatusResult{Status: domain.PaymentStatusSucceeded}, nil
}

// Charge performs a recurrent charge using a saved token.
func (p *Provider) Charge(ctx context.Context, req application.ChargeRequest) (res application.ChargeResult, err error) {
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

	if strings.HasPrefix(req.Token, fakeFailTokenPrefix) {
		log.InfoContext(ctx, "fake charge failed",
			"provider_payment_id", providerPaymentID,
			"internal_payment_id", req.PaymentID.String(),
		)
		return application.ChargeResult{
			ProviderPaymentID: providerPaymentID,
			Status:            domain.PaymentStatusFailed,
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

// InitAddCard is not supported by the fake provider.
func (p *Provider) InitAddCard(ctx context.Context, req application.InitAddCardRequest) (res application.InitAddCardResult, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "InitAddCard", status, time.Since(start))
	}()

	_ = req
	return application.InitAddCardResult{}, errors.New("fake: add card flow is not supported")
}

// RemoveCard is a no-op for the fake provider.
func (p *Provider) RemoveCard(ctx context.Context, customerKey, cardID string) (err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "RemoveCard", status, time.Since(start))
	}()

	_ = customerKey
	_ = cardID
	return nil
}

// GetCardList always returns an empty list: the fake provider creates payment
// methods synchronously from tokens and has no provider-side card storage.
func (p *Provider) GetCardList(ctx context.Context, customerKey string) (cards []application.ProviderCard, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "GetCardList", status, time.Since(start))
	}()

	_ = customerKey
	return []application.ProviderCard{}, nil
}

// SetAddCardState programs the GetAddCardState result for a request key. It is
// the fake counterpart of the T-Kassa add-card state polling and is used by
// tests that drive the card-binding flow.
func (p *Provider) SetAddCardState(requestKey string, state application.CardBindingState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.addCardStates[requestKey] = state
}

// GetAddCardState returns the state programmed with SetAddCardState. The fake
// provider has no provider-side card storage, so an unprogrammed request key
// is an error.
func (p *Provider) GetAddCardState(ctx context.Context, requestKey string) (res application.CardBindingState, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "GetAddCardState", status, time.Since(start))
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	state, ok := p.addCardStates[requestKey]
	if !ok {
		return application.CardBindingState{}, errors.New("fake: add card state not programmed for request key")
	}
	return state, nil
}

// Cancel refunds a finalized fake payment. For the fake provider we treat every
// cancel as successful. For full refunds the original payment amount is reported
// back so callers do not need to track it separately.
func (p *Provider) Cancel(ctx context.Context, req application.CancelRequest) (res application.CancelResult, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "Cancel", status, time.Since(start))
	}()

	if req.PaymentID == uuid.Nil {
		return application.CancelResult{}, errors.New("fake: payment id is required")
	}
	if req.ProviderPaymentID == "" {
		return application.CancelResult{}, errors.New("fake: provider payment id is required")
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

	// Partial refunds are no longer initiated by the system; the fake provider
	// always reports a full refund.
	status := domain.PaymentStatusRefunded

	log.InfoContext(ctx, "fake payment cancelled",
		"provider_payment_id", req.ProviderPaymentID,
		"internal_payment_id", req.PaymentID.String(),
		"amount_kopecks", refundedAmount,
	)

	return application.CancelResult{
		ProviderPaymentID:     req.ProviderPaymentID,
		Status:                status,
		RefundedAmountKopecks: refundedAmount,
	}, nil
}

// WebhookResponse returns the fixed success response the fake provider expects
// HTTP handlers to send back after receiving a webhook.
func (p *Provider) WebhookResponse() []byte {
	return []byte(`{"status":"ok"}`)
}

// ParseWebhook parses the fake provider webhook JSON payload.
func (p *Provider) ParseWebhook(_ context.Context, payload []byte) (application.WebhookPayload, error) {
	var raw struct {
		ProviderPaymentID string `json:"provider_payment_id"`
		InternalPaymentID string `json:"internal_payment_id"`
		Status            string `json:"status"`
		ErrorCode         string `json:"error_code,omitempty"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return application.WebhookPayload{}, fmt.Errorf("fake: invalid webhook payload: %w", err)
	}

	internalPaymentID, err := uuid.Parse(raw.InternalPaymentID)
	if err != nil {
		return application.WebhookPayload{}, fmt.Errorf("fake: invalid internal payment id: %w", err)
	}

	status := domain.PaymentStatus(raw.Status)
	if status != domain.PaymentStatusSucceeded && status != domain.PaymentStatusFailed {
		return application.WebhookPayload{}, fmt.Errorf("fake: unsupported webhook status %q", raw.Status)
	}

	var errorCode *string
	if status == domain.PaymentStatusFailed {
		code := raw.ErrorCode
		if code == "" {
			code = defaultErrorCode
		}
		errorCode = &code
	}

	return application.WebhookPayload{
		ProviderPaymentID: raw.ProviderPaymentID,
		InternalPaymentID: internalPaymentID,
		Status:            status,
		ErrorCode:         errorCode,
	}, nil
}

// ConfirmPayment completes a previously initialized fake payment as succeeded.
// It is used by the local fake confirmation HTTP handler and is not part of the
// Provider interface.
func (p *Provider) ConfirmPayment(ctx context.Context, internalPaymentID string) (res application.WebhookPayload, err error) {
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

// ConfirmPaymentFailed completes a previously initialized fake payment as failed.
func (p *Provider) ConfirmPaymentFailed(ctx context.Context, internalPaymentID string, errorCode *string) (res application.WebhookPayload, err error) {
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

func (p *Provider) confirm(internalPaymentID string, failed bool, errorCode *string) (application.WebhookPayload, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()
	entry, ok := p.pending[internalPaymentID]
	if ok {
		delete(p.pending, internalPaymentID)
	}
	if !ok {
		return application.WebhookPayload{}, errors.New("fake: payment not found")
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

	return application.WebhookPayload{
		ProviderPaymentID: entry.payload.ProviderPaymentID,
		InternalPaymentID: entry.payload.InternalPaymentID,
		Status:            status,
		ErrorCode:         errorCode,
	}, nil
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
