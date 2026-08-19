package payment

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds low-cardinality billing provider metrics.
type Metrics struct {
	requests metric.Int64Counter
	latency  metric.Float64Histogram
	errors   metric.Int64Counter
}

// NewMetrics builds the payment-provider instruments from the global OpenTelemetry MeterProvider.
// When no provider is configured, the instruments are no-ops. RecordRequest is nil-safe.
func NewMetrics() (*Metrics, error) {
	meter := otel.GetMeterProvider().Meter("github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment")

	requests, err := meter.Int64Counter("payment.provider.requests",
		metric.WithDescription("Total provider requests"))
	if err != nil {
		return nil, err
	}

	latency, err := meter.Float64Histogram("payment.provider.latency",
		metric.WithDescription("Provider request latency in seconds"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}

	errors, err := meter.Int64Counter("payment.provider.errors",
		metric.WithDescription("Total provider request errors"))
	if err != nil {
		return nil, err
	}

	return &Metrics{
		requests: requests,
		latency:  latency,
		errors:   errors,
	}, nil
}

// Status label values of the RED convention shared by every provider adapter
// (see RecordRequest).
const (
	StatusOK    = "ok"
	StatusError = "error"
)

// RecordRequest records a provider operation attempt. Status should be one
// of StatusOK or StatusError.
func (m *Metrics) RecordRequest(ctx context.Context, provider, operation, status string, duration time.Duration) {
	if m == nil {
		return
	}
	attrs := attribute.NewSet(
		attribute.String("provider", provider),
		attribute.String("operation", operation),
		attribute.String("status", status),
	)
	m.requests.Add(ctx, 1, metric.WithAttributeSet(attrs))
	m.latency.Record(ctx, duration.Seconds(), metric.WithAttributeSet(attrs))
	if status == "error" {
		m.errors.Add(ctx, 1, metric.WithAttributeSet(attrs))
	}
}
