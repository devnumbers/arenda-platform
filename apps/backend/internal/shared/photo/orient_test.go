package photo

import (
	"image"
	"image/color"
	"testing"
)

// orientTestSource builds a 3x2 NRGBA whose every pixel is unique and
// recognizable: R carries the column, G carries the row.
func orientTestSource() *image.NRGBA {
	cols := [3]uint8{60, 120, 180}
	rows := [2]uint8{120, 240}
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for x := range 3 {
		for y := range 2 {
			img.SetNRGBA(x, y, color.NRGBA{R: cols[x], G: rows[y], B: 7, A: 255})
		}
	}
	return img
}

func TestApplyOrientationPixelMaps(t *testing.T) {
	t.Parallel()

	// The expected[o][dy][dx] map is the source coordinate the baked image must
	// show at (dx, dy) — hand-derived from the EXIF orientation
	// definitions, not from the implementation.
	expected := map[int][][][2]int{
		// Mirror horizontal: row reversed, columns intact.
		2: {
			{{2, 0}, {1, 0}, {0, 0}},
			{{2, 1}, {1, 1}, {0, 1}},
		},
		// Rotate 180: both axes reversed.
		3: {
			{{2, 1}, {1, 1}, {0, 1}},
			{{2, 0}, {1, 0}, {0, 0}},
		},
		// Mirror vertical: rows reversed, columns intact.
		4: {
			{{0, 1}, {1, 1}, {2, 1}},
			{{0, 0}, {1, 0}, {2, 0}},
		},
		// Transpose: (x, y) → (y, x), dims swap to 2x3.
		5: {
			{{0, 0}, {0, 1}},
			{{1, 0}, {1, 1}},
			{{2, 0}, {2, 1}},
		},
		// Rotate 90° CW: dims swap to 2x3, the raw left column becomes the top row.
		6: {
			{{0, 1}, {0, 0}},
			{{1, 1}, {1, 0}},
			{{2, 1}, {2, 0}},
		},
		// Transverse: dims swap to 2x3.
		7: {
			{{2, 1}, {2, 0}},
			{{1, 1}, {1, 0}},
			{{0, 1}, {0, 0}},
		},
		// Rotate 270° CW: dims swap to 2x3, the raw left column becomes the bottom row.
		8: {
			{{2, 0}, {2, 1}},
			{{1, 0}, {1, 1}},
			{{0, 0}, {0, 1}},
		},
	}

	src := orientTestSource()
	for o, rows := range expected {
		got := applyOrientation(src, o)
		dh := len(rows)
		dw := len(rows[0])
		if b := got.Bounds(); b.Dx() != dw || b.Dy() != dh {
			t.Errorf("orientation %d: dims = %dx%d, want %dx%d", o, b.Dx(), b.Dy(), dw, dh)
			continue
		}
		for y := range dh {
			for x := range dw {
				sx, sy := rows[y][x][0], rows[y][x][1]
				want := src.NRGBAAt(sx, sy)
				if g := color.NRGBAModel.Convert(got.At(x, y)); g != want {
					t.Errorf("orientation %d: pixel (%d,%d) = %v, want %v (source (%d,%d))", o, x, y, g, want, sx, sy)
				}
			}
		}
	}

	// An unknown or missing orientation is a no-op returning the same image.
	for _, o := range []int{0, 1, 9, -1} {
		if got := applyOrientation(src, o); got != image.Image(src) {
			t.Errorf("orientation %d: image must pass through untouched", o)
		}
	}
}

// fillYCbCr fills every plane byte with an arbitrary deterministic pattern.
func fillYCbCr(img *image.YCbCr) {
	for i := range img.Y {
		img.Y[i] = byte((3*i + 7) % 251)
	}
	for i := range img.Cb {
		img.Cb[i] = byte((5*i + 11) % 251)
	}
	for i := range img.Cr {
		img.Cr[i] = byte((7*i + 13) % 251)
	}
}

func TestOrientYCbCrPlaneGeometry(t *testing.T) {
	t.Parallel()

	// The plane fast path must move every plane byte exactly where the
	// orientation mapping says, for every subsample ratio the jpeg decoder
	// emits and every orientation.
	ratios := []image.YCbCrSubsampleRatio{
		image.YCbCrSubsampleRatio444,
		image.YCbCrSubsampleRatio422,
		image.YCbCrSubsampleRatio420,
		image.YCbCrSubsampleRatio440,
		image.YCbCrSubsampleRatio411,
		image.YCbCrSubsampleRatio410,
	}
	for _, ratio := range ratios {
		t.Run(ratio.String(), func(t *testing.T) {
			t.Parallel()
			assertOrientYCbCrRatio(t, ratio)
		})
	}
}

// assertOrientYCbCrRatio checks every orientation of one subsample ratio on
// an odd-sized 5x3 raster (the chroma plane of a 5x3 luma is 3x2 under
// 4:2:0). Plane indices are checked directly — a color comparison would
// drown the geometry in 8-bit quantization noise, which the strip's
// re-encode absorbs anyway.
func assertOrientYCbCrRatio(t *testing.T, ratio image.YCbCrSubsampleRatio) {
	t.Helper()

	w, h := 5, 3
	src := image.NewYCbCr(image.Rect(0, 0, w, h), ratio)
	fillYCbCr(src)
	cw, ch := chromaDims(w, h, ratio)
	for o := 2; o <= 8; o++ {
		assertOrientYCbCrOrientation(t, src, ratio, cw, ch, o)
	}
}

func assertOrientYCbCrOrientation(t *testing.T, src *image.YCbCr, ratio image.YCbCrSubsampleRatio, cw, ch, o int) {
	t.Helper()

	dstRatio, representable := rotatedYCbCrRatio(ratio, o)
	dst := orientYCbCr(src, o)
	w, h := src.Rect.Dx(), src.Rect.Dy()
	dw, dh := orientedDims(w, h, o)
	if !representable {
		// A quarter turn of 4:1:1 / 4:1:0 has no stdlib label: the
		// generic RGBA path takes over.
		if _, isYCbCr := dst.(*image.YCbCr); isYCbCr {
			t.Fatalf("orientation %d: expected the generic RGBA fallback", o)
		}
		return
	}
	ycbcr, ok := dst.(*image.YCbCr)
	if !ok {
		t.Fatalf("orientation %d: output = %T, want *image.YCbCr", o, dst)
	}
	if ycbcr.SubsampleRatio != dstRatio {
		t.Fatalf("orientation %d: label = %v, want %v", o, ycbcr.SubsampleRatio, dstRatio)
	}
	if b := ycbcr.Bounds(); b.Dx() != dw || b.Dy() != dh {
		t.Fatalf("orientation %d: dims %v, want %dx%d", o, b, dw, dh)
	}
	for y := range dh {
		for x := range dw {
			sx, sy := orientedSource(x, y, w, h, o)
			if ycbcr.Y[y*ycbcr.YStride+x] != src.Y[sy*src.YStride+sx] {
				t.Fatalf("orientation %d: Y(%d,%d) mismatch", o, x, y)
			}
		}
	}
	dcw, dch := chromaDims(dw, dh, dstRatio)
	for y := range dch {
		for x := range dcw {
			sx, sy := orientedSource(x, y, cw, ch, o)
			if ycbcr.Cb[y*ycbcr.CStride+x] != src.Cb[sy*src.CStride+sx] {
				t.Fatalf("orientation %d: Cb(%d,%d) mismatch", o, x, y)
			}
			if ycbcr.Cr[y*ycbcr.CStride+x] != src.Cr[sy*src.CStride+sx] {
				t.Fatalf("orientation %d: Cr(%d,%d) mismatch", o, x, y)
			}
		}
	}
}

func TestOrientGray(t *testing.T) {
	t.Parallel()

	src := image.NewGray(image.Rect(0, 0, 3, 2))
	for i := range src.Pix {
		src.Pix[i] = byte(11 * i)
	}
	got := applyOrientation(src, 6)
	if b := got.Bounds(); b.Dx() != 2 || b.Dy() != 3 {
		t.Fatalf("dims = %dx%d, want 2x3", b.Dx(), b.Dy())
	}
	// The raw left column becomes the top row under rotate 90° CW.
	for x := range 2 {
		for y := range 3 {
			sx, sy := orientedSource(x, y, 3, 2, 6)
			if g := got.At(x, y); g != src.At(sx, sy) {
				t.Errorf("pixel (%d,%d) = %v, want source (%d,%d) = %v", x, y, g, sx, sy, src.At(sx, sy))
			}
		}
	}
}

func TestOrientYCbCrCroppedRaster(t *testing.T) {
	t.Parallel()

	// The jpeg decoder hands out Min=(0,0) rasters, so a cropped subimage
	// is outside the fast path's contract — it must take the Min-aware
	// generic path and still come out geometrically correct. The generic
	// output is RGBA-quantized, so the comparison allows one 8-bit step
	// per channel.
	big := image.NewYCbCr(image.Rect(0, 0, 24, 16), image.YCbCrSubsampleRatio420)
	fillYCbCr(big)
	sub, ok := big.SubImage(image.Rect(5, 3, 10, 6)).(*image.YCbCr) // A 5x3 crop at Min=(5,3).
	if !ok {
		t.Fatal("SubImage is not a *image.YCbCr")
	}
	if sub.Rect.Min.X != 5 || sub.Rect.Min.Y != 3 {
		t.Fatalf("fixture is not cropped: %v", sub.Rect)
	}
	for o := 2; o <= 8; o++ {
		got := orientYCbCr(sub, o)
		if _, isYCbCr := got.(*image.YCbCr); isYCbCr {
			t.Fatalf("orientation %d: cropped input must not take the plane path", o)
		}
		dw, dh := orientedDims(sub.Rect.Dx(), sub.Rect.Dy(), o)
		if b := got.Bounds(); b.Dx() != dw || b.Dy() != dh {
			t.Fatalf("orientation %d: dims %v, want %dx%d", o, b, dw, dh)
		}
		for y := range dh {
			for x := range dw {
				sx, sy := orientedSource(x, y, sub.Rect.Dx(), sub.Rect.Dy(), o)
				gr, gg, gb, ga := got.At(x, y).RGBA()
				sr, sg, sb, sa := sub.At(sub.Rect.Min.X+sx, sub.Rect.Min.Y+sy).RGBA()
				const step = 257 // One 8-bit quantization step.
				if absDiff(int(gr), int(sr)) > step || absDiff(int(gg), int(sg)) > step ||
					absDiff(int(gb), int(sb)) > step || absDiff(int(ga), int(sa)) > step {
					t.Fatalf("orientation %d: pixel (%d,%d) beyond one quantization step", o, x, y)
				}
			}
		}
	}
}

// absDiff keeps the cropped-raster tolerance readable.
func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
