package tkassa

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
)

// TestMetricStatus pins the RED convention for the "status" metric label:
//   - "ok" for nil error and for any definitive provider response, including a
//     business decline surfaced as *ProviderError (and even when wrapped), a
//     blocked charge, or a payment-not-found outcome;
//   - "error" for genuine operational failures (transport/timeout/context,
//     request-build, non-2xx HTTP, unmarshal) and for broken-integration
//     signals (auth-rejected 204/205, invalid-operation 9/12/1125/1126) —
//     whether direct or wrapped.
func TestMetricStatus(t *testing.T) {
	// Mirror how post constructs a provider error-code response.
	providerErr := &ProviderError{
		Method:    methodCharge,
		ErrorCode: "105",
		Message:   "Charge rejected",
		Details:   "insufficient funds",
	}
	// Mirror how post wraps a transport failure.
	transportErr := fmt.Errorf("tkassa: Charge request failed: %w", net.ErrClosed)

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil error is ok",
			err:  nil,
			want: payment.StatusOK,
		},
		{
			name: "business decline ProviderError is ok",
			err:  providerErr,
			want: payment.StatusOK,
		},
		{
			name: "wrapped ProviderError is still ok",
			err:  fmt.Errorf("tkassa: add customer failed: %w", providerErr),
			want: payment.StatusOK,
		},
		{
			name: "method-not-found sentinel is ok",
			err:  application.ErrProviderMethodNotFound,
			want: payment.StatusOK,
		},
		{
			name: "charge-blocked sentinel is ok",
			err:  fmt.Errorf("%w: %w", application.ErrProviderChargeBlocked, providerErr),
			want: payment.StatusOK,
		},
		{
			name: "payment-not-found sentinel is ok",
			err:  fmt.Errorf("%w: %w", application.ErrProviderPaymentNotFound, providerErr),
			want: payment.StatusOK,
		},
		{
			name: "insufficient-funds sentinel is ok (business outcome)",
			err:  fmt.Errorf("%w: %w", application.ErrProviderInsufficientFunds, providerErr),
			want: payment.StatusOK,
		},
		{
			name: "saved-method-expired sentinel is ok (business outcome)",
			err:  fmt.Errorf("%w: %w", application.ErrProviderSavedMethodExpired, providerErr),
			want: payment.StatusOK,
		},
		{
			name: "duplicate-operation sentinel is ok (business outcome)",
			err:  fmt.Errorf("%w: %w", application.ErrProviderDuplicateOperation, providerErr),
			want: payment.StatusOK,
		},
		{
			name: "auth-rejected sentinel is a broken-integration error",
			err:  fmt.Errorf("%w: %w", application.ErrProviderAuthRejected, providerErr),
			want: payment.StatusError,
		},
		{
			name: "invalid-operation sentinel is a broken-integration error",
			err:  fmt.Errorf("%w: %w", application.ErrProviderInvalidOperation, providerErr),
			want: payment.StatusError,
		},
		{
			name: "plain transport error is error",
			err:  net.ErrClosed,
			want: payment.StatusError,
		},
		{
			name: "wrapped transport error is error",
			err:  transportErr,
			want: payment.StatusError,
		},
		{
			name: "canceled context is error",
			err:  context.Canceled,
			want: payment.StatusError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := metricStatus(tt.err); got != tt.want {
				t.Fatalf("metricStatus(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}
