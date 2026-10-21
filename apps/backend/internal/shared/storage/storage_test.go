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

// TestVersionOfPhotoURL pins the cache version of a photo key (ADR 0065):
// the key's sha256 hex prefix — the same derivation the serving ETag uses —
// and the composition of a streaming path with it as the ?v= query. The
// expected hex is an independent literal (sha256("photos/x.jpg")[:16]),
// not a recomputation.
func TestVersionOfPhotoURL(t *testing.T) {
	t.Parallel()
	if got := VersionOf("photos/x.jpg"); got != "8fe0a9930afe3eabb4560c5db3f4da44" {
		t.Fatalf("VersionOf = %q, want the pinned sha256 hex prefix", got)
	}
	got := VersionedPhotoURL("/api/me/photo", "photos/x.jpg")
	want := "/api/me/photo?v=8fe0a9930afe3eabb4560c5db3f4da44"
	if got != want {
		t.Fatalf("VersionedPhotoURL = %q, want %q", got, want)
	}
}
