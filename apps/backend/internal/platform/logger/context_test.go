package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
	"go.opentelemetry.io/otel/trace"
)

func TestWithCorrelation_AddsIDs(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := requestctx.WithRequestID(context.Background(), "req-1")
	ctx = requestctx.WithTraceID(ctx, "trace-1")

	log := WithCorrelation(ctx, base)
	if log == nil {
		t.Fatal("expected non-nil logger")
	}

	log.InfoContext(ctx, "hello")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to unmarshal log record: %v", err)
	}

	if got, want := record["request_id"], "req-1"; got != want {
		t.Errorf("request_id = %q, want %q", got, want)
	}
	if got, want := record["trace_id"], "trace-1"; got != want {
		t.Errorf("trace_id = %q, want %q", got, want)
	}
}

func TestWithCorrelation_NoIDs_ReturnsSameLogger(t *testing.T) {
	base := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	log := WithCorrelation(context.Background(), base)
	if log != base {
		t.Fatalf("expected same logger pointer, got different pointer")
	}
}

func TestWithCorrelation_PassesNil(t *testing.T) {
	if got := WithCorrelation(context.Background(), nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestWithCorrelation_PrefersOTelSpanTraceID(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))

	// A local fallback id is present but MUST be ignored while a valid span exists.
	ctx := requestctx.WithTraceID(t.Context(), "local-trace-fallback")

	traceID := trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	spanID := trace.SpanID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18}
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID})
	ctx = trace.ContextWithSpanContext(ctx, sc)

	log := WithCorrelation(ctx, base)
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
	log.InfoContext(ctx, "hello")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to unmarshal log record: %v", err)
	}

	if got, want := record["trace_id"], traceID.String(); got != want {
		t.Errorf("trace_id = %q, want %q (active otel span trace id)", got, want)
	}
	if got := record["trace_id"]; got == "local-trace-fallback" {
		t.Errorf("trace_id resolved to local fallback %q despite active otel span", got)
	}
}

func TestWithCorrelation_FallsBackToLocalTraceIDWithoutSpan(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := requestctx.WithTraceID(t.Context(), "local-trace-only")

	log := WithCorrelation(ctx, base)
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
	log.InfoContext(ctx, "hello")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to unmarshal log record: %v", err)
	}

	if got, want := record["trace_id"], "local-trace-only"; got != want {
		t.Errorf("trace_id = %q, want %q (local requestctx fallback)", got, want)
	}
}
