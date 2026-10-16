// Package photo is the upload seam of the private photos (ADR 0065): every
// byte a browser sends travels through Process before it may reach the
// object storage — the size cap, the magic-byte format sniff (the client's
// header is never trusted; SVG dies in the allowlist), the pixel-bomb bound
// and the EXIF/GPS strip happen here, before any storage write.
//
// The strip mechanism is per format (решение #1220): jpeg and png are
// decoded and re-encoded — a generation loss, but it also breaks any
// embedded payload (OWASP file-upload guidance); webp is not decoded at all
// (the Go toolchain cannot encode webp) — its RIFF EXIF/XMP chunks are cut
// and the container rebuilt.
package photo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
)

// MaxSize caps one photo upload and one stored object — the same 5 MiB the
// wire contract and the multipart reader enforce.
const MaxSize = 5 << 20

// MaxPixels bounds the decoded image area (research #1218: the OWASP
// pixel-bomb bound) — checked on image.DecodeConfig, before any full decode
// allocates.
const MaxPixels = 50_000_000

// jpegQuality is the re-encode quality of the jpeg strip: a mild generation
// loss that keeps the photos visually intact while cutting the metadata.
const jpegQuality = 85

// The content types the upload seam accepts, by magic bytes only. The same
// values travel into storage and become the stored content type.
const (
	ContentTypeJPEG = "image/jpeg"
	ContentTypePNG  = "image/png"
	ContentTypeWebP = "image/webp"
)

// The upload seam's error vocabulary; the transports map them onto the 4xx
// wire contract.
var (
	// ErrTooLarge marks an upload or a stripped object over MaxSize.
	ErrTooLarge = errors.New("photo: too large")
	// ErrUnsupportedFormat marks bytes outside the jpeg/png/webp allowlist
	// — SVG included — and bytes that do not decode as the sniffed format.
	ErrUnsupportedFormat = errors.New("photo: unsupported format")
	// ErrTooManyPixels marks a pixel bomb: a tiny file declaring huge
	// dimensions.
	ErrTooManyPixels = errors.New("photo: too many pixels")
	// ErrUploadForm marks a body that is not a well-formed multipart form.
	ErrUploadForm = errors.New("photo: malformed upload form")
	// ErrUploadMissing marks a multipart form without the "file" part.
	ErrUploadMissing = errors.New("photo: no file field")
)

// photoUploadFieldName is the multipart form field carrying the photo bytes.
const photoUploadFieldName = "file"

// Processed is a normalized upload: the stripped bytes, their sniffed
// content type (the value stored alongside the object key) and the size.
type Processed struct {
	Data        []byte
	ContentType string
	Size        int64
}

// limits carries the knobs the tests shrink; Process runs the production
// defaults.
type limits struct {
	maxUploadSize int64
	maxObjectSize int64
	maxPixels     int
}

func defaultLimits() limits {
	return limits{maxUploadSize: MaxSize, maxObjectSize: MaxSize, maxPixels: MaxPixels}
}

// Process validates and normalizes one upload (ADR 0065): size caps, the
// magic-byte sniff over the allowlist, the pixel-bomb bound and the
// EXIF/GPS strip. The input is the bounded multipart read — over-size
// bodies never get this far, the cap here is defense in depth.
func Process(data []byte) (Processed, error) {
	return process(data, defaultLimits())
}

func process(data []byte, lim limits) (Processed, error) {
	if int64(len(data)) > lim.maxUploadSize {
		return Processed{}, newTooLargeError(int64(len(data)), lim.maxUploadSize)
	}
	if len(data) == 0 {
		return Processed{}, fmt.Errorf("%w: empty upload", ErrUnsupportedFormat)
	}

	contentType := http.DetectContentType(data)
	var out []byte
	switch contentType {
	case ContentTypeJPEG:
		stripped, err := stripJPEG(data, lim)
		if err != nil {
			return Processed{}, err
		}
		out = stripped
	case ContentTypePNG:
		stripped, err := stripPNG(data, lim)
		if err != nil {
			return Processed{}, err
		}
		out = stripped
	case ContentTypeWebP:
		stripped, err := stripWebP(data)
		if err != nil {
			return Processed{}, err
		}
		out = stripped
	default:
		// SVG, GIF, HEIC, PDF… — everything outside the allowlist,
		// including executable-content vectors, dies here.
		return Processed{}, fmt.Errorf("%w: content type %q is not accepted", ErrUnsupportedFormat, contentType)
	}

	if int64(len(out)) > lim.maxObjectSize {
		return Processed{}, newTooLargeError(int64(len(out)), lim.maxObjectSize)
	}
	return Processed{Data: out, ContentType: contentType, Size: int64(len(out))}, nil
}

func newTooLargeError(size, limit int64) error {
	return fmt.Errorf("%w: file size %d exceeds %d bytes", ErrTooLarge, size, limit)
}

// stripJPEG re-encodes the image: EXIF/GPS live in APP1 markers a re-encode
// never carries. DecodeConfig first — the pixel bound must fire before a
// full decode allocates.
func stripJPEG(data []byte, lim limits) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: broken jpeg: %w", ErrUnsupportedFormat, err)
	}
	if cfg.Width*cfg.Height > lim.maxPixels {
		return nil, newPixelBombError(cfg, lim)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: broken jpeg: %w", ErrUnsupportedFormat, err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("re-encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

// stripPNG re-encodes the image: lossless for the pixels, but ancillary
// chunks (tEXt, eXIf, iCCP) do not survive.
func stripPNG(data []byte, lim limits) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: broken png: %w", ErrUnsupportedFormat, err)
	}
	if cfg.Width*cfg.Height > lim.maxPixels {
		return nil, newPixelBombError(cfg, lim)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: broken png: %w", ErrUnsupportedFormat, err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("re-encode png: %w", err)
	}
	return buf.Bytes(), nil
}

func newPixelBombError(cfg image.Config, lim limits) error {
	return fmt.Errorf("%w: %dx%d exceeds the %d pixel bound",
		ErrTooManyPixels, cfg.Width, cfg.Height, lim.maxPixels)
}

// webpHeaderLen is the fixed RIFF container head: fourcc + size + "WEBP".
const webpHeaderLen = 12

// stripWebP cuts the EXIF and XMP RIFF chunks without decoding (ADR 0065:
// webp travels verbatim) and rebuilds the container with the new size.
// Chunks are top-level: EXIF metadata only exists in the extended format
// next to VP8X; a simple lossy/lossless webp carries none and passes
// through byte-identical.
func stripWebP(data []byte) ([]byte, error) {
	// DetectContentType has verified RIFF…WEBP already.
	var out bytes.Buffer
	out.Write(data[:webpHeaderLen])

	pos := int64(webpHeaderLen)
	for pos+8 <= int64(len(data)) {
		fourcc := string(data[pos : pos+4])
		// The uint32 → int64 widening never overflows: the arithmetic stays
		// in int64 so gosec G115 has no narrowing to flag.
		size := int64(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		end := pos + 8 + size
		if end > int64(len(data)) {
			return nil, fmt.Errorf("%w: broken webp: chunk %q overruns the container", ErrUnsupportedFormat, fourcc)
		}
		if fourcc != "EXIF" && fourcc != "XMP " {
			chunk := data[pos:end]
			if _, err := out.Write(chunk); err != nil {
				return nil, fmt.Errorf("keep webp chunk: %w", err)
			}
			if size%2 == 1 && end < int64(len(data)) {
				// The odd-chunk pad byte travels with its chunk.
				if _, err := out.Write(data[end : end+1]); err != nil {
					return nil, fmt.Errorf("keep webp pad byte: %w", err)
				}
			}
		}
		pos = end + size%2
	}

	if out.Len() <= webpHeaderLen {
		return nil, fmt.Errorf("%w: broken webp: no image chunks", ErrUnsupportedFormat)
	}
	// Fix the RIFF size: the byte count after the size field itself. The
	// conversion is bounded by the explicit comparison (gosec G115).
	riffSize := out.Len() - 8
	if riffSize < 0 || riffSize > math.MaxUint32 {
		return nil, fmt.Errorf("%w: broken webp: container size %d out of bounds", ErrUnsupportedFormat, riffSize)
	}
	binary.LittleEndian.PutUint32(out.Bytes()[4:8], uint32(riffSize))
	return out.Bytes(), nil
}
