package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
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

	log.Info("hello")

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
