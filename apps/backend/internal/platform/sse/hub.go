package sse

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

const (
	// The per-connection frame buffer size. An overflowing buffer is the
	// slow-consumer signal: the connection is dismissed rather than allowed
	// to grow memory without bound.
	subscriberBuffer = 32
	// The sanitary per-user connection limit (tabs, devices, stuck
	// connections). The oldest connection is displaced — the client
	// reconnects on its own.
	maxConnsPerUser = 8
) // Event is one frame addressed to one user's open connections.
type Event struct {
	UserID uuid.UUID
	// Frame carries the event name and the one-line payload; the hub stamps
	// the monotonic frame ID on publish.
	Frame Frame
}

// subscriber is one open connection.
type subscriber struct {
	userID uuid.UUID
	ch     chan Frame
	// The once-guard keeps the channel close single: the owning handler's
	// unsubscribe, the hub's eviction, a slow-consumer drop and the hub
	// shutdown can all race to dismiss the same connection.
	once sync.Once
}

// Hub keeps the registry of open stream connections and fans events out to
// them (ADR 0060). It is the in-memory transport for the single-instance
// backend; the N-replica upgrade path is a Postgres LISTEN/NOTIFY bridge
// feeding the same hub, so the Subscribe/Publish surface does not change.
type Hub struct {
	mu      sync.RWMutex
	byUser  map[uuid.UUID][]*subscriber
	done    chan struct{}
	seq     atomic.Uint64
	metrics *Metrics
}

// NewHub creates an empty hub. A nil Metrics disables instrumentation.
func NewHub(m *Metrics) *Hub {
	return &Hub{
		byUser:  make(map[uuid.UUID][]*subscriber),
		done:    make(chan struct{}),
		metrics: m,
	}
}

// Subscribe registers one connection and returns its frame channel plus the
// unsubscribe function the stream handler defers. The returned function is
// idempotent and safe to race with the hub's own dismissals.
func (h *Hub) Subscribe(ctx context.Context, userID uuid.UUID) (frames <-chan Frame, unsubscribe func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	sub := &subscriber{userID: userID, ch: make(chan Frame, subscriberBuffer)}
	select {
	case <-h.done:
		// The hub is shutting down: hand back an already-closed channel so
		// the handler returns immediately.
		close(sub.ch)
		return sub.ch, func() {}
	default:
	}

	conns := h.byUser[userID]
	if len(conns) >= maxConnsPerUser {
		h.dismissLocked(ctx, conns[0])
		conns = h.byUser[userID]
	}
	h.byUser[userID] = append(conns, sub)
	h.metrics.connOpened(ctx)

	return sub.ch, func() { h.dismiss(ctx, sub) }
}

// Publish delivers the event to every open connection of the user without
// ever blocking: a connection whose buffer is full is dismissed (closed) —
// its client reconnects and re-reads the state through its API. Publish is a
// no-op after Close.
//
// The sends run under the read lock: the channel close paths (dismissal,
// eviction, shutdown) all take the write lock, so a send can never race a
// close. Drops are collected and dismissed after the lock is released —
// dismissing inline would upgrade the lock and deadlock.
func (h *Hub) Publish(ctx context.Context, e Event) {
	h.mu.RLock()
	select {
	case <-h.done:
		h.mu.RUnlock()
		return
	default:
	}
	frame := e.Frame
	frame.ID = strconv.FormatUint(h.seq.Add(1), 10)
	var dropped []*subscriber
	for _, c := range h.byUser[e.UserID] {
		select {
		case c.ch <- frame:
		default:
			dropped = append(dropped, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range dropped {
		h.dismiss(ctx, c)
		h.metrics.frameDropped(ctx)
	}
}

// Close shuts the hub down: every open connection's channel is closed, so the
// stream handlers return and a graceful HTTP shutdown is not held up by
// long-lived streams.
func (h *Hub) Close(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.done:
		return
	default:
	}
	close(h.done)
	for _, conns := range h.byUser {
		for _, c := range conns {
			h.dismissLocked(ctx, c)
		}
	}
}

// dismiss closes the connection once and removes it from the registry.
func (h *Hub) dismiss(ctx context.Context, sub *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dismissLocked(ctx, sub)
}

// dismissLocked is dismiss for callers already holding the write lock.
func (h *Hub) dismissLocked(ctx context.Context, sub *subscriber) {
	sub.once.Do(func() {
		close(sub.ch)
		h.metrics.connClosed(ctx)
		conns := h.byUser[sub.userID]
		for i, c := range conns {
			if c == sub {
				h.byUser[sub.userID] = append(conns[:i], conns[i+1:]...)
				break
			}
		}
		if len(h.byUser[sub.userID]) == 0 {
			delete(h.byUser, sub.userID)
		}
	})
}
