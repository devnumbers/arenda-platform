// Package http hosts the realtime entity-events stream's HTTP adapter
// (карта #714, #716; ADR 0062): the second per-user SSE endpoint.
package http

import (
	"log/slog"
	"net/http"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
)

// RealtimeStreamHandlers implements the realtime entity-events stream (карта #714,
// тикет #716; ADR 0062): GET /realtime/stream is the second per-user SSE
// endpoint over the session middleware's actor, carrying the coarse
// entity.changed invalidation frames. No entity data rides the stream — the
// clients re-read through their APIs. The connection loop is the shared
// transport (sse.Serve, the notifications stream's twin); the auth and
// availability answers live in the embedded shared shell.
type RealtimeStreamHandlers struct {
	*sse.StreamHandler
}

// NewRealtimeStreamHandlers creates the realtime stream handler. The per-user
// connection budget (8, ADR 0060) is shared with the notifications stream —
// both endpoints subscribe the same hub — so a tab holding both streams
// counts two of its owner's eight.
func NewRealtimeStreamHandlers(hub *sse.Hub, logger *slog.Logger) *RealtimeStreamHandlers {
	return &RealtimeStreamHandlers{
		StreamHandler: sse.NewStreamHandler(hub, logger),
	}
}

// StreamRealtime implements GET /realtime/stream.
func (h *RealtimeStreamHandlers) StreamRealtime(w http.ResponseWriter, r *http.Request) {
	h.ServeStream(w, r)
}
