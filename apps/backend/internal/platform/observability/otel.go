// Package observability initializes the OpenTelemetry SDK (tracer and meter providers) with a combined Shutdown flush.
package observability

import (
	"context"
	"errors"
	"fmt"

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
}

// Config controls whether observability is enabled and how it exports.
type Config struct {
	ServiceName  string
	Enabled      bool
	OTLPEndpoint string
	TraceSampler float64
}

// NewSDK initializes tracer and meter providers when enabled. Callers must
// invoke Shutdown before exit.
func NewSDK(ctx context.Context, cfg Config) (*SDK, error) {
	if !cfg.Enabled {
		return &SDK{}, nil
	}

	res, err := sdkresource.New(ctx,
		// WithFromEnv comes first so the configured service.name wins over
		// OTEL_SERVICE_NAME while OTEL_RESOURCE_ATTRIBUTES still merge in.
		sdkresource.WithFromEnv(),
		sdkresource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("observability resource: %w", err)
	}

	sampler := cfg.TraceSampler
	if sampler < 0 {
		sampler = 0
	}
	if sampler > 1 {
		sampler = 1
	}

	traceOpts := []otlptracegrpc.Option{}
	if cfg.OTLPEndpoint != "" {
		traceOpts = append(traceOpts, otlptracegrpc.WithEndpointURL(cfg.OTLPEndpoint))
	}
	traceExp, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		return nil, fmt.Errorf("observability trace exporter: %w", err)
	}

	metricOpts := []otlpmetricgrpc.Option{}
	if cfg.OTLPEndpoint != "" {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithEndpointURL(cfg.OTLPEndpoint))
	}
	metricExp, err := otlpmetricgrpc.New(ctx, metricOpts...)
	if err != nil {
		// The trace exporter must not leak; its shutdown failure joins the
		// metric exporter error instead of being discarded.
		return nil, errors.Join(
			fmt.Errorf("observability metric exporter: %w", err),
			fmt.Errorf("shutdown trace exporter: %w", traceExp.Shutdown(ctx)),
		)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(sampler)),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return &SDK{
		tracerProvider: tp,
		meterProvider:  mp,
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
