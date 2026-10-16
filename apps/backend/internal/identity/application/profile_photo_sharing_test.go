package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/stretchr/testify/require"
)

// fakeSharedChecker is the SharedPropertyChecker fake: a canned verdict plus
// the call counter the self-read test asserts against.
type fakeSharedChecker struct {
	shared bool
	err    error
	calls  int
}

func (f *fakeSharedChecker) ShareReadableProperty(_ context.Context, _, _ uuid.UUID) (bool, error) {
	f.calls++
	return f.shared, f.err
}

// seedPhotoUser creates a verified owner whose profile photo exists both as
// the user's key and as a stored object. The fake repo stores users by
// value, so the keyed user is written back.
func seedPhotoUser(t *testing.T, h *profileHarness) domain.User {
	t.Helper()
	user := seedProfileUser(t, h.users)
	key := "photos/" + uuid.Must(uuid.NewV7()).String() + ".jpg"
	if err := h.photos.Put(context.Background(), key, strings.NewReader("jpeg-bytes"), "image/jpeg", 10); err != nil {
		t.Fatalf("put photo: %v", err)
	}
	user.PhotoKey = new(key)
	user.PhotoContentType = new("image/jpeg")
	h.users.byPhone[user.Phone.String()] = user
	return user
}

// TestProfileService_OpenPhoto_SelfSkipsAccessCheck proves the owner reads
// their own photo without consulting the shared-property checker: /me/photo
// never pays the relation query.
func TestProfileService_OpenPhoto_SelfSkipsAccessCheck(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)

	opened, err := h.svc.OpenPhoto(context.Background(), user.ID, user.ID)
	if err != nil {
		t.Fatalf("OpenPhoto error = %v", err)
	}
	t.Cleanup(func() { require.NoError(t, opened.Body.Close()) })
	if opened.Size != 10 || opened.ContentType != "image/jpeg" || opened.Key != *user.PhotoKey {
		t.Fatalf("open = (%d, %q, %q), want (10, image/jpeg, %q)", opened.Size, opened.ContentType, opened.Key, *user.PhotoKey)
	}
	if h.checker.calls != 0 {
		t.Errorf("checker calls = %d, want 0 for a self-read", h.checker.calls)
	}
}

// TestProfileService_OpenPhoto_SharedObjectAllows proves a viewer connected
// through a shared readable property (ADR 0028, symmetric) opens the other
// user's photo (решение #1286).
func TestProfileService_OpenPhoto_SharedObjectAllows(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)
	h.checker.shared = true
	viewer := uuid.Must(uuid.NewV7())

	opened, err := h.svc.OpenPhoto(context.Background(), viewer, user.ID)
	if err != nil {
		t.Fatalf("OpenPhoto error = %v", err)
	}
	t.Cleanup(func() { require.NoError(t, opened.Body.Close()) })
	if h.checker.calls != 1 {
		t.Errorf("checker calls = %d, want 1", h.checker.calls)
	}
}

// TestProfileService_OpenPhoto_NoSharedObjectIsPhotoNotFound proves a viewer
// without a readable shared property gets the same ErrPhotoNotFound as a
// missing photo: existence is not disclosed (privacy-404, #152).
func TestProfileService_OpenPhoto_NoSharedObjectIsPhotoNotFound(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)
	viewer := uuid.Must(uuid.NewV7())

	_, err := h.svc.OpenPhoto(context.Background(), viewer, user.ID)
	if !errors.Is(err, ErrPhotoNotFound) {
		t.Fatalf("OpenPhoto error = %v, want ErrPhotoNotFound", err)
	}
}

// TestProfileService_OpenPhoto_CheckerFailureFailsClosed proves a checker
// outage never degrades into a pass or a 404: the error propagates for the
// transport's 500 (fail-closed gates, #998).
func TestProfileService_OpenPhoto_CheckerFailureFailsClosed(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)
	checkerErr := errors.New("relation store down")
	h.checker.err = checkerErr
	viewer := uuid.Must(uuid.NewV7())

	_, err := h.svc.OpenPhoto(context.Background(), viewer, user.ID)
	if !errors.Is(err, checkerErr) {
		t.Fatalf("OpenPhoto error = %v, want wrap of %v", err, checkerErr)
	}
	if errors.Is(err, ErrPhotoNotFound) {
		t.Fatal("checker failure must not read as photo-not-found")
	}
}

// TestProfileService_PhotoDescriptor_SharedObjectGivesKey proves the
// descriptor (the serving ETag source) is gated by the same rule as the body.
func TestProfileService_PhotoDescriptor_SharedObjectGivesKey(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)
	h.checker.shared = true
	viewer := uuid.Must(uuid.NewV7())

	key, contentType, err := h.svc.PhotoDescriptor(context.Background(), viewer, user.ID)
	if err != nil {
		t.Fatalf("PhotoDescriptor error = %v", err)
	}
	if key != *user.PhotoKey || contentType != "image/jpeg" {
		t.Fatalf("descriptor = (%q, %q), want (%q, image/jpeg)", key, contentType, *user.PhotoKey)
	}
}

// TestProfileService_PhotoDescriptor_NoSharedObjectIsPhotoNotFound proves the
// 304 answer cannot leak a photo the viewer must not see: no relation —
// ErrPhotoNotFound before the key is read.
func TestProfileService_PhotoDescriptor_NoSharedObjectIsPhotoNotFound(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)
	viewer := uuid.Must(uuid.NewV7())

	_, _, err := h.svc.PhotoDescriptor(context.Background(), viewer, user.ID)
	if !errors.Is(err, ErrPhotoNotFound) {
		t.Fatalf("PhotoDescriptor error = %v, want ErrPhotoNotFound", err)
	}
}

// compile-time guard the tests rely on: the harness fake implements the port.
var _ SharedPropertyChecker = (*fakeSharedChecker)(nil)
