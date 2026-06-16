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

// RequestLogger logs the outcome of every HTTP request.
// It captures method, route, status, duration, request ID and problem error code.
// It never logs bodies, cookies, or phone numbers.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := withLogger(r.Context(), logger)
			r = r.WithContext(ctx)

			lw := &loggingResponseWriter{ResponseWriter: w}
			next.ServeHTTP(lw, r)

			duration := time.Since(start)
			route := chi.RouteContext(ctx).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}

			if lw.status == 0 {
				lw.status = http.StatusOK
			}

			attrs := []slog.Attr{
				slog.String("request_id", RequestIDFromContext(ctx)),
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
