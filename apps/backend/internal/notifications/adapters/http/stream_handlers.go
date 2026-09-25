package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
)

// StreamHandlers implements the shared user event stream (карта #734, #742;
// ADR 0060): GET /notifications/stream is an SSE endpoint over the session
// middleware's actor. The stream carries no domain state — coarse event
// frames only, the clients re-read through their APIs. The connection loop —
// the handshake, the heartbeat, the TTL, the frame writes — is the shared
// transport (sse.Serve, ADR 0062's second stream rides it too); this handler
// owns the endpoint's auth and availability answers.
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
		heartbeat: sse.DefaultHeartbeat,
		ttl:       sse.DefaultConnTTL,
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

	sse.Serve(w, r, h.hub, userID, h.heartbeat, h.ttl, h.logger)
}
