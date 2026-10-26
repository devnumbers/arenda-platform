package application

import (
	"bytes"
	"context"
	"encoding/base64"
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

// The blur-up frame's life next to the card photo (решение #1594, тикет
// #1603 — аменд ADR 0065): the upload lands the frame in the row next to
// the key; a pre-blur card materializes its frame lazily on the photo
// serve; a deleted card takes its frame with it.

// blurPhotoSeed puts a genuinely decodable jpeg behind the card's columns
// the way the pre-blur uploads look: a key and a content type, no frame.
func blurPhotoSeed(t *testing.T, h *serviceHarness, owner, contactID uuid.UUID, w, hgt int) (key string, full []byte) {
	t.Helper()
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
	key = "photos/" + uuid.Must(uuid.NewV7()).String() + ".jpg"
	img := image.NewRGBA(image.Rect(0, 0, w, hgt))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	full = buf.Bytes()
	require.NoError(t, photos.Put(context.Background(), key, strings.NewReader(string(full)), photo.ContentTypeJPEG, int64(len(full))))
	contentType := photo.ContentTypeJPEG
	_, err := h.contacts.SetPhoto(context.Background(), contactID, owner, &key, &contentType, nil)
	require.NoError(t, err)
	return key, full
}

func TestContactService_UploadCarriesBlur(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(nil))
	require.NoError(t, err)

	processed, err := photo.Process(blurUploadJPEG(t))
	require.NoError(t, err)
	require.NotEmpty(t, processed.Blur, "test precondition: the processed upload carries a frame")

	updated, err := h.svc.SetContactPhoto(context.Background(), h.owner, created.ID, processed)
	require.NoError(t, err)
	require.NotNil(t, updated.PhotoBlur)
	require.Equal(t, processed.Blur, *updated.PhotoBlur)
}

func TestContactService_DeleteClearsBlur(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(nil))
	require.NoError(t, err)
	processed, err := photo.Process(blurUploadJPEG(t))
	require.NoError(t, err)
	_, err = h.svc.SetContactPhoto(context.Background(), h.owner, created.ID, processed)
	require.NoError(t, err)

	_, err = h.svc.DeleteContactPhoto(context.Background(), h.owner, created.ID)
	require.NoError(t, err)
	after, err := h.contacts.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	require.Nil(t, after.PhotoBlur, "the frame shares the key's fate: cleared with the photo")
}

func TestContactService_MaterializesBlurOnThumbServe(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(nil))
	require.NoError(t, err)
	key, _ := blurPhotoSeed(t, h, h.owner, created.ID, 1200, 900)

	body, _, contentType, servedKey, err := h.svc.OpenContactPhoto(context.Background(), h.owner, created.ID, true)
	require.NoError(t, err)
	_, err = io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, photo.ThumbKeyOf(key), servedKey)
	require.Equal(t, photo.ContentTypeJPEG, contentType)

	stored, err := h.contacts.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.PhotoBlur, "the thumb serve must materialize the frame")
	raw, err := base64.StdEncoding.DecodeString(*stored.PhotoBlur)
	require.NoError(t, err)
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	require.NoError(t, err)
	require.Equal(t, "jpeg", format)
	require.LessOrEqual(t, max(cfg.Width, cfg.Height), photo.BlurMaxEdge)
}

func TestContactService_MaterializesBlurOnFullServe(t *testing.T) {
	t.Parallel()
	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	created, err := h.svc.CreateContact(context.Background(), h.owner, createCmd(nil))
	require.NoError(t, err)
	key, full := blurPhotoSeed(t, h, h.owner, created.ID, 200, 100)

	body, _, _, servedKey, err := h.svc.OpenContactPhoto(context.Background(), h.owner, created.ID, false)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, full, data, "the full serve hands out the very bytes the frame came from")
	require.Equal(t, key, servedKey)

	stored, err := h.contacts.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.PhotoBlur, "the full serve must materialize the frame")
}

// blurUploadJPEG renders an upload-shaped jpeg for the Process seam.
func blurUploadJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}
