package webpush

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Push outcome values used as the low-cardinality "outcome" attribute on the
// push-dispatch counter. Keeping them in one place guards against typos at the
// recording sites.
const (
	outcomeSent        = "sent"
	outcomeGone        = "gone"
	outcomeRateLimited = "rate_limited"
	outcomeFailed      = "failed"
)

// Metrics holds the OpenTelemetry instruments for Web Push delivery. When no
// meter provider is configured (local dev, tests) the instruments are no-ops
// and RecordDispatch is nil-safe. The shape mirrors payment.Metrics — one
// counter with an "outcome" label is the idiomatic OTel way to expose several
// mutually-exclusive result buckets.
type Metrics struct {
	dispatched metric.Int64Counter
}

// NewMetrics builds the push-delivery instruments from the global OTel
// MeterProvider.
func NewMetrics() (*Metrics, error) {
	meter := otel.GetMeterProvider().Meter("github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/webpush")

	dispatched, err := meter.Int64Counter("notifications.push.dispatched",
		metric.WithDescription("Web Push messages dispatched per outcome (sent, gone, rate_limited, failed)"))
	if err != nil {
		return nil, err
	}

	return &Metrics{dispatched: dispatched}, nil
}

// RecordDispatch records one push delivery attempt with its outcome. outcome
// must be one of the outcome* constants. It is nil-safe so the Sender can call
// it unconditionally even when metrics are disabled.
func (m *Metrics) RecordDispatch(ctx context.Context, outcome string) {
	if m == nil {
		return
	}
	attrs := attribute.NewSet(attribute.String("outcome", outcome))
	m.dispatched.Add(ctx, 1, metric.WithAttributeSet(attrs))
}
