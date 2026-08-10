package scheduler

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
	pushOutcomeSent        = "sent"
	pushOutcomeGone        = "gone"
	pushOutcomeRateLimited = "rate_limited"
	pushOutcomeFailed      = "failed"
)

// PushMetrics holds the OpenTelemetry instruments for Web Push delivery. When
// no meter provider is configured (local dev, tests) the instruments are no-ops
// and RecordDispatch is nil-safe. The shape mirrors payment.Metrics — one
// counter with an "outcome" label is the idiomatic OTel way to expose several
// mutually-exclusive result buckets.
type PushMetrics struct {
	dispatched metric.Int64Counter
}

// NewPushMetrics builds the push-delivery instruments from the global OTel
// MeterProvider.
func NewPushMetrics() (*PushMetrics, error) {
	meter := otel.GetMeterProvider().Meter("github.com/nambers/arenda-planform/apps/backend/internal/platform/scheduler")

	dispatched, err := meter.Int64Counter("notifications.push.dispatched",
		metric.WithDescription("Web Push messages dispatched per outcome (sent, gone, rate_limited, failed)"))
	if err != nil {
		return nil, err
	}

	return &PushMetrics{dispatched: dispatched}, nil
}

// RecordDispatch records one push delivery attempt with its outcome. outcome
// must be one of the pushOutcome* constants. It is nil-safe so the worker can
// call it unconditionally even when metrics are disabled.
func (m *PushMetrics) RecordDispatch(ctx context.Context, outcome string) {
	if m == nil {
		return
	}
	attrs := attribute.NewSet(attribute.String("outcome", outcome))
	m.dispatched.Add(ctx, 1, metric.WithAttributeSet(attrs))
}
