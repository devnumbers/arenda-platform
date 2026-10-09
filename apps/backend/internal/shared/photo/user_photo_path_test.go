package photo

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestUserPhotoPath pins the same-origin streaming path: the user's uuid in
// the /users segment, and the empty string for the photoless profile — the
// nullable-contract null and the payload snapshots' empty string.
func TestUserPhotoPath(t *testing.T) {
	t.Parallel()
	id := uuid.Must(uuid.NewV7())

	if got := UserPhotoPath(id, new("photos/x.jpg")); !strings.HasSuffix(got, "/api/v1/users/"+id.String()+"/photo") {
		t.Fatalf("UserPhotoPath = %q, want the /users path suffix", got)
	}
	if got := UserPhotoPath(id, nil); got != "" {
		t.Fatalf("UserPhotoPath(nil key) = %q, want empty", got)
	}
}
