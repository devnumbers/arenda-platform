package http

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse/ssetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fastStreamTimers shrink the heartbeat and the connection TTL so the
// lifecycle paths run in tests. The production values are in the handler.
func fastStreamTimers(h *StreamHandlers) *StreamHandlers {
	h.Heartbeat = 15 * time.Millisecond
	h.TTL = 150 * time.Millisecond
	return h
}

// newStreamTestServer serves one handler over a real HTTP server and injects
// the authenticated user into every request.
func newStreamTestServer(t *testing.T, h *StreamHandlers, user uuid.UUID) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.StreamNotifications(w, r.WithContext(httpsupport.WithUserID(r.Context(), user)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// openStream connects, reads the handshake bytes (headers, retry hint,
// connected event) and hands the live stream to consume.
func openStream(t *testing.T, srv *httptest.Server, headers map[string]string, consume func(resp *http.Response, handshake string)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()

	consume(resp, ssetest.ReadUntilContains(t, resp.Body, "event: connected"))
}

// getAnonymous calls the server without any auth context and hands the raw
// response to check.
func getAnonymous(t *testing.T, srv *httptest.Server, check func(resp *http.Response)) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()
	check(resp)
}

// expectEOF asserts the stream is over: the server closed the response.
func expectEOF(t *testing.T, body io.Reader) {
	t.Helper()
	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			continue // Buffered frames may still drain before the close.
		}
		if err == io.EOF || strings.Contains(err.Error(), "body closed") {
			return
		}
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestStreamRequiresAuth(t *testing.T) {
	t.Parallel()

	h := NewStreamHandlers(sse.NewHub(nil), slog.Default())
	srv := httptest.NewServer(http.HandlerFunc(h.StreamNotifications))
	t.Cleanup(srv.Close)

	getAnonymous(t, srv, func(resp *http.Response) {
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "application/problem+json")
	})
}

func TestStreamWithoutHubIsUnavailable(t *testing.T) {
	t.Parallel()

	h := NewStreamHandlers(nil, slog.Default())
	srv := newStreamTestServer(t, h, uuid.Must(uuid.NewV7()))

	getAnonymous(t, srv, func(resp *http.Response) {
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	})
}

func TestStreamHandshake(t *testing.T) {
	t.Parallel()

	user := uuid.Must(uuid.NewV7())
	h := NewStreamHandlers(sse.NewHub(nil), slog.Default())
	srv := newStreamTestServer(t, fastStreamTimers(h), user)

	openStream(t, srv, nil, func(resp *http.Response, handshake string) {
		assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
		assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
		assert.Equal(t, "no", resp.Header.Get("X-Accel-Buffering"))
		assert.Contains(t, handshake, "retry: 3000")
		assert.Contains(t, handshake, "event: connected")
		assert.Contains(t, handshake, "data: {}")
		// The TTL ends the connection: the client reconnects and
		// re-authenticates.
		expectEOF(t, resp.Body)
	})
}

func TestStreamDeliversPublishedFrames(t *testing.T) {
	t.Parallel()

	user := uuid.Must(uuid.NewV7())
	hub := sse.NewHub(nil)
	h := NewStreamHandlers(hub, slog.Default())
	srv := newStreamTestServer(t, h, user)

	openStream(t, srv, nil, func(resp *http.Response, handshake string) {
		require.Contains(t, handshake, "event: connected")

		hub.Publish(context.Background(), sse.Event{
			UserID: user,
			Frame:  sse.Frame{Event: "notification.created", Data: `{"v":1}`},
		})

		frame := ssetest.ReadUntilContains(t, resp.Body, "event: notification.created")
		assert.Contains(t, frame, "id: 1", "the hub's monotonic id travels with the frame")
		assert.Contains(t, frame, `data: {"v":1}`)
	})
}

func TestStreamHeartbeatKeepsConnectionAlive(t *testing.T) {
	t.Parallel()

	user := uuid.Must(uuid.NewV7())
	h := NewStreamHandlers(sse.NewHub(nil), slog.Default())
	srv := newStreamTestServer(t, fastStreamTimers(h), user)

	openStream(t, srv, nil, func(resp *http.Response, _ string) {
		assert.Contains(t, ssetest.ReadUntilContains(t, resp.Body, ": ping"), ": ping")
	})
}

func TestStreamLogsLastEventID(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	h := NewStreamHandlers(sse.NewHub(nil), logger)
	srv := newStreamTestServer(t, fastStreamTimers(h), uuid.Must(uuid.NewV7()))

	openStream(t, srv, map[string]string{"Last-Event-ID": "42"}, func(_ *http.Response, _ string) {})

	assert.Contains(t, logs.String(), "42", "the reconnect cursor is logged (v2 replay seam)")
}
