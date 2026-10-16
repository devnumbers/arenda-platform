package photo

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// defaultTestLimits mirrors defaultLimits: the real caps.
func defaultTestLimits() limits {
	return limits{maxUploadSize: MaxSize, maxObjectSize: MaxSize, maxPixels: MaxPixels}
}

// asUint32 narrows n for the RIFF/PNG fixtures. The bounds must stay
// explicit if-comparisons with an explicit exit: gosec G115 recognizes only
// those (the intconv.go convention).
func asUint32(t *testing.T, n int) uint32 {
	t.Helper()
	if n < 0 || n > math.MaxUint32 {
		t.Fatalf("value %d out of uint32 bounds", n)
		return 0
	}
	return uint32(n)
}

// asByte narrows n for the gray fixture pixels, same explicit-bounds shape.
func asByte(t *testing.T, n int) byte {
	t.Helper()
	if n < 0 || n > 255 {
		t.Fatalf("value %d out of byte bounds", n)
		return 0
	}
	return byte(n)
}

// le32 encodes n as little-endian uint32 for the RIFF fixtures.
func le32(t *testing.T, n int) []byte {
	t.Helper()
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], asUint32(t, n))
	return b[:]
}

func mustJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.SetGray(x, y, color.Gray{Y: asByte(t, (x*16+y*16)%256)})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// jpegWithExif inserts an APP1 marker carrying an Exif payload (with a GPS
// trace string) right after the SOI — the shape a phone camera produces.
func jpegWithExif(t *testing.T) []byte {
	t.Helper()
	base := mustJPEG(t, 4, 4)
	payload := append([]byte("Exif\x00\x00II*\x00\x08\x00\x00\x00\x00\x00"), []byte("GPS:55.75,37.61")...)
	segLen := len(payload) + 2
	if segLen > math.MaxUint16 {
		t.Fatalf("fixture segment length %d out of bounds", segLen)
		return nil
	}
	segment := []byte{0xFF, 0xE1, asByte(t, segLen>>8), asByte(t, segLen)}
	out := append([]byte{}, base[:2]...)
	out = append(out, segment...)
	out = append(out, payload...)
	return append(out, base[2:]...)
}

func mustPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// pngChunk assembles one PNG chunk: length + fourcc + data + CRC.
func pngChunk(t *testing.T, fourcc string, data []byte) []byte {
	t.Helper()
	out := make([]byte, 8+len(data)+4)
	binary.BigEndian.PutUint32(out[:4], asUint32(t, len(data)))
	copy(out[4:8], fourcc)
	copy(out[8:], data)
	binary.BigEndian.PutUint32(out[len(out)-4:], crc32.ChecksumIEEE(out[4:len(out)-4]))
	return out
}

// pngWithText inserts a tEXt chunk (metadata a re-encode must drop) after IHDR.
func pngWithText(t *testing.T) []byte {
	t.Helper()
	base := mustPNG(t, 4, 4)
	// IHDR is the first chunk: fixed 25-byte header (8 signature + 4 length
	// + 4 fourcc) + 13 data + 4 CRC.
	text := pngChunk(t, "tEXt", []byte("Software\x00CameraApp"))
	out := append([]byte{}, base[:33]...)
	out = append(out, text...)
	return append(out, base[33:]...)
}

// webpChunk assembles one RIFF chunk with the odd-length padding byte the
// RIFF spec mandates.
func webpChunk(t *testing.T, fourcc string, data []byte) []byte {
	t.Helper()
	out := append([]byte(fourcc), le32(t, len(data))...)
	out = append(out, data...)
	if len(data)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// webpVP8X is the extended-webp header chunk: flags + canvas 4x4.
func webpVP8X(t *testing.T) []byte {
	t.Helper()
	payload := []byte{0x10, 0, 0, 0, 0, 3, 0, 0, 3, 0}
	return webpChunk(t, "VP8X", payload)
}

// riffWebp assembles an extended WebP: RIFF container + chunks.
func riffWebp(t *testing.T, chunks ...[]byte) []byte {
	t.Helper()
	body := []byte("WEBP")
	for _, c := range chunks {
		body = append(body, c...)
	}
	out := append([]byte("RIFF"), le32(t, len(body))...)
	return append(out, body...)
}

func TestProcess_JPEGWithoutMetadata(t *testing.T) {
	t.Parallel()

	data := mustJPEG(t, 4, 4)
	got, err := Process(data)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if got.ContentType != ContentTypeJPEG {
		t.Errorf("content type = %q, want image/jpeg", got.ContentType)
	}
	if got.Size != int64(len(got.Data)) {
		t.Errorf("size = %d, want %d", got.Size, len(got.Data))
	}
	img, err := jpeg.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatalf("output does not decode: %v", err)
	}
	if img.Bounds().Dx() != 4 || img.Bounds().Dy() != 4 {
		t.Errorf("dims = %v, want 4x4", img.Bounds())
	}
}

func TestProcess_JPEGStripsExifAndGPS(t *testing.T) {
	t.Parallel()

	got, err := Process(jpegWithExif(t))
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if got.ContentType != ContentTypeJPEG {
		t.Errorf("content type = %q, want image/jpeg", got.ContentType)
	}
	if bytes.Contains(got.Data, []byte("Exif\x00\x00")) {
		t.Error("EXIF APP1 marker survived the strip")
	}
	if bytes.Contains(got.Data, []byte("GPS:55.75")) {
		t.Error("GPS payload survived the strip")
	}
	if _, err := jpeg.Decode(bytes.NewReader(got.Data)); err != nil {
		t.Fatalf("stripped output does not decode: %v", err)
	}
}

func TestProcess_PNGStripsTextAndKeepsAlpha(t *testing.T) {
	t.Parallel()

	got, err := Process(pngWithText(t))
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if got.ContentType != ContentTypePNG {
		t.Errorf("content type = %q, want image/png", got.ContentType)
	}
	if bytes.Contains(got.Data, []byte("tEXt")) {
		t.Error("tEXt chunk survived the re-encode")
	}
	img, err := png.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatalf("output does not decode: %v", err)
	}
	nrgba, ok := img.(*image.NRGBA)
	if !ok {
		t.Fatalf("output model = %T, want *image.NRGBA", img)
	}
	if a := nrgba.NRGBAAt(0, 0).A; a != 128 {
		t.Errorf("alpha at (0,0) = %d, want 128 (alpha must survive the roundtrip)", a)
	}
}

func TestProcess_WebPStripsExifChunk(t *testing.T) {
	t.Parallel()

	exifPayload := []byte("II*\x00\x08\x00\x00\x00\x00\x00GPS:55.75,37.61")
	data := riffWebp(t, webpVP8X(t), webpChunk(t, "EXIF", exifPayload))

	got, err := Process(data)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if got.ContentType != ContentTypeWebP {
		t.Errorf("content type = %q, want image/webp", got.ContentType)
	}
	if bytes.Contains(got.Data, []byte("EXIF")) || bytes.Contains(got.Data, []byte("GPS:55.75")) {
		t.Error("EXIF chunk survived the strip")
	}
	if !bytes.Contains(got.Data, []byte("VP8X")) {
		t.Error("VP8X chunk must survive the strip")
	}
	// The rebuilt RIFF size must match the new body.
	riffSize := len(got.Data) - 8
	if gotRiff := binary.LittleEndian.Uint32(got.Data[4:8]); gotRiff != asUint32(t, riffSize) {
		t.Errorf("RIFF size = %d, want %d", gotRiff, riffSize)
	}
}

func TestProcess_WebPWithoutExifPassesThrough(t *testing.T) {
	t.Parallel()

	data := riffWebp(t, webpVP8X(t), webpChunk(t, "VP8L", []byte{0x2F, 0x00, 0x00, 0x00}))
	got, err := Process(data)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if !bytes.Equal(got.Data, data) {
		t.Errorf("clean webp must pass through verbatim, got %d bytes vs %d", len(got.Data), len(data))
	}
}

func TestProcess_RejectsNonImages(t *testing.T) {
	t.Parallel()

	cases := map[string][]byte{
		"svg":            []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`),
		"gif":            []byte("GIF87a\x01\x00\x01\x00\x00\x00\x00;"),
		"plain text":     []byte("just some text"),
		"corrupt jpeg":   {0xFF, 0xD8, 0xFF, 0x00, 0x01, 0x02},
		"corrupt png":    append([]byte("\x89PNG\r\n\x1a\n"), []byte("garbage")...),
		"corrupt webp":   append([]byte("RIFF\x00\x00\x00\x00WEBP"), []byte("\x00\x00")...),
		"truncated webp": append(append([]byte("RIFF"), le32(t, 1000)...), []byte("WEBPVP8X")...),
		"empty":          {},
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := Process(data); !errors.Is(err, ErrUnsupportedFormat) {
				t.Fatalf("err = %v, want ErrUnsupportedFormat", err)
			}
		})
	}
}

// The client's content-type header never travels into Process: the sniff
// alone decides, svg bytes are rejected whatever the header said.
func TestProcess_RejectsSVGBytes(t *testing.T) {
	t.Parallel()

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	if _, err := Process(svg); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("err = %v, want ErrUnsupportedFormat", err)
	}
}

func TestProcess_SizeLimit(t *testing.T) {
	t.Parallel()

	data := mustJPEG(t, 8, 8)

	t.Run("over the upload bound", func(t *testing.T) {
		t.Parallel()
		lim := defaultTestLimits()
		lim.maxUploadSize = 8
		if _, err := process(data, lim); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("err = %v, want ErrTooLarge", err)
		}
	})

	// A re-encode can grow the bytes (a heavily under-compressed source
	// re-encoded at higher quality); the stored object carries the same cap.
	t.Run("over the object bound after the strip", func(t *testing.T) {
		t.Parallel()
		lim := defaultTestLimits()
		lim.maxObjectSize = 8
		if _, err := process(data, lim); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("err = %v, want ErrTooLarge", err)
		}
	})
}

func TestProcess_PixelBomb(t *testing.T) {
	t.Parallel()

	t.Run("jpeg over the pixel bound", func(t *testing.T) {
		t.Parallel()
		lim := defaultTestLimits()
		lim.maxPixels = 9
		if _, err := process(mustJPEG(t, 4, 4), lim); !errors.Is(err, ErrTooManyPixels) {
			t.Fatalf("err = %v, want ErrTooManyPixels", err)
		}
	})

	t.Run("png under the pixel bound passes", func(t *testing.T) {
		t.Parallel()
		lim := defaultTestLimits()
		lim.maxPixels = 16
		if _, err := process(mustPNG(t, 4, 4), lim); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
	})
}

func TestProcess_SmallPhotoPassesDefaultBounds(t *testing.T) {
	t.Parallel()

	// A legitimate small photo passes the default 50 MP bound.
	if _, err := Process(mustJPEG(t, 32, 32)); err != nil {
		t.Fatalf("small photo rejected: %v", err)
	}
}

func TestNewKey(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		ContentTypeJPEG: ".jpg",
		ContentTypePNG:  ".png",
		ContentTypeWebP: ".webp",
	}
	for ct, ext := range cases {
		key, err := NewKey(ct)
		if err != nil {
			t.Fatalf("NewKey(%q): %v", ct, err)
		}
		if !strings.HasPrefix(key, "photos/") {
			t.Errorf("key = %q, want the photos/ prefix", key)
		}
		if !strings.HasSuffix(key, ext) {
			t.Errorf("key = %q, want the %q extension", key, ext)
		}
		name := strings.TrimSuffix(strings.TrimPrefix(key, "photos/"), ext)
		if _, err := uuid.Parse(name); err != nil {
			t.Errorf("key = %q: object name %q is not a uuid: %v", key, name, err)
		}
	}

	if _, err := NewKey("image/gif"); err == nil {
		t.Error("unknown content type must not mint a key")
	}
}
