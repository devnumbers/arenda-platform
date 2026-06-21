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
	pendingTTL                  = 24 * time.Hour
	defaultErrorCode            = "fake_error"
	fakeFailTokenPrefix         = "fake_fail_"
	fakeProviderPaymentIDPrefix = "fake_"
)

type pendingEntry struct {
	payload    application.WebhookPayload
	savedToken string
	createdAt  time.Time
}

// Provider is a fake payment processor that keeps pending payments in memory.
type Provider struct {
	baseURL string
	log     *slog.Logger
	clock   clock.Clock
	mu      sync.Mutex
	pending map[string]pendingEntry
}

// Name returns the provider identity used by the application layer.
func (p *Provider) Name() domain.PaymentProvider {
	return domain.ProviderFake
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
// Calling Init twice with the same internal payment id is idempotent: the
// existing provider payment id and confirmation URL are returned.
func (p *Provider) Init(ctx context.Context, req application.InitRequest) (application.InitResult, error) {
	if req.PaymentID == uuid.Nil {
		return application.InitResult{}, errors.New("fake: payment id is required")
	}
	if req.AmountKopecks <= 0 {
		return application.InitResult{}, errors.New("fake: amount must be positive")
	}

	p.mu.Lock()
	p.purgeLocked()

	if entry, ok := p.pending[req.PaymentID.String()]; ok {
		p.mu.Unlock()
		p.log.InfoContext(ctx, "fake payment init is idempotent",
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

	providerPaymentID := fakeProviderPaymentIDPrefix + uuid.NewString()
	savedToken := "fake_token_" + uuid.NewString()

	p.pending[req.PaymentID.String()] = pendingEntry{
		payload: application.WebhookPayload{
			ProviderPaymentID: providerPaymentID,
			InternalPaymentID: req.PaymentID,
			Status:            domain.PaymentStatusPending,
		},
		savedToken: savedToken,
		createdAt:  p.clock.Now().UTC(),
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
func (p *Provider) Status(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (domain.PaymentStatus, error) {
	_ = ctx
	_ = providerPaymentID
	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()

	if entry, ok := p.pending[paymentID.String()]; ok {
		return entry.payload.Status, nil
	}
	return domain.PaymentStatusSucceeded, nil
}

// Charge performs a recurrent charge using a saved token.
func (p *Provider) Charge(ctx context.Context, req application.ChargeRequest) (application.ChargeResult, error) {
	if req.PaymentID == uuid.Nil {
		return application.ChargeResult{}, errors.New("fake: payment id is required")
	}
	if req.AmountKopecks <= 0 {
		return application.ChargeResult{}, errors.New("fake: amount must be positive")
	}

	providerPaymentID := fakeProviderPaymentIDPrefix + uuid.NewString()

	if strings.HasPrefix(req.Token, fakeFailTokenPrefix) {
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

// ConfirmPayment completes a previously initialized fake payment as succeeded.
// It is used by the local fake confirmation HTTP handler and is not part of the
// Provider interface.
func (p *Provider) ConfirmPayment(ctx context.Context, internalPaymentID string) (application.WebhookPayload, error) {
	_ = ctx
	return p.confirm(internalPaymentID, false, nil)
}

// ConfirmPaymentFailed completes a previously initialized fake payment as failed.
func (p *Provider) ConfirmPaymentFailed(ctx context.Context, internalPaymentID string, errorCode *string) (application.WebhookPayload, error) {
	_ = ctx
	return p.confirm(internalPaymentID, true, errorCode)
}

func (p *Provider) confirm(internalPaymentID string, failed bool, errorCode *string) (application.WebhookPayload, error) {
	p.mu.Lock()
	p.purgeLocked()
	entry, ok := p.pending[internalPaymentID]
	if ok {
		delete(p.pending, internalPaymentID)
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
