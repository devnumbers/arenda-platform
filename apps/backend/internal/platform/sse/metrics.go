package sse

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds the OpenTelemetry instruments for the shared stream's load
// picture (карта #734, #742): sse.connections tracks how many streams are
// open right now (the load boundary the deployment watches), sse.frames.dropped
// counts the slow-consumer dismissals — a sustained non-zero drop rate means
// clients cannot keep up or stay connected. The shape mirrors the delivery
// metrics (notificationsjob.EmailMetrics, webpush.Metrics): instruments from
// the global meter provider, nil-safe recording so local dev and tests need
// no provider.
type Metrics struct {
	connections metric.Int64UpDownCounter
	dropped     metric.Int64Counter
}

// NewMetrics builds the stream instruments from the global OTel MeterProvider.
func NewMetrics() (*Metrics, error) {
	meter := otel.GetMeterProvider().Meter("github.com/nambers/arenda-planform/apps/backend/internal/platform/sse")

	connections, err := meter.Int64UpDownCounter("sse.connections",
		metric.WithDescription("Open SSE stream connections"))
	if err != nil {
		return nil, err
	}
	dropped, err := meter.Int64Counter("sse.frames.dropped",
		metric.WithDescription("Connections dismissed for falling behind (slow consumer)"))
	if err != nil {
		return nil, err
	}

	return &Metrics{connections: connections, dropped: dropped}, nil
}

// connOpened records one open connection.
func (m *Metrics) connOpened(ctx context.Context) {
	if m == nil {
		return
	}
	m.connections.Add(ctx, 1)
}

// connClosed records one closed connection (unsubscribed, dropped or hub shutdown).
func (m *Metrics) connClosed(ctx context.Context) {
	if m == nil {
		return
	}
	m.connections.Add(ctx, -1)
}

// frameDropped records one slow-consumer dismissal.
func (m *Metrics) frameDropped(ctx context.Context) {
	if m == nil {
		return
	}
	m.dropped.Add(ctx, 1)
}
