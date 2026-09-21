package http

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
)

const (
	// The heartbeat keeps NAT/proxy idle timers away from the connection: a
	// write failure on the ping surfaces a dead connection within one
	// interval.
	streamHeartbeatInterval = 25 * time.Second
	// The TTL ends every connection after an hour, so a client
	// re-authenticates through the session middleware on reconnect (ADR 0060).
	streamMaxConnTTL = time.Hour
	// The browser's reconnect delay hint (retry:).
	streamReconnectHintMs = 3000
)

// StreamHandlers implements the shared user event stream (карта #734, #742;
// ADR 0060): GET /notifications/stream is an SSE endpoint over the session
// middleware's actor. The stream carries no domain state — coarse event
// frames only, the clients re-read through their APIs.
type StreamHandlers struct {
	hub    *sse.Hub
	logger *slog.Logger

	// Heartbeat and TTL hold the production lifecycle values; tests shrink
	// them in place.
	heartbeat time.Duration
	ttl       time.Duration
}

// NewStreamHandlers creates the event stream handler.
func NewStreamHandlers(hub *sse.Hub, logger *slog.Logger) *StreamHandlers {
	return &StreamHandlers{
		hub:       hub,
		logger:    logger,
		heartbeat: streamHeartbeatInterval,
		ttl:       streamMaxConnTTL,
	}
}

// StreamNotifications implements GET /notifications/stream.
func (h *StreamHandlers) StreamNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}
	if h.hub == nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusServiceUnavailable,
			httpsupport.Problem(r.Context(), "Service Unavailable", "Стрим событий недоступен"))
		return
	}

	// Last-Event-ID is the browser's reconnect cursor. Replay is not
	// implemented (v1, ADR 0060): the client re-reads the state through its
	// API on open; the log keeps the seam visible for a v2 replay buffer.
	if lastEventID := r.Header.Get("Last-Event-ID"); lastEventID != "" {
		h.logger.InfoContext(r.Context(), "stream reconnect with last event id",
			slog.String("last_event_id", lastEventID))
	}

	sse.WriteHeaders(w)
	flusher, ok := w.(http.Flusher)
	if !ok {
		// A ResponseWriter without flush support cannot carry a stream; the
		// headers are already sent, so the connection just ends.
		h.logger.ErrorContext(r.Context(), "response writer does not support flushing")
		return
	}
	if err := sse.WriteRetry(w, streamReconnectHintMs); err != nil {
		return
	}
	// The start frame tells the client the stream is live; no id — it is not
	// part of the event sequence.
	if _, err := w.Write(sse.Frame{Event: "connected", Data: "{}"}.Bytes()); err != nil {
		return
	}
	flusher.Flush()

	frames, unsubscribe := h.hub.Subscribe(r.Context(), userID)
	defer unsubscribe()

	heartbeat := time.NewTicker(h.heartbeat)
	defer heartbeat.Stop()
	ttl := time.NewTimer(h.ttl)
	defer ttl.Stop()

	for {
		select {
		case <-r.Context().Done(): // The client is gone.
			return
		case frame, open := <-frames:
			if !open { // The hub closed the connection (shutdown, eviction).
				return
			}
			if _, err := w.Write(frame.Bytes()); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-ttl.C:
			return
		}
	}
}
