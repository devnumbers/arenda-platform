package sse

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrameBytesFull(t *testing.T) {
	t.Parallel()

	f := Frame{ID: "7", Event: "notification.created", Data: `{"a":1}`}
	assert.Equal(t, "id: 7\nevent: notification.created\ndata: {\"a\":1}\n\n", string(f.Bytes()))
}

func TestFrameBytesDataOnly(t *testing.T) {
	t.Parallel()

	// No id/event fields: a generic message frame the browser dispatches to
	// the onmessage listener.
	f := Frame{Data: "hello"}
	assert.Equal(t, "data: hello\n\n", string(f.Bytes()))
}

func TestFrameBytesFieldOrder(t *testing.T) {
	t.Parallel()

	// The SSE parser treats each line independently, but the stable
	// id → event → data order keeps frames readable in DevTools.
	f := Frame{ID: "1", Event: "e", Data: "d"}
	lines := strings.Split(strings.TrimRight(string(f.Bytes()), "\n"), "\n")
	assert.Equal(t, []string{"id: 1", "event: e", "data: d"}, lines)
}

func TestWriteHeaders(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	WriteHeaders(w)

	h := w.Header()
	assert.Equal(t, "text/event-stream; charset=utf-8", h.Get("Content-Type"))
	assert.Equal(t, "no-store", h.Get("Cache-Control"))
	assert.Equal(t, "no", h.Get("X-Accel-Buffering"))
	assert.Equal(t, 200, w.Code)
}

func TestWriteRetry(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	require.NoError(t, WriteRetry(w, 3000))
	assert.Equal(t, "retry: 3000\n\n", w.Body.String())
}
