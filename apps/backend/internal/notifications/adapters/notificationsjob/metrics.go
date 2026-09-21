package notificationsjob

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Email outcome values used as the low-cardinality "outcome" attribute on
// the email-dispatch counter. Keeping them in one place guards against typos
// at the recording sites.
const (
	outcomeSent        = "sent"
	outcomeSkipped     = "skipped"
	outcomeRateLimited = "rate_limited"
	outcomeFailed      = "failed"
	outcomeCancelled   = "cancelled"
)

// EmailMetrics holds the OpenTelemetry instruments for email delivery. The
// shape mirrors webpush.Metrics — one counter with an "outcome" label is the
// idiomatic OTel way to expose several mutually-exclusive result buckets.
// When no meter provider is configured (local dev, tests) the instruments
// are no-ops and RecordDispatch is nil-safe.
type EmailMetrics struct {
	dispatched metric.Int64Counter
}

// NewEmailMetrics builds the email-delivery instruments from the global OTel
// MeterProvider.
func NewEmailMetrics() (*EmailMetrics, error) {
	meter := otel.GetMeterProvider().Meter("github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters/notificationsjob")

	dispatched, err := meter.Int64Counter("notifications.email.dispatched",
		metric.WithDescription("Notification emails dispatched per outcome (sent, skipped, rate_limited, failed, cancelled)"))
	if err != nil {
		return nil, err
	}

	return &EmailMetrics{dispatched: dispatched}, nil
}

// RecordDispatch records one email delivery attempt with its outcome. The
// outcome must be one of the outcome* constants. It is nil-safe so the
// worker can call it unconditionally even when metrics are disabled.
func (m *EmailMetrics) RecordDispatch(ctx context.Context, outcome string) {
	if m == nil {
		return
	}
	attrs := attribute.NewSet(attribute.String("outcome", outcome))
	m.dispatched.Add(ctx, 1, metric.WithAttributeSet(attrs))
}
