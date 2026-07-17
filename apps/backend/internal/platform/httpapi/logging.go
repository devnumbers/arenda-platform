package httpapi

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	platformlogger "github.com/nambers/arenda-planform/apps/backend/internal/platform/logger"
	"go.opentelemetry.io/otel/trace"
)

// loggerCtxKey is the context key used to store the request-scoped logger.
type loggerCtxKey struct{}

// withLogger stores a logger in the context so that downstream code can retrieve
// it with loggerFromContext.
func withLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, logger)
}

// loggerFromContext returns the logger stored in the context, or the default
// logger if none is present.
func loggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// problemTitleSetter lets handlers attach a problem title to the response writer
// so the request logger can include an error code without logging the body.
type problemTitleSetter interface {
	SetProblemTitle(string)
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status       int
	wroteHeader  bool
	problemTitle string
}

func (w *loggingResponseWriter) SetProblemTitle(title string) {
	w.problemTitle = title
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *loggingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("response writer does not support hijacking")
}

type RequestLoggerOptions struct {
	LogSuccessfulRequests bool
	SlowRequestThreshold  time.Duration
}

// RequestLogger logs the outcome of every HTTP request.
// It captures method, route, status, duration, request ID and problem error code.
// It never logs bodies, cookies, or phone numbers.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return RequestLoggerWithOptions(logger, RequestLoggerOptions{LogSuccessfulRequests: true})
}

// RequestLoggerWithOptions logs request outcomes with optional suppression of
// successful fast requests for local performance runs.
func RequestLoggerWithOptions(logger *slog.Logger, opts RequestLoggerOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := r.Context()
			// WithCorrelation attaches request_id plus the OTel trace_id/span_id
			// when a span context is present (otelhttp runs before this
			// middleware), falling back to the lightweight local trace ID.
			requestLogger := platformlogger.WithCorrelation(ctx, logger)
			ctx = withLogger(ctx, requestLogger)
			r = r.WithContext(ctx)

			lw := &loggingResponseWriter{ResponseWriter: w}
			next.ServeHTTP(lw, r)

			duration := time.Since(start)
			route := chi.RouteContext(ctx).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			// Rename the server span now that the route is known so span names
			// stay low-cardinality: the route pattern for matched requests
			// (e.g. "GET /properties/{id}") and "unmatched" for 404s from
			// scanners and bots.
			if span := trace.SpanFromContext(ctx); span.IsRecording() {
				span.SetName(r.Method + " " + route)
			}

			if lw.status == 0 {
				lw.status = http.StatusOK
			}

			if !opts.LogSuccessfulRequests && lw.status < 400 {
				if opts.SlowRequestThreshold <= 0 || duration < opts.SlowRequestThreshold {
					return
				}
			}

			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.Int("status", lw.status),
				slog.Duration("duration", duration),
			}
			if lw.problemTitle != "" {
				attrs = append(attrs, slog.String("error_code", lw.problemTitle))
			}

			level := slog.LevelInfo
			switch {
			case lw.status >= 500:
				level = slog.LevelError
			case lw.status >= 400:
				level = slog.LevelWarn
			}
			loggerFromContext(ctx).LogAttrs(ctx, level, "request handled", attrs...)
		})
	}
}
