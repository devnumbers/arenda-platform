package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	sharedstorage "github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
	"github.com/stretchr/testify/require"
)

// The dangling-photo self-heal (ADR 0065): the storage has lost the object
// the columns advertise (a fake-storage restart, a drifted bucket) — the
// stream still answers the honest 404, and the dangling key is repaired so
// /me stops promising a photo the storage will never deliver. The UI then
// shows the truthful «Добавить фото» instead of a silent dead placeholder.

func TestProfileService_OpenPhoto_HealsDanglingKey(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)

	// The object vanishes behind the backend's back.
	require.NoError(t, h.photos.Delete(context.Background(), *user.PhotoKey))

	opened, err := h.svc.OpenPhoto(context.Background(), user.ID, user.ID)
	require.ErrorIs(t, err, ErrPhotoNotFound)
	require.Nil(t, opened.Body)

	stored, err := h.users.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.Nil(t, stored.PhotoKey, "the dangling key must be healed away")
	require.Nil(t, stored.PhotoContentType)
}

// failingOpenStorage passes everything through to the real fake except the
// opens of one key, which fail with a transient (non-NotFound) error.
type failingOpenStorage struct {
	sharedstorage.PhotoStorage
	failKey string
}

func (s *failingOpenStorage) Open(ctx context.Context, key string) (body io.ReadCloser, size int64, contentType string, err error) {
	if key == s.failKey {
		return nil, 0, "", errors.New("dial storage: connection refused")
	}
	return s.PhotoStorage.Open(ctx, key)
}

func TestProfileService_OpenPhoto_TransientErrorSparesTheKey(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)
	h.svc = NewProfileService(
		h.factory(h.audit),
		&failingOpenStorage{PhotoStorage: h.photos, failKey: *user.PhotoKey},
		h.checker,
		slog.New(slog.DiscardHandler),
	)

	// A storage outage is not a lost object: the columns must keep
	// advertising the photo, the viewer gets the 500-shaped failure.
	_, err := h.svc.OpenPhoto(context.Background(), user.ID, user.ID)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrPhotoNotFound)

	stored, err := h.users.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.PhotoKey, "a transient storage error must not heal the key away")
}

func TestProfileService_OpenPhoto_HealSparesReplacedKey(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)

	// The direct contract of the conditional clear: the heal of the dead
	// key must not wipe a fresh one. The SQL pins the same semantics.
	require.NoError(t, h.photos.Delete(context.Background(), *user.PhotoKey))
	rows, err := h.users.ClearPhotoIfKey(context.Background(), user.ID, "photos/other-key.jpg")
	require.NoError(t, err)
	require.Zero(t, rows, "a key that is not advertised must not be cleared")

	stored, err := h.users.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.PhotoKey)
}
