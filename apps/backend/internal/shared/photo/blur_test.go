package photo

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// The blur-up frame is a second derivative of the very same pixels the
// thumb seam already decodes (решение #1594, тикет #1603): a 16px jpeg
// ready to travel as base64 inside the DTOs. The tests pin the geometry,
// the opacity, the size bound and the honest degradation (webp, a frame at
// or below the bound — the column stays NULL and the client keeps its
// placeholder).

func TestBlurFromStored(t *testing.T) {
	t.Parallel()
	src := mustJPEG(t, 1200, 900)

	blur, ok := BlurFromStored(src, ContentTypeJPEG)
	if !ok {
		t.Fatal("jpeg must produce a blur frame")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(mustDecodeBase64(t, blur)))
	if err != nil {
		t.Fatalf("blur frame does not decode: %v", err)
	}
	if format != testThumbFormat || max(cfg.Width, cfg.Height) != BlurMaxEdge {
		t.Fatalf("blur = %s %dx%d, want jpeg capped at %d", format, cfg.Width, cfg.Height, BlurMaxEdge)
	}
}

func TestBlurFromStored_Png(t *testing.T) {
	t.Parallel()
	if _, ok := BlurFromStored(mustPNG(t, 800, 600), ContentTypePNG); !ok {
		t.Fatal("png must produce a blur frame")
	}
}

func TestBlurFromStored_WebpDegrades(t *testing.T) {
	t.Parallel()
	// WebP cannot decode in the Go toolchain — the honest degradation is
	// no frame, the column stays NULL (ADR 0065: webp travels undecoded).
	if blur, ok := BlurFromStored(riffWebp(t, webpVP8X(t), webpChunk(t, "VP8L", []byte{0x2F, 0x00, 0x00, 0x00})), ContentTypeWebP); ok {
		t.Fatalf("webp must report no blur, got %d bytes", len(blur))
	}
}

func TestBlurFromStored_WithinBoundIsItsOwnBlur(t *testing.T) {
	t.Parallel()
	// A frame at or below 16px needs no blur — the client already holds the
	// whole image at the circle's scale.
	if _, ok := BlurFromStored(mustJPEG(t, 16, 12), ContentTypeJPEG); ok {
		t.Fatal("a within-bound image must report no blur")
	}
	if _, ok := BlurFromStored(mustJPEG(t, 17, 12), ContentTypeJPEG); !ok {
		t.Fatal("one pixel over the bound must produce a frame")
	}
}

func TestBlurFromStored_OpaqueOverWhite(t *testing.T) {
	t.Parallel()
	// The frame is always an opaque jpeg: a transparent png must not turn
	// into a black hole at the circle's backing (the thumb's canon).
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := range 100 {
		for x := range 100 {
			src.SetRGBA(x, y, color.RGBA{A: 0})
		}
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, src); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	blur, ok := BlurFromStored(raw.Bytes(), ContentTypePNG)
	if !ok {
		t.Fatal("transparent png must still produce a frame")
	}
	frame, err := jpeg.Decode(bytes.NewReader(mustDecodeBase64(t, blur)))
	if err != nil {
		t.Fatalf("blur frame does not decode: %v", err)
	}
	if _, _, _, a := frame.At(0, 0).RGBA(); a != 0xffff {
		t.Fatalf("frame alpha = %d, want fully opaque", a)
	}
}

func TestBlurFromStored_SizeBound(t *testing.T) {
	t.Parallel()
	// The canon: ~0.5–1 КБ before base64 — the 2 КБ ceiling on the wire
	// form guards a regression that ships full-size bytes in the column.
	for _, tc := range []struct {
		name string
		src  []byte
		ct   string
	}{
		{"portrait jpeg", mustJPEG(t, 3024, 4032), ContentTypeJPEG},
		{"large png", mustPNG(t, 1600, 1200), ContentTypePNG},
	} {
		blur, ok := BlurFromStored(tc.src, tc.ct)
		if !ok {
			t.Fatalf("%s: expected a frame", tc.name)
		}
		if len(blur) > 2048 {
			t.Fatalf("%s: blur base64 = %d bytes, want ≤ 2048", tc.name, len(blur))
		}
	}
}

// Process bakes the frame into the upload seam's result: the ready base64
// rides in Processed.Blur next to the thumb, empty when there is none —
// the blur never fails the upload («the thumbnail rides with the upload»).
func TestProcess_CarriesBlur(t *testing.T) {
	t.Parallel()

	processed, err := Process(mustJPEG(t, 1200, 900))
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if processed.Blur == "" {
		t.Fatal("processed jpeg must carry the blur base64")
	}
	raw := mustDecodeBase64(t, processed.Blur)
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || format != testThumbFormat || max(cfg.Width, cfg.Height) != BlurMaxEdge {
		t.Fatalf("Blur = %s %dx%d (err %v), want jpeg capped at %d", format, cfg.Width, cfg.Height, err, BlurMaxEdge)
	}

	webpProcessed, err := Process(riffWebp(t, webpVP8X(t), webpChunk(t, "VP8L", []byte{0x2F, 0x00, 0x00, 0x00})))
	if err != nil {
		t.Fatalf("process webp: %v", err)
	}
	if webpProcessed.Blur != "" {
		t.Fatalf("webp must carry no blur, got %d bytes", len(webpProcessed.Blur))
	}

	tinyProcessed, err := Process(mustJPEG(t, 16, 12))
	if err != nil {
		t.Fatalf("process tiny: %v", err)
	}
	if tinyProcessed.Blur != "" {
		t.Fatalf("within-bound image must carry no blur, got %d bytes", len(tinyProcessed.Blur))
	}
}

func mustDecodeBase64(t *testing.T, s string) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("not valid base64: %v", err)
	}
	return raw
}
