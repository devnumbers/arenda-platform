package application

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"strings"
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	"github.com/stretchr/testify/require"
)

// The thumbnail's life next to the profile photo (ADR 0065, тикет
// миниатюр): the upload of a processed upload carrying a thumb lands BOTH
// objects; the thumb-requesting open serves the thumb bytes under the thumb
// key; a pre-thumb object materializes its thumb lazily; a webp falls back
// to the full object; a healed dangling key takes its stray thumb with it.

// realTestJPEG renders a genuinely decodable jpeg — the lazy
// materialization branches on decodability.
func realTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	return buf.Bytes()
}

func TestProfileService_StoresAndServesThumb(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedProfileUser(t, h.users)

	processed := photo.Processed{
		Data:        []byte("jpeg-bytes"),
		ContentType: photo.ContentTypeJPEG,
		Size:        10,
		Thumb:       []byte("thumb-bytes"),
	}
	updated, err := h.svc.SetProfilePhoto(context.Background(), user.ID, processed)
	require.NoError(t, err)

	thumbKey := photo.ThumbKeyOf(*updated.PhotoKey)
	thumbBytes, ok := h.photos.Object(thumbKey)
	require.True(t, ok, "the thumb object must land next to the main one")

	opened, err := h.svc.OpenPhoto(context.Background(), user.ID, user.ID, true)
	require.NoError(t, err)
	require.Equal(t, thumbKey, opened.Key)
	require.Equal(t, photo.ContentTypeJPEG, opened.ContentType)
	data, err := io.ReadAll(opened.Body)
	require.NoError(t, err)
	require.Equal(t, thumbBytes, data)
}

func TestProfileService_MaterializesThumbLazily(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)

	// The seed object is "jpeg-bytes" — undecodable. Replace it with a real
	// jpeg to play the pre-thumb upload: the columns advertise a photo whose
	// storage object predates the thumbnails.
	key := *user.PhotoKey
	require.NoError(t, h.photos.Delete(context.Background(), key))
	full := realTestJPEG(t, 1200, 900)
	require.NoError(t, h.photos.Put(context.Background(), key, strings.NewReader(string(full)), photo.ContentTypeJPEG, int64(len(full))))

	opened, err := h.svc.OpenPhoto(context.Background(), user.ID, user.ID, true)
	require.NoError(t, err)
	require.Equal(t, photo.ThumbKeyOf(key), opened.Key)

	cfg, format, err := image.DecodeConfig(bytes.NewReader(mustRead(t, opened.Body)))
	require.NoError(t, err)
	require.Equal(t, "jpeg", format)
	require.Equal(t, photo.ThumbMaxEdge, max(cfg.Width, cfg.Height))

	_, ok := h.photos.Object(photo.ThumbKeyOf(key))
	require.True(t, ok, "the materialized thumb must be stored for the next request")
}

func TestProfileService_WebPFallsBackToFull(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)

	require.NoError(t, h.photos.Delete(context.Background(), *user.PhotoKey))
	require.NoError(t, h.photos.Put(context.Background(), *user.PhotoKey,
		strings.NewReader("webp-bytes"), photo.ContentTypeWebP, 10))
	user.PhotoContentType = new(photo.ContentTypeWebP)
	h.users.byEmail[user.Email.String()] = user

	opened, err := h.svc.OpenPhoto(context.Background(), user.ID, user.ID, true)
	require.NoError(t, err)
	require.Equal(t, *user.PhotoKey, opened.Key, "webp has no thumb: the full object serves")
	data, err := io.ReadAll(opened.Body)
	require.NoError(t, err)
	require.Equal(t, []byte("webp-bytes"), data)
}

func TestProfileService_HealRemovesStrayThumbToo(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)

	require.NoError(t, h.photos.Delete(context.Background(), *user.PhotoKey))
	require.NoError(t, h.photos.Put(context.Background(),
		photo.ThumbKeyOf(*user.PhotoKey), strings.NewReader("stray-thumb"), photo.ContentTypeJPEG, 11))

	opened, err := h.svc.OpenPhoto(context.Background(), user.ID, user.ID, true)
	require.ErrorIs(t, err, ErrPhotoNotFound)
	require.Nil(t, opened.Body)

	_, ok := h.photos.Object(photo.ThumbKeyOf(*user.PhotoKey))
	require.False(t, ok, "the stray thumb must die with the healed key")
}

// mustRead drains a body for the assertions.
func mustRead(t *testing.T, body io.ReadCloser) []byte {
	t.Helper()
	defer func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close body: %v", closeErr)
		}
	}()
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	return data
}
