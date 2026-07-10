package tkassa

import (
	"context"
	"fmt"
	"net"
	"testing"
)

// TestMetricStatus pins the RED convention for the "status" metric label:
//   - "ok" for nil error and for any definitive provider response, including a
//     business decline surfaced as *ProviderError (and even when wrapped);
//   - "error" for genuine operational failures (transport/timeout/context,
//     request-build, non-2xx HTTP, unmarshal) — whether direct or wrapped.
func TestMetricStatus(t *testing.T) {
	// Mirror how post constructs a provider error-code response.
	providerErr := &ProviderError{
		Method:    "Charge",
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
			want: "ok",
		},
		{
			name: "business decline ProviderError is ok",
			err:  providerErr,
			want: "ok",
		},
		{
			name: "wrapped ProviderError is still ok",
			err:  fmt.Errorf("tkassa: add customer failed: %w", providerErr),
			want: "ok",
		},
		{
			name: "plain transport error is error",
			err:  net.ErrClosed,
			want: "error",
		},
		{
			name: "wrapped transport error is error",
			err:  transportErr,
			want: "error",
		},
		{
			name: "non-2xx HTTP error is error",
			err:  fmt.Errorf("tkassa: Charge returned HTTP 503: unavailable"),
			want: "error",
		},
		{
			name: "cancelled context is error",
			err:  context.Canceled,
			want: "error",
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
