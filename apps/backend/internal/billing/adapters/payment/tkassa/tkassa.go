// Package tkassa is a skeleton adapter for the T-Kassa payment provider.
package tkassa

import (
	"context"
	"errors"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// ErrNotImplemented is returned by all T-Kassa provider methods until they are implemented.
var ErrNotImplemented = errors.New("tkassa: not implemented")

// Provider is a placeholder T-Kassa payment adapter.
type Provider struct {
	terminalKey string
	password    string
	log         *slog.Logger
}

// NewProvider creates a T-Kassa provider instance.
func NewProvider(terminalKey, password string, log *slog.Logger) *Provider {
	return &Provider{
		terminalKey: terminalKey,
		password:    password,
		log:         log,
	}
}

// Name returns the provider identity used by the application layer.
func (p *Provider) Name() domain.PaymentProvider {
	return domain.ProviderTkassa
}

// Init starts a new payment through T-Kassa.
func (p *Provider) Init(ctx context.Context, req application.InitRequest) (application.InitResult, error) {
	_ = ctx
	_ = req
	return application.InitResult{}, ErrNotImplemented
}

// Charge performs a recurrent charge through T-Kassa.
func (p *Provider) Charge(ctx context.Context, req application.ChargeRequest) (application.ChargeResult, error) {
	_ = ctx
	_ = req
	return application.ChargeResult{}, ErrNotImplemented
}

// ParseWebhook parses a T-Kassa webhook payload.
func (p *Provider) ParseWebhook(ctx context.Context, payload []byte) (application.WebhookPayload, error) {
	_ = ctx
	_ = payload
	return application.WebhookPayload{}, ErrNotImplemented
}
