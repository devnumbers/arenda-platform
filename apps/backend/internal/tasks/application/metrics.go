package application

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds the tick heartbeat instrument of the tasks module: the unix
// timestamp of the last successful materialization sweep — the heartbeat the
// "tick has not run in N hours" alert watches (the payments precedent,
// ticket #458; the threshold itself is an operational alerting setting, not
// code). When no meter provider is configured (local dev, tests) the
// instrument is a no-op and RecordTickSuccess is nil-safe.
type Metrics struct {
	lastSuccess metric.Int64Gauge
}

// NewMetrics builds the heartbeat instrument from the global OTel
// MeterProvider.
func NewMetrics() (*Metrics, error) {
	meter := otel.GetMeterProvider().Meter("github.com/nambers/arenda-planform/apps/backend/internal/tasks/application")

	lastSuccess, err := meter.Int64Gauge("tasks.tick.last_success",
		metric.WithDescription("Unix seconds of the last successful materialization tick sweep; "+
			"alert when it lags by more than the operational threshold"))
	if err != nil {
		return nil, err
	}

	return &Metrics{lastSuccess: lastSuccess}, nil
}

// RecordTickSuccess records the heartbeat of one fully successful sweep — a
// sweep with no zones included, which is a successful no-op run. Nil-safe.
func (m *Metrics) RecordTickSuccess(ctx context.Context, now time.Time) {
	if m == nil {
		return
	}
	m.lastSuccess.Record(ctx, now.Unix())
}
