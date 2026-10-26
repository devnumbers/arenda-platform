// The blur-up frame of the private photos (решение #1594, тикет #1603 —
// аменд ADR 0065): a second derivative artifact built from the very pixels
// the thumb seam already decodes. The frame is a long-edge-16px opaque jpeg
// (a few hundred bytes), delivered as ready base64 — the photo_blur column
// next to photo_key feeds the DTOs from the row they already selected, zero
// storage round-trips per circle. A separate ?size=blur request is rejected
// (an extra round-trip per circle, multiplied by the lists) — the frame
// must be there with the first paint.

package photo

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
)

// BlurMaxEdge caps the blur-up frame's long edge (решение #1594): 16 px at
// a mild quality is the instant color wash the CirclePhoto paints over the
// placeholder before the real thumb bytes arrive.
const BlurMaxEdge = 16

// blurQuality is the re-encode quality of the blur frame (решение #1594:
// q≈50). The banding a 16px jpeg shows is a feature — the frame is a color
// wash, not a preview; the artifacts vanish at the circle's scale.
const blurQuality = 50

// FramePossible reports whether a content type can yield a blur frame at
// all: webp never decodes in the Go toolchain — a column that would stay
// NULL forever must not turn its full-photo serve into a permanent
// in-memory detour.
func FramePossible(contentType string) bool {
	return contentType == ContentTypeJPEG || contentType == ContentTypePNG
}

// BlurFromStored builds the blur-up frame from the STORED object bytes and
// answers it as ready base64 — the exact value the photo_blur column and
// the DTO field carry (empty string means no frame). Everything in storage
// passed Process, so jpeg/png are upright and decodable; webp cannot decode
// and reports no frame, as does an image already within the bound — its own
// pixels are the blur at the circle's scale.
func BlurFromStored(data []byte, contentType string) (string, bool) {
	decoded, ok := decodeStored(data, contentType)
	if !ok {
		return "", false
	}
	return blurBase64(decoded)
}

// BlurPtr is the column form of a processed upload's frame: the ready
// base64, nil when there is none — the shape the photo write queries take.
func (p Processed) BlurPtr() *string {
	if p.Blur == "" {
		return nil
	}
	return &p.Blur
}

// blurBase64 encodes the frame from the already-decoded pixels: the same
// area downscale the thumbnail uses, then a q≈50 jpeg over white. A false
// result means "no blur" — a frame at or below the 16px bound is its own
// blur, the column stays NULL, the client keeps its placeholder.
func blurBase64(decoded image.Image) (string, bool) {
	small := downscaleArea(decoded, BlurMaxEdge)
	if small == nil {
		return "", false
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, small, &jpeg.Options{Quality: blurQuality}); err != nil {
		// The blur never fails the upload or the serve («the thumbnail
		// rides with the upload»): a frame that cannot encode degrades to
		// no frame — the column stays NULL and the next attempt retries.
		return "", false
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), true
}

// decodeStored decodes the STORED object bytes: everything in storage
// passed Process, so jpeg/png decode here — or the content type is webp,
// which the Go toolchain cannot decode and the frame seam must skip.
func decodeStored(data []byte, contentType string) (image.Image, bool) {
	switch contentType {
	case ContentTypeJPEG:
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, false
		}
		return img, true
	case ContentTypePNG:
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, false
		}
		return img, true
	default:
		return nil, false
	}
}
