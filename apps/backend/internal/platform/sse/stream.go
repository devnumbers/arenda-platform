package sse

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// The connection lifecycle defaults shared by every stream endpoint (ADR
// 0060): the heartbeat keeps NAT/proxy idle timers away (a write failure on a
// ping surfaces a dead connection within one interval), the TTL ends every
// connection after an hour so the client re-authenticates through the session
// middleware on reconnect, and the retry hint paces the browser's reconnect.
const (
	DefaultHeartbeat    = 25 * time.Second
	DefaultConnTTL      = time.Hour
	ReconnectHintMs     = 3000
	connectedEventName  = "connected"
	lastEventIDLogScope = "stream reconnect with last event id"
)

// Serve holds one authenticated user's stream connection open and writes the
// hub's frames to it (ADR 0060): the SSE headers, the retry hint and the
// connected start frame (no id — it is not part of the event sequence), then
// the select loop over the client's disappearance, the subscribed frames, the
// heartbeat and the TTL. The handler owning the endpoint keeps the auth
// check, the hub-availability answer and the endpoint's event semantics; this
// is the transport loop they share. The caller must pass a non-nil hub.
//
// Last-Event-ID is the browser's reconnect cursor. Replay is not implemented
// (v1, ADR 0060): the client re-reads the state through its API on open; the
// log keeps the seam visible for a v2 replay buffer.
func Serve(
	w http.ResponseWriter, r *http.Request,
	hub *Hub, userID uuid.UUID,
	heartbeat, ttl time.Duration,
	log *slog.Logger,
) {
	if lastEventID := r.Header.Get("Last-Event-ID"); lastEventID != "" && log != nil {
		log.InfoContext(r.Context(), lastEventIDLogScope,
			slog.String("last_event_id", lastEventID))
	}

	WriteHeaders(w)
	flusher, ok := w.(http.Flusher)
	if !ok {
		// A ResponseWriter without flush support cannot carry a stream; the
		// headers are already sent, so the connection just ends.
		if log != nil {
			log.ErrorContext(r.Context(), "response writer does not support flushing")
		}
		return
	}
	if err := WriteRetry(w, ReconnectHintMs); err != nil {
		return
	}
	// The start frame tells the client the stream is live; no id — it is not
	// part of the event sequence.
	if _, err := w.Write(Frame{Event: connectedEventName, Data: "{}"}.Bytes()); err != nil {
		return
	}
	flusher.Flush()

	frames, unsubscribe := hub.Subscribe(r.Context(), userID)
	defer unsubscribe()

	heartbeatTicker := time.NewTicker(heartbeat)
	defer heartbeatTicker.Stop()
	connTTL := time.NewTimer(ttl)
	defer connTTL.Stop()

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
		case <-heartbeatTicker.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-connTTL.C:
			return
		}
	}
}
