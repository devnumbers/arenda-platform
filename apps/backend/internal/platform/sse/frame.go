// Package sse is the shared user event stream's transport primitives
// (карта #734, #742; ADR 0060): the SSE frame writer and the per-user
// subscription hub. The package knows nothing about notification semantics —
// the envelope's wire contract lives here too (envelope.go, ADR 0060 §5),
// contexts fill only their payload and address recipients through the hub;
// the stream's first consumer is notifications (#742), the map #714 rewires
// as a consumer of the same transport.
package sse

import (
	"fmt"
	"net/http"
	"strings"
)

// Frame is one SSE frame. Data must be a single line — the SSE grammar cuts
// frames at newlines, so multi-line payloads would turn into broken frames.
type Frame struct {
	// ID is the hub's monotonic sequence number; empty means "no id" (the
	// browser's Last-Event-ID cursor stays untouched).
	ID string
	// Event is the stable coarse event name the browser dispatches to its
	// addEventListener(name); empty means the generic "message" event.
	Event string
	// Data is the ready-to-send payload, one line of JSON for the envelope.
	Data string
}

// Bytes renders the frame per the WHATWG server-sent events grammar. Field
// order (id → event → data) is fixed for readability in DevTools.
func (f Frame) Bytes() []byte {
	var b strings.Builder
	if f.ID != "" {
		b.WriteString("id: ")
		b.WriteString(f.ID)
		b.WriteByte('\n')
	}
	if f.Event != "" {
		b.WriteString("event: ")
		b.WriteString(f.Event)
		b.WriteByte('\n')
	}
	b.WriteString("data: ")
	b.WriteString(f.Data)
	b.WriteString("\n\n")
	return []byte(b.String())
}

// WriteHeaders turns the response into an event stream: the Content-Type is
// what makes the browser treat the connection as SSE (anything else fails the
// connection), no-store keeps proxies from caching, and X-Accel-Buffering
// asks nginx-family proxies not to buffer (Caddy ignores it — harmless).
// The server's WriteTimeout is 0 (ADR 0060): no per-connection deadline to
// clear, the heartbeat's write errors surface dead connections.
func WriteHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
}

// WriteRetry hints the browser's reconnect delay in milliseconds.
func WriteRetry(w http.ResponseWriter, ms int) error {
	_, err := fmt.Fprintf(w, "retry: %d\n\n", ms)
	return err
}
