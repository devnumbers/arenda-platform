// Package http hosts the realtime entity-events stream's HTTP adapter
// (карта #714, #716; ADR 0062): the second per-user SSE endpoint.
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
)

// RealtimeStreamHandlers implements the realtime entity-events stream (карта #714,
// тикет #716; ADR 0062): GET /realtime/stream is the second per-user SSE
// endpoint over the session middleware's actor, carrying the coarse
// entity.changed invalidation frames. No entity data rides the stream — the
// clients re-read through their APIs. The connection loop is the shared
// transport (sse.Serve, the notifications stream's twin); this handler owns
// the endpoint's auth and availability answers.
type RealtimeStreamHandlers struct {
	hub    *sse.Hub
	logger *slog.Logger

	// Heartbeat and TTL hold the production lifecycle values; tests shrink
	// them in place.
	heartbeat time.Duration
	ttl       time.Duration
}

// NewRealtimeStreamHandlers creates the realtime stream handler. The per-user
// connection budget (8, ADR 0060) is shared with the notifications stream —
// both endpoints subscribe the same hub — so a tab holding both streams
// counts two of its owner's eight.
func NewRealtimeStreamHandlers(hub *sse.Hub, logger *slog.Logger) *RealtimeStreamHandlers {
	return &RealtimeStreamHandlers{
		hub:       hub,
		logger:    logger,
		heartbeat: sse.DefaultHeartbeat,
		ttl:       sse.DefaultConnTTL,
	}
}

// StreamRealtime implements GET /realtime/stream.
func (h *RealtimeStreamHandlers) StreamRealtime(w http.ResponseWriter, r *http.Request) {
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
