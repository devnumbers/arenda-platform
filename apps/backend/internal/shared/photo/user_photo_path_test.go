package photo

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestUserPhotoPath pins the same-origin streaming path: the user's uuid in
// the /users segment, and the empty string for the photoless profile — the
// nullable-contract null and the payload snapshots' empty string. The exact
// versioned form is pinned by TestUserPhotoPathVersioned.
func TestUserPhotoPath(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())

	if got := UserPhotoPath(id, new("photos/x.jpg")); !strings.HasPrefix(got, "/api/users/"+id.String()+"/photo") {
		t.Fatalf("UserPhotoPath = %q, want the /users path prefix", got)
	}
	if got := UserPhotoPath(id, nil); got != "" {
		t.Fatalf("UserPhotoPath(nil key) = %q, want empty", got)
	}
}

// TestUserPhotoPathVersioned pins the cache version query (ADR 0065): a key
// carries its sha256 hex prefix as ?v=, so a replaced photo (a new UUID key)
// is a new URL and no browser serves the previous bytes. The expected hex is
// an independent literal (sha256("photos/x.jpg")[:16]).
func TestUserPhotoPathVersioned(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())

	want := "/api/users/" + id.String() + "/photo?v=8fe0a9930afe3eabb4560c5db3f4da44"
	if got := UserPhotoPath(id, new("photos/x.jpg")); got != want {
		t.Fatalf("UserPhotoPath = %q, want %q", got, want)
	}
}
