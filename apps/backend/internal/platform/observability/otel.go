package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// SDK holds initialized OpenTelemetry providers.
type SDK struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	logger         *slog.Logger
}

// Config controls whether observability is enabled and how it exports.
type Config struct {
	ServiceName   string
	Enabled       bool
	OTLPEndpoint  string
	TraceSampler  float64
}

// NewSDK initializes tracer and meter providers when enabled. Callers must
// invoke Shutdown before exit.
func NewSDK(ctx context.Context, cfg Config, logger *slog.Logger) (*SDK, error) {
	if !cfg.Enabled {
		return &SDK{logger: logger}, nil
	}
	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("observability resource: %w", err)
	}

	traceExp, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("observability trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.TraceSampler)),
	)
	otel.SetTracerProvider(tp)

	metricExp, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("observability metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	otel.SetTextMapPropagator(propagation.TraceContext{})

	return &SDK{
		tracerProvider: tp,
		meterProvider:  mp,
		logger:         logger,
	}, nil
}

// Shutdown flushes and stops providers. Timeout is applied to the shutdown
// context.
func (s *SDK) Shutdown(ctx context.Context) error {
	var err error
	if s.tracerProvider != nil {
		err = errors.Join(err, s.tracerProvider.Shutdown(ctx))
	}
	if s.meterProvider != nil {
		err = errors.Join(err, s.meterProvider.Shutdown(ctx))
	}
	return err
}
