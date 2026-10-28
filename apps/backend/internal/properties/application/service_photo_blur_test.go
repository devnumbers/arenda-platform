package application

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	"github.com/stretchr/testify/require"
)

// The blur-up frame's life next to the object photo (решение #1594, тикет
// #1603 — аменд ADR 0065): the upload lands the frame in the row next to
// the key; a pre-blur object materializes its frame lazily on the photo
// serve; webp gets no frame and no in-memory detour on the full serve; a
// deleted photo takes its frame with it.

func blurPhotoHarness(
	t *testing.T,
) (svc *PropertyService, repo *lockingFakePropertyRepo, storage *fakePropertyPhotoStorage, ownerID, propID uuid.UUID) {
	t.Helper()
	ownerID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	prop := photoTestProperty(ownerID)
	repo = newLockingFakePropertyRepo(prop)
	storage = newFakePropertyPhotoStorage()
	svc = NewPropertyService(
		repo,
		storage,
		newPropertyTestFactory(repo, nil),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
	return svc, repo, storage, ownerID, prop.ID
}

// blurUploadJPEG renders an upload-shaped jpeg for the Process seam.
func blurUploadJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

// blurSeedObject puts a genuinely decodable object behind the row's columns
// the way the pre-blur uploads look: a key and a content type, no frame.
func blurSeedObject(
	t *testing.T, storage *fakePropertyPhotoStorage, repo *lockingFakePropertyRepo, propID, ownerID uuid.UUID, contentType string, raw []byte,
) string {
	t.Helper()
	key := "photos/" + uuid.Must(uuid.NewV7()).String() + ".jpg"
	err := storage.Put(
		context.Background(), key, strings.NewReader(string(raw)), contentType, int64(len(raw)))
	require.NoError(t, err)
	_, err = repo.SetPropertyPhoto(context.Background(), propID, ownerID, &key, &contentType, nil)
	require.NoError(t, err)
	return key
}

func TestPropertyService_UploadCarriesBlur(t *testing.T) {
	t.Parallel()
	svc, _, _, ownerID, propID := blurPhotoHarness(t)

	processed, err := photo.Process(blurUploadJPEG(t))
	require.NoError(t, err)
	require.NotEmpty(t, processed.Blur, "test precondition: the processed upload carries a frame")

	updated, err := svc.SetPropertyPhoto(context.Background(), ownerID, propID, processed)
	require.NoError(t, err)
	require.NotNil(t, updated.PhotoBlur)
	require.Equal(t, processed.Blur, *updated.PhotoBlur)
}

func TestPropertyService_DeleteClearsBlur(t *testing.T) {
	t.Parallel()
	svc, repo, _, ownerID, propID := blurPhotoHarness(t)

	processed, err := photo.Process(blurUploadJPEG(t))
	require.NoError(t, err)
	_, err = svc.SetPropertyPhoto(context.Background(), ownerID, propID, processed)
	require.NoError(t, err)

	_, err = svc.DeletePropertyPhoto(context.Background(), ownerID, propID)
	require.NoError(t, err)
	after, err := repo.GetByIDAndOwner(context.Background(), propID, ownerID)
	require.NoError(t, err)
	require.Nil(t, after.PhotoBlur, "the frame shares the key's fate: cleared with the photo")
}

func TestPropertyService_MaterializesBlurOnFullServe(t *testing.T) {
	t.Parallel()
	svc, repo, storage, ownerID, propID := blurPhotoHarness(t)

	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	var raw bytes.Buffer
	require.NoError(t, jpeg.Encode(&raw, img, nil))
	key := blurSeedObject(t, storage, repo, propID, ownerID, photo.ContentTypeJPEG, raw.Bytes())

	body, _, _, servedKey, err := svc.OpenPropertyPhoto(context.Background(), ownerID, propID, false)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, raw.Bytes(), data, "the full serve hands out the very bytes the frame came from")
	require.Equal(t, key, servedKey)

	stored, err := repo.GetByIDAndOwner(context.Background(), propID, ownerID)
	require.NoError(t, err)
	require.NotNil(t, stored.PhotoBlur, "the full serve must materialize the frame")
}

// WebP cannot yield a frame in the Go toolchain — the column stays NULL
// forever, so the full serve must keep streaming: no in-memory detour, no
// pointless frame work (the permanently-NULL column would otherwise
// re-buffer the whole object on every request).
func TestPropertyService_WebpFullServeStreamsUntouched(t *testing.T) {
	t.Parallel()
	svc, repo, storage, ownerID, propID := blurPhotoHarness(t)

	const payload = "RIFF....WEBPVP8 ...."
	key := blurSeedObject(t, storage, repo, propID, ownerID, photo.ContentTypeWebP, []byte(payload))

	body, size, contentType, servedKey, err := svc.OpenPropertyPhoto(context.Background(), ownerID, propID, false)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, []byte(payload), data)
	require.Equal(t, int64(len(payload)), size)
	require.Equal(t, photo.ContentTypeWebP, contentType)
	require.Equal(t, key, servedKey)

	stored, err := repo.GetByIDAndOwner(context.Background(), propID, ownerID)
	require.NoError(t, err)
	require.Nil(t, stored.PhotoBlur, "webp must report no frame")
}

func TestPropertyService_MaterializesBlurOnThumbServe(t *testing.T) {
	t.Parallel()
	svc, repo, storage, ownerID, propID := blurPhotoHarness(t)

	img := image.NewRGBA(image.Rect(0, 0, 1200, 900))
	var raw bytes.Buffer
	require.NoError(t, jpeg.Encode(&raw, img, nil))
	key := blurSeedObject(t, storage, repo, propID, ownerID, photo.ContentTypeJPEG, raw.Bytes())

	body, _, contentType, servedKey, err := svc.OpenPropertyPhoto(context.Background(), ownerID, propID, true)
	require.NoError(t, err)
	_, err = io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, photo.ThumbKeyOf(key), servedKey)
	require.Equal(t, photo.ContentTypeJPEG, contentType)

	stored, err := repo.GetByIDAndOwner(context.Background(), propID, ownerID)
	require.NoError(t, err)
	require.NotNil(t, stored.PhotoBlur, "the thumb serve must materialize the frame")
}
