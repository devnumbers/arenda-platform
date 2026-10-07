package storage

import (
	"strings"
	"testing"
)

func TestETagOf(t *testing.T) {
	t.Parallel()

	etag := ETagOf("photos/01987654-3210-7abc-9def-0123456789ab.jpg")
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("etag = %q, want a quoted strong etag", etag)
	}
	if again := ETagOf("photos/01987654-3210-7abc-9def-0123456789ab.jpg"); again != etag {
		t.Errorf("etag not deterministic: %q vs %q", etag, again)
	}
	if other := ETagOf("photos/01987654-3210-7abc-9def-0123456789ac.jpg"); other == etag {
		t.Error("different keys must produce different etags")
	}
}
