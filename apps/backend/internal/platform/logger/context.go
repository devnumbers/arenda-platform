package logger

import (
	"context"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx"
	"go.opentelemetry.io/otel/trace"
)

// WithCorrelation returns a logger that includes request_id and trace_id from
// ctx when they are present. When ctx carries a valid OpenTelemetry span
// context, the real OTel trace_id and span_id are used so logs correlate with
// traces; otherwise it falls back to the lightweight local trace ID.
// It is safe to call with a nil logger.
func WithCorrelation(ctx context.Context, log *slog.Logger) *slog.Logger {
	if log == nil {
		return nil
	}
	attrs := make([]any, 0, 3)
	if rid := requestctx.RequestIDFromContext(ctx); rid != "" {
		attrs = append(attrs, slog.String("request_id", rid))
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		attrs = append(attrs,
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	} else if tid := requestctx.TraceIDFromContext(ctx); tid != "" {
		attrs = append(attrs, slog.String("trace_id", tid))
	}
	if len(attrs) == 0 {
		return log
	}
	return log.With(attrs...)
}
