package photo

// Baking the EXIF orientation into the pixels. A phone camera stores the
// raw sensor raster and lets the orientation tag tell the viewer how to
// turn it upright; the strip removes the tag, so the correction must happen
// here instead — otherwise every stripped photo displays sideways or
// mirrored exactly as the sensor saw it.

import "image"

// applyOrientation returns img rewritten so it displays upright with no
// metadata: orientation 1 (and anything unreadable) returns img unchanged,
// every defined EXIF orientation maps onto the transform a viewer would
// have applied.
func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 1:
		return img
	case 2, 3, 4, 5, 6, 7, 8:
	default:
		return img
	}
	switch src := img.(type) {
	case *image.YCbCr:
		// The camera-photo path: a decoded jpeg is a *image.YCbCr, and the
		// transform runs plane-wise at byte speed.
		return orientYCbCr(src, orientation)
	case *image.Gray:
		return orientGray(src, orientation)
	default:
		return orientGeneric(img, orientation)
	}
}

// orientedDims reports the baked image's dimensions: the quarter-turn
// orientations swap the sides.
func orientedDims(w, h, o int) (dw, dh int) {
	switch o {
	case 5, 6, 7, 8:
		return h, w
	default:
		return w, h
	}
}

// orientedSource maps one output pixel of the baked image back onto the raw
// sensor pixel, per the EXIF orientation's viewer correction:
//
//	2 mirror horizontal   5 transpose (mirrored + 270°)   6 rotate 90° CW
//	3 rotate 180°         7 transverse (mirrored + 90°)   8 rotate 270° CW
//	4 mirror vertical
func orientedSource(x, y, w, h, o int) (sx, sy int) {
	switch o {
	case 2:
		return w - 1 - x, y
	case 3:
		return w - 1 - x, h - 1 - y
	case 4:
		return x, h - 1 - y
	case 5:
		return y, x
	case 6:
		return y, h - 1 - x
	case 7:
		return w - 1 - y, h - 1 - x
	case 8:
		return w - 1 - y, x
	default:
		return x, y
	}
}

// orientGeneric is the fallback for decodes without a plane path (png
// lands here): an RGBA canvas filled pixel by pixel.
func orientGeneric(src image.Image, o int) image.Image {
	b := src.Bounds()
	dw, dh := orientedDims(b.Dx(), b.Dy(), o)
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range dh {
		for x := range dw {
			sx, sy := orientedSource(x, y, b.Dx(), b.Dy(), o)
			dst.Set(x, y, src.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}

// orientGray is the grayscale-jpeg path: a single 2-D byte plane. Gray
// planes index linearly, so a cropped raster shifts uniformly — but the
// decoders never hand one out, and the guard keeps the seam honest.
func orientGray(src *image.Gray, o int) image.Image {
	if src.Rect.Min != (image.Point{}) {
		return orientGeneric(src, o)
	}
	b := src.Bounds()
	dw, dh := orientedDims(b.Dx(), b.Dy(), o)
	dst := image.NewGray(image.Rect(0, 0, dw, dh))
	orientPlane(src.Pix, b.Dx(), b.Dy(), src.Stride, dst.Pix, dw, dh, dst.Stride, o)
	return dst
}

// orientYCbCr keeps the re-encode loss profile of the strip unchanged: the
// planes transform directly, with no RGBA detour. Every plane — luma and
// the two subsampled chroma planes — is an ordinary w×h byte raster, so the
// same pixel mapping runs on each with its own dimensions. The subsample
// label travels along rotated (a quarter turn turns a 2×1 chroma block into
// a 1×2 one) — jpeg.Encode reads the planes back by that label.
func orientYCbCr(src *image.YCbCr, o int) image.Image {
	dstRatio, ok := rotatedYCbCrRatio(src.SubsampleRatio, o)
	if !ok {
		// A quarter turn of an exotic subsampling (4:1:1, 4:1:0) has no
		// stdlib ratio to label the result with: take the generic RGBA
		// path — the encoder re-subsamples to 4:2:0 anyway.
		return orientGeneric(src, o)
	}
	if src.Rect.Min != (image.Point{}) {
		// The chroma-plane read offset of a cropped subimage does not
		// follow the luma one under subsampling; the decoder never hands
		// one out, so anything cropped takes the Min-aware generic path.
		return orientGeneric(src, o)
	}
	b := src.Bounds()
	dw, dh := orientedDims(b.Dx(), b.Dy(), o)
	dst := image.NewYCbCr(image.Rect(0, 0, dw, dh), dstRatio)
	cw, ch := chromaDims(b.Dx(), b.Dy(), src.SubsampleRatio)
	dcw, dch := chromaDims(dw, dh, dstRatio)
	orientPlane(src.Y, b.Dx(), b.Dy(), src.YStride, dst.Y, dw, dh, dst.YStride, o)
	orientPlane(src.Cb, cw, ch, src.CStride, dst.Cb, dcw, dch, dst.CStride, o)
	orientPlane(src.Cr, cw, ch, src.CStride, dst.Cr, dcw, dch, dst.CStride, o)
	return dst
}

// rotatedYCbCrRatio reports the subsample label the rotated image carries
// and whether the stdlib has one. Mirrors keep the label; a quarter turn
// swaps the halves of the chroma block (422 ↔ 440) and leaves the
// symmetric 444 and 420 in place.
func rotatedYCbCrRatio(r image.YCbCrSubsampleRatio, o int) (image.YCbCrSubsampleRatio, bool) {
	switch o {
	case 2, 3, 4:
		return r, true
	case 5, 6, 7, 8:
		switch r {
		case image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio420:
			return r, true
		case image.YCbCrSubsampleRatio422:
			return image.YCbCrSubsampleRatio440, true
		case image.YCbCrSubsampleRatio440:
			return image.YCbCrSubsampleRatio422, true
		case image.YCbCrSubsampleRatio411, image.YCbCrSubsampleRatio410:
			// No stdlib ratio labels a quarter turn of these.
			return r, false
		}
	}
	return r, false
}

// chromaDims replicates the chroma-plane geometry of image.NewYCbCr (the
// stdlib switch over the subsample ratio) — the plane transform needs the
// same dimensions the stdlib itself would allocate.
func chromaDims(w, h int, r image.YCbCrSubsampleRatio) (cw, ch int) {
	switch r {
	case image.YCbCrSubsampleRatio422:
		return (w + 1) / 2, h
	case image.YCbCrSubsampleRatio420:
		return (w + 1) / 2, (h + 1) / 2
	case image.YCbCrSubsampleRatio440:
		return w, (h + 1) / 2
	case image.YCbCrSubsampleRatio411:
		return (w + 3) / 4, h
	case image.YCbCrSubsampleRatio410:
		return (w + 3) / 4, (h + 1) / 2
	default: // 4:4:4
		return w, h
	}
}

// orientPlane fills dst (dw×dh, row stride dstride) from src (w×h, row
// stride stride; the caller guarantees Min=(0,0)) by the orientation
// mapping.
func orientPlane(src []byte, w, h, stride int, dst []byte, dw, dh, dstride, o int) {
	for y := range dh {
		row := dst[y*dstride : y*dstride+dw]
		for x := range dw {
			sx, sy := orientedSource(x, y, w, h, o)
			row[x] = src[sy*stride+sx]
		}
	}
}
