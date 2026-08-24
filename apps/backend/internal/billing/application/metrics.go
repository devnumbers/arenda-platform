package application

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Stuck-payment kinds used as the low-cardinality "kind" attribute on the
// stuck gauge. Keeping them in one place guards against typos at the
// recording site.
const (
	stuckKindPending   = "pending"
	stuckKindRefunding = "refunding"
)

// Metrics holds the worker-hygiene instruments of the billing module (ticket
// #433): gauges over payments and refunds "in flight" whose outcome the
// reconciliation watchdog has not resolved — the counts the stale selections
// of the phase scheduler compute. When no meter provider is configured
// (local dev, tests) the instruments are no-ops and every Record method is
// nil-safe. The shape mirrors the module's provider metrics (adapters/payment)
// and the notifications push metrics.
type Metrics struct {
	stuck metric.Int64Gauge
}

// NewMetrics builds the hygiene instruments from the global OTel
// MeterProvider.
func NewMetrics() (*Metrics, error) {
	meter := otel.GetMeterProvider().Meter("github.com/nambers/arenda-planform/apps/backend/internal/billing/application")

	stuck, err := meter.Int64Gauge("billing.payments.stuck",
		metric.WithDescription("Unresolved payments and refunds past the reconciliation staleness, by kind"))
	if err != nil {
		return nil, err
	}

	return &Metrics{stuck: stuck}, nil
}

// RecordStuck records the current count of unresolved payments of one kind.
// The kind must be one of the stuckKind* constants. Nil-safe.
func (m *Metrics) RecordStuck(ctx context.Context, kind string, count int64) {
	if m == nil {
		return
	}
	attrs := attribute.NewSet(attribute.String("kind", kind))
	m.stuck.Record(ctx, count, metric.WithAttributeSet(attrs))
}
