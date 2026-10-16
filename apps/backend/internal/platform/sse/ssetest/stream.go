// Package ssetest holds helpers shared by the SSE stream handlers' test
// families (notifications, realtime): the streams share one wire behaviour,
// so the reader is shared instead of copied per context.
package ssetest

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// ReadUntilContains reads the stream until the buffer contains the marker and
// returns everything read so far.
func ReadUntilContains(t *testing.T, body io.Reader, marker string) string {
	t.Helper()
	var acc bytes.Buffer
	buf := make([]byte, 4096)
	for !strings.Contains(acc.String(), marker) {
		n, err := body.Read(buf)
		if n > 0 {
			if _, werr := acc.Write(buf[:n]); werr != nil {
				t.Fatalf("buffer write: %v", werr)
			}
		}
		if err != nil {
			t.Fatalf("stream ended before %q: %v (read so far: %q)", marker, err, acc.String())
		}
	}
	return acc.String()
}
