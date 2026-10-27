package photo

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"math"
	"path/filepath"
	"strings"

	storageshared "github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// ThumbMaxEdge caps the thumbnail's long edge (ADR 0065, аменд — тикет
// миниатюр): 320 px covers the largest avatar circle (96 CSS px) at the 3x
// retina tier; avatars never render larger, the full object stays available
// through the same stream without the size parameter.
const ThumbMaxEdge = 320

// ThumbKeyOf derives the thumbnail's storage key from the main one (ADR
// 0065, тикет миниатюр): the same UUID with the _t infix — deterministic,
// so the serving ETag of a thumb is computable without touching storage and
// the 304 path stays storage-free. The thumb object's lifetime belongs to
// the main key: replacement and deletion always take both.
func ThumbKeyOf(key string) string {
	ext := filepath.Ext(key)
	return strings.TrimSuffix(key, ext) + "_t" + ext
}

// ThumbFromStored builds a thumbnail from the STORED object bytes (the lazy
// materialization of the pre-thumb uploads): everything in storage passed
// Process, so jpeg/png are upright and decodable; webp cannot decode and
// reports no thumb, as does an image already within the bound — the serve
// path falls back to the full object for those.
func ThumbFromStored(data []byte, contentType string) ([]byte, bool) {
	decoded, ok := decodeStored(data, contentType)
	if !ok {
		return nil, false
	}
	return thumbFromDecoded(decoded)
}

// thumbFromDecoded encodes the thumbnail from the already-decoded pixels:
// the area downscale, then the re-encode quality. A false result means "no
// thumbnail" — a within-bound image or a frame that cannot encode.
func thumbFromDecoded(decoded image.Image) ([]byte, bool) {
	small := downscaleArea(decoded, ThumbMaxEdge)
	if small == nil {
		return nil, false
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, small, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// downscaleArea shrinks src so its long edge fits maxEdge, area-weighted:
// every destination pixel averages the source box it covers, with
// fractional edge coverage. Large factors are exactly where area averaging
// beats interpolation — one full-res pass, no ringing, no resampling
// artifacts. Pixels composite over white first (avatar circles sit on the
// white/muted CircleIcon backing, and the thumb is always an opaque jpeg —
// a transparent png must not turn into a black hole). An image already
// within the bound needs no thumb: nil means "no thumbnail", the serve path
// falls back to the full object.
func downscaleArea(src image.Image, maxEdge int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw == 0 || sh == 0 {
		return nil
	}
	longest := max(sw, sh)
	if longest <= maxEdge {
		return nil
	}
	dw := max(1, sw*maxEdge/longest)
	dh := max(1, sh*maxEdge/longest)

	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for dy := range dh {
		y0, y1 := float64(dy)*float64(sh)/float64(dh), float64(dy+1)*float64(sh)/float64(dh)
		for dx := range dw {
			x0, x1 := float64(dx)*float64(sw)/float64(dw), float64(dx+1)*float64(sw)/float64(dw)
			var rSum, gSum, bSum, aSum, area float64
			for sy := int(y0); sy < sh && float64(sy) < y1; sy++ {
				h := math.Min(y1, float64(sy+1)) - math.Max(y0, float64(sy))
				for sx := int(x0); sx < sw && float64(sx) < x1; sx++ {
					w := math.Min(x1, float64(sx+1)) - math.Max(x0, float64(sx))
					// At().RGBA() is 16-bit alpha-premultiplied — the >>8 halves
					// are exactly color.RGBA's 8-bit premultiplied fields.
					r16, g16, b16, a16 := src.At(sb.Min.X+sx, sb.Min.Y+sy).RGBA()
					px := color.RGBA{
						R: uint8(r16 >> 8 & 0xff), G: uint8(g16 >> 8 & 0xff),
						B: uint8(b16 >> 8 & 0xff), A: uint8(a16 >> 8 & 0xff),
					}
					wa := w * h * (float64(px.A) / 255)
					rSum += wa * float64(px.R)
					gSum += wa * float64(px.G)
					bSum += wa * float64(px.B)
					aSum += wa
					area += w * h
				}
			}
			// The box average of the source composited over white:
			// channel = avg(a·channel) + 255·avg(1−a) — the transparent
			// share of every pixel shows white, the opaque one its color.
			dst.SetRGBA(dx, dy, color.RGBA{
				R: overWhite(rSum, aSum, area),
				G: overWhite(gSum, aSum, area),
				B: overWhite(bSum, aSum, area),
				A: 255,
			})
		}
	}
	return dst
}

// overWhite folds the box-average alpha into the premultiplied channel sum:
// avg(a·c) + 255·(1 − avg(a)).
func overWhite(sum, alphaSum, area float64) uint8 {
	if area == 0 {
		return 255
	}
	v := sum/area + 255*(1-alphaSum/area)
	return uint8(math.Round(math.Min(v, 255)))
}

// ThumbETagOf returns the serving ETag of the thumbnail derived from the
// main key: computable without touching storage, so a matching
// If-None-Match answers 304 storage-free. The thumb bytes are a
// deterministic function of the main object's bytes — a 304 never lies even
// when the thumb object itself needs re-materialization.
func ThumbETagOf(mainKey string) string {
	return storageshared.ETagOf(ThumbKeyOf(mainKey))
}

// ThumbStore is the minimal storage surface the thumb serving needs — the
// services' shared PhotoStorage port satisfies it.
type ThumbStore interface {
	Open(ctx context.Context, key string) (body io.ReadCloser, size int64, contentType string, err error)
	Put(ctx context.Context, key string, data io.Reader, contentType string, size int64) error
}

// ServeOrMaterializeThumb answers a thumb request given the OPEN main
// object and ALWAYS returns the body to serve: the stored thumb untouched
// (the main body is closed then, isThumb=true), the freshly materialized
// thumb (stored best-effort — a failed Put still serves, the next request
// retries), or the full bytes re-wrapped when no thumb is possible (webp, a
// within-bound image) — isThumb=false, the caller serves the full body it
// already holds. The full body is closed on every consumed path.
//
// The blurSink argument (тикет #1603, nil when the caller's blur column
// is already filled) receives the blur-up frame built from the very decode the serving
// runs: a materialization reports the frame it built from the main bytes it
// already read; a stored-thumb hit reads the main bytes once for the frame
// (jpeg/png only — webp never decodes); a failed frame costs nothing — the
// column stays NULL and the next request retries. The sink owns its error
// policy: the write is best-effort on the caller's side, a failed write is
// logged there and never fails the serve.
func ServeOrMaterializeThumb(
	ctx context.Context, store ThumbStore, mainKey, contentType string, full io.ReadCloser, log *slog.Logger,
	blurSink func(base64 string),
) (body io.ReadCloser, size int64, isThumb bool, err error) {
	if contentType != ContentTypeJPEG && contentType != ContentTypePNG {
		// An undecorable format: the full body must stay untouched — nil
		// tells the caller to serve the body it already holds.
		return nil, 0, false, nil
	}
	// The consumed paths close the full body at return, best-effort: the
	// response bytes never depend on a close outcome (the thumb is already
	// read, or the body is fully in memory). The undecorable path above
	// returns first, so the caller keeps the open body there.
	defer func() {
		if cerr := full.Close(); cerr != nil {
			log.WarnContext(ctx, "failed to close photo main body",
				slog.String("key", mainKey), slog.String("error", cerr.Error()))
		}
	}()
	if tb, ts, _, err := store.Open(ctx, ThumbKeyOf(mainKey)); err == nil {
		if blurSink != nil {
			// The stored thumb answers this request; the frame still needs
			// the main bytes — one read, one decode, then the sink carries
			// it to the column. A failed read only skips the frame: the
			// thumb serves either way, the next request retries.
			if data, rerr := io.ReadAll(full); rerr == nil {
				if frame, ok := BlurFromStored(data, contentType); ok {
					blurSink(frame)
				}
			} else {
				log.WarnContext(ctx, "failed to read photo main bytes for the blur frame",
					slog.String("key", mainKey), slog.String("error", rerr.Error()))
			}
		}
		return tb, ts, true, nil
	}
	data, err := io.ReadAll(full)
	if err != nil {
		return nil, 0, false, fmt.Errorf("read photo: %w", err)
	}
	// One decode feeds both artifacts: the thumb and the blur frame are the
	// same pixels at two scales (тикет #1603 — «за тот же декод»).
	decoded, decodable := decodeStored(data, contentType)
	if !decodable {
		return io.NopCloser(bytes.NewReader(data)), int64(len(data)), false, nil
	}
	if blurSink != nil {
		if frame, ok := blurBase64(decoded); ok {
			blurSink(frame)
		}
	}
	thumb, buildable := thumbFromDecoded(decoded)
	if !buildable {
		return io.NopCloser(bytes.NewReader(data)), int64(len(data)), false, nil
	}
	thumbKey := ThumbKeyOf(mainKey)
	if err := store.Put(ctx, thumbKey, bytes.NewReader(thumb), ContentTypeJPEG, int64(len(thumb))); err != nil {
		// Storing the thumb is an optimization, not a correctness step: the
		// fresh thumb still serves, the next request retries the Put.
		log.WarnContext(ctx, "failed to store materialized photo thumb",
			slog.String("key", thumbKey),
			slog.String("error", err.Error()),
		)
	}
	return io.NopCloser(bytes.NewReader(thumb)), int64(len(thumb)), true, nil
}
