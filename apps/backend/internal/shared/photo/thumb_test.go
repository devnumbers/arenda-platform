package photo

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

// The downscaler is pure pixel math, so the tests pin exact values: an area
// filter averages the covered source box, premultiplied over white (avatar
// circles sit on white/muted backing, and the thumb is always an opaque
// jpeg).

func solidImage(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestDownscaleArea_SolidStaysSolid(t *testing.T) {
	t.Parallel()
	red := color.RGBA{R: 200, G: 40, B: 60, A: 255}
	dst := downscaleArea(solidImage(100, 100, red), 10)
	if dst == nil {
		t.Fatal("downscale returned nil")
	}
	b := dst.Bounds()
	if b.Dx() != 10 || b.Dy() != 10 {
		t.Fatalf("size = %dx%d, want 10x10", b.Dx(), b.Dy())
	}
	for y := range b.Dy() {
		for x := range b.Dx() {
			if got := dst.RGBAAt(x, y); got != (color.RGBA{R: 200, G: 40, B: 60, A: 255}) {
				t.Fatalf("pixel(%d,%d) = %+v, want the solid red", x, y, got)
			}
		}
	}
}

func TestDownscaleArea_AveragesColors(t *testing.T) {
	t.Parallel()
	// 2x1: black | white → 1x1: the exact midpoint.
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, color.Black)
	src.Set(1, 0, color.White)
	dst := downscaleArea(src, 1)
	if got := dst.RGBAAt(0, 0); got.R != 128 || got.G != 128 || got.B != 128 {
		t.Fatalf("pixel = %+v, want the exact black/white midpoint", got)
	}
}

func TestDownscaleArea_FractionalCoverage(t *testing.T) {
	t.Parallel()
	// 3x1: black | black | white → 1x1: 2/3 black + 1/3 white.
	src := image.NewRGBA(image.Rect(0, 0, 3, 1))
	src.Set(0, 0, color.Black)
	src.Set(1, 0, color.Black)
	src.Set(2, 0, color.White)
	dst := downscaleArea(src, 1)
	if got := dst.RGBAAt(0, 0); got.R != 85 || got.G != 85 || got.B != 85 { // 255 * 1/3
		t.Fatalf("pixel = %+v, want 85 (one third of white)", got)
	}
}

func TestDownscaleArea_KeepsAspectLongEdge(t *testing.T) {
	t.Parallel()
	dst := downscaleArea(solidImage(3024, 4032, color.White), 320)
	if dst == nil {
		t.Fatal("downscale returned nil")
	}
	b := dst.Bounds()
	if b.Dx() != 240 || b.Dy() != 320 {
		t.Fatalf("size = %dx%d, want 240x320 (long edge capped)", b.Dx(), b.Dy())
	}

	dst = downscaleArea(solidImage(4032, 3024, color.White), 320)
	if dst == nil {
		t.Fatal("downscale returned nil")
	}
	if b := dst.Bounds(); b.Dx() != 320 || b.Dy() != 240 {
		t.Fatalf("landscape size = %dx%d, want 320x240", b.Dx(), b.Dy())
	}
}

func TestDownscaleArea_SmallImageSkipped(t *testing.T) {
	t.Parallel()
	// An upload already within the thumb bound needs no thumb: nil means
	// "no thumbnail", the serve path falls back to the full object.
	if dst := downscaleArea(solidImage(320, 200, color.White), ThumbMaxEdge); dst != nil {
		t.Fatalf("downscale of a within-bound image = %+v, want nil", dst)
	}
	if dst := downscaleArea(solidImage(321, 200, color.White), ThumbMaxEdge); dst == nil {
		t.Fatal("one pixel over the bound must produce a thumb")
	}
}

func TestDownscaleArea_AlphaCompositedOverWhite(t *testing.T) {
	t.Parallel()
	// Left half fully transparent, right half fully black → the thumb is
	// the horizontal midpoint of {white, black}: an opaque gray, never a
	// black hole or a premultiplied smear.
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.Set(0, 0, color.RGBA{A: 0})
	src.Set(1, 0, color.Black)
	dst := downscaleArea(src, 1)
	got := dst.RGBAAt(0, 0)
	if got.A != 255 {
		t.Fatalf("alpha = %d, want 255 (thumb is opaque)", got.A)
	}
	if got.R != 128 || got.G != 128 || got.B != 128 {
		t.Fatalf("pixel = %+v, want transparent-over-white midpoint gray", got)
	}
}

func TestThumbKeyOf(t *testing.T) {
	t.Parallel()
	if got := ThumbKeyOf("photos/01987654-3210-7abc-9def-0123456789ab.jpg"); got != "photos/01987654-3210-7abc-9def-0123456789ab_t.jpg" {
		t.Fatalf("ThumbKeyOf = %q", got)
	}
}

// ThumbFromStored materializes a thumb from the STORED bytes (every object
// in storage passed Process: upright, stripped, jpeg/png decodable — or webp,
// which cannot decode and reports no thumb; the serve path then falls back
// to the full object).
func TestThumbFromStored(t *testing.T) {
	t.Parallel()
	src := mustJPEG(t, 1200, 900)

	thumb, ok := ThumbFromStored(src, ContentTypeJPEG)
	if !ok {
		t.Fatal("jpeg must produce a thumb")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("thumb does not decode: %v", err)
	}
	if format != "jpeg" || max(cfg.Width, cfg.Height) != ThumbMaxEdge {
		t.Fatalf("thumb = %s %dx%d, want jpeg capped at %d", format, cfg.Width, cfg.Height, ThumbMaxEdge)
	}

	if _, ok := ThumbFromStored(src, ContentTypeWebP); ok {
		t.Fatal("webp must report no thumb")
	}

	// A within-bound stored image needs no thumb either.
	small := mustJPEG(t, 200, 100)
	if _, ok := ThumbFromStored(small, ContentTypeJPEG); ok {
		t.Fatal("within-bound image must report no thumb")
	}
}
