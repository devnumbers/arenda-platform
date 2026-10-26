package application

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/objectstorage"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/stretchr/testify/require"
)

// The dangling-card-photo self-heal (ADR 0065): the storage has lost the
// object the columns advertise — the stream still answers the honest 404,
// and the dangling key is repaired so the book stops promising a photo the
// storage will never deliver.

func TestContactService_OpenContactPhoto_HealsDanglingKey(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	// The service is rebuilt around a harness-owned storage: the test must
	// be able to lose an object behind the backend's back.
	photos := objectstorage.NewFakeStorage()
	h.svc = NewContactService(
		NewTxStoreFactory(
			h.contacts,
			fakePropertyStore{owners: map[uuid.UUID]uuid.UUID{h.property: h.owner}},
			h.audit,
			h.history,
			fakeUoW{},
		),
		stubPolicy{role: sharedpolicy.RoleOwner},
		photos,
	)

	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(nil))
	require.NoError(t, err, "create contact")

	key := "photos/" + uuid.Must(uuid.NewV7()).String() + ".jpg"
	contentType := photo.ContentTypeJPEG
	require.NoError(t, photos.Put(context.Background(), key, strings.NewReader("jpeg-bytes"), contentType, 10))
	_, err = h.contacts.SetPhoto(context.Background(), created.ID, h.owner, &key, &contentType, nil)
	require.NoError(t, err, "seed photo")

	// The object vanishes behind the backend's back.
	require.NoError(t, photos.Delete(context.Background(), key))

	body, size, _, key, err := h.svc.OpenContactPhoto(context.Background(), h.owner, created.ID, false)
	require.ErrorIs(t, err, ErrPhotoNotFound)
	require.Nil(t, body)
	require.Zero(t, size)
	require.Empty(t, key)

	stored, err := h.contacts.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	require.Nil(t, stored.PhotoKey, "the dangling key must be healed away")
	require.Nil(t, stored.PhotoContentType)
}

// The thumbnail's life next to the card photo (ADR 0065, тикет миниатюр):
// the thumb-requesting open serves the derived _t object when it exists, a
// pre-thumb object materializes its thumb lazily, a webp falls back to the
// full object, and a healed dangling key takes its stray thumb with it.

// realContactJPEG renders a genuinely decodable jpeg — the lazy
// materialization branches on decodability.
func realContactJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	return buf.Bytes()
}

// openContactPhotoForRead unwraps the 5-value OpenContactPhoto for the
// assertions that care about the body, the key and the error only.
func openContactPhotoForRead(
	h *serviceHarness, id uuid.UUID, wantThumb bool,
) (io.ReadCloser, string, error) {
	body, _, _, key, err := h.svc.OpenContactPhoto(context.Background(), h.owner, id, wantThumb)
	return body, key, err
}

// newThumbHarness rebuilds the service around a harness-owned storage (the
// test must see and lose objects behind the backend's back).
func newThumbHarness(t *testing.T) (*serviceHarness, *objectstorage.FakeStorage) {
	t.Helper()
	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	photos := objectstorage.NewFakeStorage()
	h.svc = NewContactService(
		NewTxStoreFactory(
			h.contacts,
			fakePropertyStore{owners: map[uuid.UUID]uuid.UUID{h.property: h.owner}},
			h.audit,
			h.history,
			fakeUoW{},
		),
		stubPolicy{role: sharedpolicy.RoleOwner},
		photos,
	)
	return h, photos
}

func TestContactService_StoresAndServesThumb(t *testing.T) {
	t.Parallel()

	h, photos := newThumbHarness(t)
	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(nil))
	require.NoError(t, err, "create contact")

	updated, err := h.svc.SetContactPhoto(context.Background(), h.owner, created.ID, photo.Processed{
		Data:        []byte("jpeg-bytes"),
		ContentType: photo.ContentTypeJPEG,
		Size:        10,
		Thumb:       []byte("thumb-bytes"),
	})
	require.NoError(t, err)

	thumbKey := photo.ThumbKeyOf(*updated.PhotoKey)
	thumbBytes, ok := photos.Object(thumbKey)
	require.True(t, ok, "the thumb object must land next to the main one")

	body, size, contentType, servedKey, err := h.svc.OpenContactPhoto(context.Background(), h.owner, created.ID, true)
	require.NoError(t, err)
	defer func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	}()
	require.Equal(t, thumbKey, servedKey)
	require.Equal(t, photo.ContentTypeJPEG, contentType)
	require.Equal(t, int64(len(thumbBytes)), size)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, thumbBytes, data)
}

func TestContactService_MaterializesThumbLazily(t *testing.T) {
	t.Parallel()

	h, photos := newThumbHarness(t)
	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(nil))
	require.NoError(t, err, "create contact")

	key := "photos/" + uuid.Must(uuid.NewV7()).String() + ".jpg"
	contentType := photo.ContentTypeJPEG
	full := realContactJPEG(t, 1200, 900)
	require.NoError(t, photos.Put(context.Background(), key, bytes.NewReader(full), contentType, int64(len(full))))
	_, err = h.contacts.SetPhoto(context.Background(), created.ID, h.owner, &key, &contentType, nil)
	require.NoError(t, err, "seed photo")

	body, _, servedType, servedKey, err := h.svc.OpenContactPhoto(context.Background(), h.owner, created.ID, true)
	require.NoError(t, err)
	defer func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	}()
	require.Equal(t, photo.ThumbKeyOf(key), servedKey)
	require.Equal(t, photo.ContentTypeJPEG, servedType)

	cfg, _, err := image.DecodeConfig(io.Reader(body))
	require.NoError(t, err)
	require.Equal(t, photo.ThumbMaxEdge, max(cfg.Width, cfg.Height))

	_, ok := photos.Object(photo.ThumbKeyOf(key))
	require.True(t, ok, "the materialized thumb must be stored for the next request")
}

func TestContactService_WebPFallsBackToFull(t *testing.T) {
	t.Parallel()

	h, photos := newThumbHarness(t)
	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(nil))
	require.NoError(t, err, "create contact")

	key := "photos/" + uuid.Must(uuid.NewV7()).String() + ".webp"
	contentType := photo.ContentTypeWebP
	require.NoError(t, photos.Put(context.Background(), key, strings.NewReader("webp-bytes"), contentType, 10))
	_, err = h.contacts.SetPhoto(context.Background(), created.ID, h.owner, &key, &contentType, nil)
	require.NoError(t, err, "seed photo")

	body, _, _, servedKey, err := h.svc.OpenContactPhoto(context.Background(), h.owner, created.ID, true)
	require.NoError(t, err)
	defer func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	}()
	require.Equal(t, key, servedKey, "webp has no thumb: the full object serves")
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, []byte("webp-bytes"), data)
}

func TestContactService_HealRemovesStrayThumbToo(t *testing.T) {
	t.Parallel()

	h, photos := newThumbHarness(t)
	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(nil))
	require.NoError(t, err, "create contact")

	key := "photos/" + uuid.Must(uuid.NewV7()).String() + ".jpg"
	contentType := photo.ContentTypeJPEG
	_, err = h.contacts.SetPhoto(context.Background(), created.ID, h.owner, &key, &contentType, nil)
	require.NoError(t, err, "seed photo")

	// The main object is gone, its thumb lingers: the heal takes both.
	require.NoError(t, photos.Delete(context.Background(), key))
	require.NoError(t, photos.Put(context.Background(), photo.ThumbKeyOf(key),
		strings.NewReader("stray-thumb"), photo.ContentTypeJPEG, 11))

	opened, servedKey, err := openContactPhotoForRead(h, created.ID, true)
	require.ErrorIs(t, err, ErrPhotoNotFound)
	require.Nil(t, opened)
	require.Empty(t, servedKey)

	_, ok := photos.Object(photo.ThumbKeyOf(key))
	require.False(t, ok, "the stray thumb must die with the healed key")
}
