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

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	"github.com/stretchr/testify/require"
)

// The blur-up frame's life next to the profile photo (решение #1594, тикет
// #1603 — аменд ADR 0065): the upload lands the frame in the row next to
// the key; a pre-blur photo materializes its frame lazily on the photo
// serve — thumb and full variants alike — and a deleted photo takes its
// frame with it.

// realTestBlurJPEG renders a decodable jpeg big enough to carry a frame.
func realTestBlurJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	return realTestJPEG(t, w, h)
}

func decodeFrame(t *testing.T, b64 string) image.Config {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(b64)
	require.NoError(t, err)
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	require.NoError(t, err)
	require.Equal(t, "jpeg", format)
	return cfg
}

func TestProfileService_UploadCarriesBlur(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedProfileUser(t, h.users)

	blur, err := photo.Process(mustBlurUpload(t))
	require.NoError(t, err)
	require.NotEmpty(t, blur.Blur, "test precondition: the processed upload carries a frame")
	frame := blur.Blur

	updated, err := h.svc.SetProfilePhoto(context.Background(), user.ID, photo.Processed{
		Data:        []byte("jpeg-bytes"),
		ContentType: photo.ContentTypeJPEG,
		Size:        10,
		Blur:        frame,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.PhotoBlur)
	require.Equal(t, frame, *updated.PhotoBlur)
}

func TestProfileService_DeleteClearsBlur(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedProfileUser(t, h.users)
	_, err := h.svc.SetProfilePhoto(context.Background(), user.ID, photo.Processed{
		Data: []byte("jpeg-bytes"), ContentType: photo.ContentTypeJPEG, Size: 10, Blur: "ZmFrZQ==",
	})
	require.NoError(t, err)

	_, err = h.svc.DeleteProfilePhoto(context.Background(), user.ID)
	require.NoError(t, err)
	after, err := h.users.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.Nil(t, after.PhotoBlur, "the frame shares the key's fate: cleared with the photo")
}

// A pre-blur upload (the seed's columns predate the frame) materializes its
// blur on the thumb serve: the row gains the ready base64, the column reads
// come from the row ever after.
func TestProfileService_MaterializesBlurOnThumbServe(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)

	key := *user.PhotoKey
	require.NoError(t, h.photos.Delete(context.Background(), key))
	full := realTestBlurJPEG(t, 1200, 900)
	require.NoError(t, h.photos.Put(context.Background(), key, strings.NewReader(string(full)), photo.ContentTypeJPEG, int64(len(full))))

	opened, err := h.svc.OpenPhoto(context.Background(), user.ID, user.ID, true)
	require.NoError(t, err)
	require.Equal(t, photo.ThumbKeyOf(key), opened.Key)
	_, err = io.ReadAll(opened.Body)
	require.NoError(t, err)

	stored, err := h.users.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.PhotoBlur, "the thumb serve must materialize the frame")
	cfg := decodeFrame(t, *stored.PhotoBlur)
	require.LessOrEqual(t, max(cfg.Width, cfg.Height), photo.BlurMaxEdge)
}

// The full-object serve materializes the frame the same way — a
// within-thumb-bound image never gets a thumb object, its frame still lands
// from the serve's own bytes.
func TestProfileService_MaterializesBlurOnFullServe(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)

	key := *user.PhotoKey
	require.NoError(t, h.photos.Delete(context.Background(), key))
	full := realTestBlurJPEG(t, 200, 100)
	require.NoError(t, h.photos.Put(context.Background(), key, strings.NewReader(string(full)), photo.ContentTypeJPEG, int64(len(full))))

	opened, err := h.svc.OpenPhoto(context.Background(), user.ID, user.ID, false)
	require.NoError(t, err)
	data, err := io.ReadAll(opened.Body)
	require.NoError(t, err)
	require.Equal(t, full, data, "the full serve hands out the very bytes the frame came from")

	stored, err := h.users.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.PhotoBlur, "the full serve must materialize the frame")
	cfg := decodeFrame(t, *stored.PhotoBlur)
	require.LessOrEqual(t, max(cfg.Width, cfg.Height), photo.BlurMaxEdge)
}

// A frame that already lives in the row is never recomputed: the serve
// passes no sink, the bytes stream untouched.
func TestProfileService_FilledBlurIsNotRecomputed(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)

	key := *user.PhotoKey
	require.NoError(t, h.photos.Delete(context.Background(), key))
	full := realTestBlurJPEG(t, 1200, 900)
	require.NoError(t, h.photos.Put(context.Background(), key, strings.NewReader(string(full)), photo.ContentTypeJPEG, int64(len(full))))
	_, err := h.users.SetPhotoBlurIfKey(context.Background(), user.ID, key, "cHJlZmlsbGVk")
	require.NoError(t, err)

	opened, err := h.svc.OpenPhoto(context.Background(), user.ID, user.ID, false)
	require.NoError(t, err)
	data, err := io.ReadAll(opened.Body)
	require.NoError(t, err)
	require.Equal(t, full, data)

	stored, err := h.users.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.PhotoBlur)
	require.Equal(t, "cHJlZmlsbGVk", *stored.PhotoBlur, "the filled frame must survive the serve untouched")
}

// mustBlurUpload renders an upload-shaped jpeg for the Process seam.
func mustBlurUpload(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode upload jpeg: %v", err)
	}
	return buf.Bytes()
}

// guard: the fake's race verdict keeps the frame keyed to its photo.
func TestFakeUserRepo_SetPhotoBlurIfKeyGuardsKey(t *testing.T) {
	t.Parallel()
	h := newProfileHarness(t)
	user := seedPhotoUser(t, h)

	rows, err := h.users.SetPhotoBlurIfKey(context.Background(), user.ID, "photos/stale.jpg", "Zg==")
	require.NoError(t, err)
	require.Zero(t, rows, "a foreign key must not stamp the row")

	rows, err = h.users.SetPhotoBlurIfKey(context.Background(), user.ID, *user.PhotoKey, "Zg==")
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	// A cleared photo takes no frame either.
	_, err = h.users.ClearPhotoIfKey(context.Background(), user.ID, *user.PhotoKey)
	require.NoError(t, err)
	rows, err = h.users.SetPhotoBlurIfKey(context.Background(), user.ID, *user.PhotoKey, "Zg==")
	require.NoError(t, err)
	require.Zero(t, rows)
}
