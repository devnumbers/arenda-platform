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
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

const (
	pendingTTL       = 24 * time.Hour
	defaultErrorCode = "fake_error"
)

type pendingEntry struct {
	payload   application.WebhookPayload
	createdAt time.Time
}

// Provider is a fake payment processor that keeps pending payments in memory.
type Provider struct {
	baseURL string
	log     *slog.Logger
	clock   clock.Clock
	mu      sync.Mutex
	pending map[string]pendingEntry
}

// NewProvider creates a fake provider for local development and Bruno tests.
func NewProvider(baseURL string, log *slog.Logger, clk clock.Clock) *Provider {
	return &Provider{
		baseURL: strings.TrimRight(baseURL, "/"),
		log:     log,
		clock:   clk,
		pending: make(map[string]pendingEntry),
	}
}

// Init creates a pending payment and returns a confirmation URL.
func (p *Provider) Init(ctx context.Context, req application.InitRequest) (application.InitResult, error) {
	if req.PaymentID == uuid.Nil {
		return application.InitResult{}, errors.New("fake: payment id is required")
	}
	if req.AmountKopecks <= 0 {
		return application.InitResult{}, errors.New("fake: amount must be positive")
	}

	providerPaymentID := "fake_" + uuid.NewString()
	savedToken := "fake_token_" + uuid.NewString()

	p.mu.Lock()
	p.purgeLocked()
	p.pending[providerPaymentID] = pendingEntry{
		payload: application.WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: req.PaymentID,
			Status:            domain.PaymentStatusPending,
		},
		createdAt: p.clock.Now().UTC(),
	}
	p.mu.Unlock()

	p.log.InfoContext(ctx, "fake payment initialized",
		"provider_payment_id", providerPaymentID,
		"internal_payment_id", req.PaymentID.String(),
		"user_id", req.UserID.String(),
		"amount_kopecks", req.AmountKopecks,
		"period", req.Period,
		"description", req.Description,
	)

	return application.InitResult{
		ProviderPaymentID: providerPaymentID,
		PaymentURL:        fmt.Sprintf("%s/internal/fake-subscription-payment/%s/confirm", p.baseURL, providerPaymentID),
		SavedToken:        savedToken,
		Status:            domain.PaymentStatusPending,
	}, nil
}

// Charge performs a recurrent charge using a saved token.
func (p *Provider) Charge(ctx context.Context, req application.ChargeRequest) (application.ChargeResult, error) {
	if req.PaymentID == uuid.Nil {
		return application.ChargeResult{}, errors.New("fake: payment id is required")
	}
	if req.AmountKopecks <= 0 {
		return application.ChargeResult{}, errors.New("fake: amount must be positive")
	}

	providerPaymentID := "fake_" + uuid.NewString()

	if strings.HasPrefix(req.Token, "fake_fail_") {
		p.log.InfoContext(ctx, "fake charge failed",
			"provider_payment_id", providerPaymentID,
			"internal_payment_id", req.PaymentID.String(),
		)
		return application.ChargeResult{
			ProviderPaymentID: providerPaymentID,
			Status:            domain.PaymentStatusFailed,
		}, nil
	}

	p.log.InfoContext(ctx, "fake charge succeeded",
		"provider_payment_id", providerPaymentID,
		"internal_payment_id", req.PaymentID.String(),
	)
	return application.ChargeResult{
		ProviderPaymentID: providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
	}, nil
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

// ConfirmPayment completes a previously initialized fake payment.
// It is used by the local fake confirmation HTTP handler and is not part of the Provider interface.
// TODO(Task 6): the PaymentURL returned by Init is handled by POST /internal/fake-subscription-payment/{providerPaymentID}/confirm.
func (p *Provider) ConfirmPayment(providerPaymentID string, failed bool, errorCode *string) (application.WebhookPayload, error) {
	p.mu.Lock()
	p.purgeLocked()
	entry, ok := p.pending[providerPaymentID]
	if ok {
		delete(p.pending, providerPaymentID)
	}
	p.mu.Unlock()
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
	}

	return application.WebhookPayload{
		ProviderPaymentID: providerPaymentID,
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
