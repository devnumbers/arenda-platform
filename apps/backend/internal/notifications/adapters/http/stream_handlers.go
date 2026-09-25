package http

import (
	"log/slog"
	"net/http"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
)

// StreamHandlers implements the shared user event stream (карта #734, #742;
// ADR 0060): GET /notifications/stream is an SSE endpoint over the session
// middleware's actor. The stream carries no domain state — coarse event
// frames only, the clients re-read through their APIs. The connection loop —
// the handshake, the heartbeat, the TTL, the frame writes — is the shared
// transport (sse.Serve, ADR 0062's second stream rides it too); the auth and
// availability answers live in the embedded shared shell.
type StreamHandlers struct {
	*sse.StreamHandler
}

// NewStreamHandlers creates the event stream handler.
func NewStreamHandlers(hub *sse.Hub, logger *slog.Logger) *StreamHandlers {
	return &StreamHandlers{
		StreamHandler: sse.NewStreamHandler(hub, logger),
	}
}

// StreamNotifications implements GET /notifications/stream.
func (h *StreamHandlers) StreamNotifications(w http.ResponseWriter, r *http.Request) {
	h.ServeStream(w, r)
}
