package sse

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
)

// StreamHandler is the shared shell of the SSE stream endpoints (ADR 0060,
// ADR 0062): the auth answer over the session middleware's actor, the
// hub-availability answer and the shared connection loop (Serve). The
// endpoint handlers (the notifications stream and the realtime
// entity-events stream) embed it and keep only their operation names and
// endpoint docs.
type StreamHandler struct {
	hub    *Hub
	logger *slog.Logger

	// Heartbeat and TTL hold the production lifecycle values; tests shrink
	// them in place.
	Heartbeat time.Duration
	TTL       time.Duration
}

// NewStreamHandler creates the stream handler with the production lifecycle
// values (ADR 0060): the heartbeat keeps NAT/proxy idle timers away, the TTL
// ends every connection so the client re-authenticates through the session
// middleware on reconnect.
func NewStreamHandler(hub *Hub, logger *slog.Logger) *StreamHandler {
	return &StreamHandler{
		hub:       hub,
		logger:    logger,
		Heartbeat: DefaultHeartbeat,
		TTL:       DefaultConnTTL,
	}
}

// ServeStream authenticates the request's actor, answers the hub
// availability and hands the live connection to the shared transport loop.
func (h *StreamHandler) ServeStream(w http.ResponseWriter, r *http.Request) {
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

	Serve(w, r, h.hub, userID, h.Heartbeat, h.TTL, h.logger)
}
