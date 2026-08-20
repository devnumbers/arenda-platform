// Package requestctx carries request-scoped diagnostics (request ID, trace ID, client IP) through context.
package requestctx

import "context"

type (
	requestIDKey struct{}
	traceIDKey   struct{}
	clientIPKey  struct{}
)

// WithRequestID stores the request ID in context for cross-cutting diagnostics.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the request ID stored in context, if any.
func RequestIDFromContext(ctx context.Context) string {
	id, ok := ctx.Value(requestIDKey{}).(string)
	if !ok {
		return ""
	}
	return id
}

// WithTraceID stores the lightweight local trace ID in context.
func WithTraceID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, traceIDKey{}, id)
}

// TraceIDFromContext returns the trace ID stored in context, if any.
func TraceIDFromContext(ctx context.Context) string {
	id, ok := ctx.Value(traceIDKey{}).(string)
	if !ok {
		return ""
	}
	return id
}

// WithClientIP returns a context carrying the client IP address.
func WithClientIP(ctx context.Context, ip string) context.Context {
	if ip == "" {
		return ctx
	}
	return context.WithValue(ctx, clientIPKey{}, ip)
}

// ClientIPFromContext returns the client IP address stored in context, if any.
func ClientIPFromContext(ctx context.Context) string {
	ip, ok := ctx.Value(clientIPKey{}).(string)
	if !ok {
		return ""
	}
	return ip
}
