package photo

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestTiffOrientation(t *testing.T) {
	t.Parallel()

	t.Run("little-endian orientation 6", func(t *testing.T) {
		t.Parallel()
		if got := tiffOrientation(exifTIFF(t, 6, false)); got != 6 {
			t.Errorf("orientation = %d, want 6", got)
		}
	})

	t.Run("big-endian orientation 8", func(t *testing.T) {
		t.Parallel()
		if got := tiffOrientation(exifTIFF(t, 8, true)); got != 8 {
			t.Errorf("orientation = %d, want 8", got)
		}
	})

	t.Run("no orientation entry", func(t *testing.T) {
		t.Parallel()
		// A valid little-endian TIFF whose only IFD0 entry is the Make tag
		// (0x010f), not the orientation.
		tf := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x0f, 0x01, 3, 0, 1, 0, 0, 0, 5, 0, 0, 0, 0, 0, 0, 0}
		if got := tiffOrientation(tf); got != 0 {
			t.Errorf("orientation = %d, want 0", got)
		}
	})

	t.Run("out-of-range value", func(t *testing.T) {
		t.Parallel()
		if got := tiffOrientation(exifTIFF(t, 9, false)); got != 0 {
			t.Errorf("orientation = %d, want 0", got)
		}
	})

	t.Run("truncated blob", func(t *testing.T) {
		t.Parallel()
		tf := exifTIFF(t, 6, false)
		if got := tiffOrientation(tf[:10]); got != 0 {
			t.Errorf("orientation = %d, want 0", got)
		}
		if got := tiffOrientation(nil); got != 0 {
			t.Errorf("orientation = %d, want 0", got)
		}
	})

	t.Run("garbage header", func(t *testing.T) {
		t.Parallel()
		if got := tiffOrientation([]byte("GPS:55.75,37.61")); got != 0 {
			t.Errorf("orientation = %d, want 0", got)
		}
	})
}

func TestJPEGEXIFOrientation(t *testing.T) {
	t.Parallel()

	t.Run("orientation from the Exif APP1", func(t *testing.T) {
		t.Parallel()
		if got := jpegEXIFOrientation(jpegWithOrientation(t, 6)); got != 6 {
			t.Errorf("orientation = %d, want 6", got)
		}
	})

	t.Run("orientation found after a foreign segment", func(t *testing.T) {
		t.Parallel()
		// A comment marker between SOI and the Exif APP1 — the walk must
		// step over it by its declared length (5 payload bytes + 2).
		base := jpegWithOrientation(t, 7)
		comment := append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0x00, 0x07, 'h', 'e', 'l', 'l', 'o'}, base[2:]...)
		if got := jpegEXIFOrientation(comment); got != 7 {
			t.Errorf("orientation = %d, want 7", got)
		}
	})

	t.Run("xmp APP1 is not exif", func(t *testing.T) {
		t.Parallel()
		// An APP1 carrying an XMP packet must not be read as EXIF.
		base := mustJPEG(t, 4, 4)
		payload := append([]byte("http://ns.adobe.com/xap/1.0/\x00<x/>"), []byte("Exif\x00\x00II*\x00")...)
		segLen := len(payload) + 2
		segment := []byte{0xFF, 0xE1, asByte(t, segLen>>8), asByte(t, segLen)}
		data := append(append([]byte{}, base[:2]...), segment...)
		data = append(data, payload...)
		data = append(data, base[2:]...)
		if got := jpegEXIFOrientation(data); got != 0 {
			t.Errorf("orientation = %d, want 0", got)
		}
	})

	t.Run("no APP1 at all", func(t *testing.T) {
		t.Parallel()
		if got := jpegEXIFOrientation(mustJPEG(t, 4, 4)); got != 0 {
			t.Errorf("orientation = %d, want 0", got)
		}
	})

	t.Run("truncated segment", func(t *testing.T) {
		t.Parallel()
		// The APP1 ends at SOI(2) + marker/len(2+2) + payload(6+26): cutting
		// inside the declared segment length is "no orientation".
		data := jpegWithOrientation(t, 6)
		if got := jpegEXIFOrientation(data[:30]); got != 0 {
			t.Errorf("orientation = %d, want 0", got)
		}
	})
}

// pngWithExif inserts an eXIf chunk (a bare TIFF blob) between IHDR and the
// image data — the placement the PNG spec mandates for ancillary metadata.
func pngWithExif(t *testing.T, orientation int, bigEndian bool) []byte {
	t.Helper()
	base := mustPNG(t, 8, 6)
	exif := pngChunk(t, "eXIf", exifTIFF(t, orientation, bigEndian))
	out := append([]byte{}, base[:33]...) // Signature (8) + IHDR (25).
	out = append(out, exif...)
	return append(out, base[33:]...)
}

func TestPNGEXIFOrientation(t *testing.T) {
	t.Parallel()

	t.Run("orientation from the eXIf chunk", func(t *testing.T) {
		t.Parallel()
		if got := pngEXIFOrientation(pngWithExif(t, 8, false)); got != 8 {
			t.Errorf("orientation = %d, want 8", got)
		}
	})

	t.Run("big-endian eXIf", func(t *testing.T) {
		t.Parallel()
		if got := pngEXIFOrientation(pngWithExif(t, 3, true)); got != 3 {
			t.Errorf("orientation = %d, want 3", got)
		}
	})

	t.Run("plain png", func(t *testing.T) {
		t.Parallel()
		if got := pngEXIFOrientation(mustPNG(t, 4, 4)); got != 0 {
			t.Errorf("orientation = %d, want 0", got)
		}
	})
}

func TestOrientationEXIFRoundtrip(t *testing.T) {
	t.Parallel()

	for o := 2; o <= 8; o++ {
		if got := tiffOrientation(orientationEXIF(o)[len(exifHeader):]); got != o {
			t.Errorf("rewrite of orientation %d parses back as %d", o, got)
		}
	}
	// The rewrite carries nothing else: no GPS, no description — the parse
	// of every entry outside IFD0 must find nothing to leak.
	payload := orientationEXIF(6)
	if bytes.Contains(payload, []byte("GPS")) {
		t.Error("orientation rewrite carries a GPS payload")
	}
	if len(payload) != 6+26 {
		t.Errorf("rewrite length = %d, want the minimal 32 bytes", len(payload))
	}
}

func TestWebPEXIFOrientation(t *testing.T) {
	t.Parallel()

	raw := exifTIFF(t, 6, false)
	if got := webpEXIFOrientation(raw); got != 6 {
		t.Errorf("bare TIFF: orientation = %d, want 6", got)
	}
	prefixed := append(append([]byte{}, exifHeader...), raw...)
	if got := webpEXIFOrientation(prefixed); got != 6 {
		t.Errorf("prefixed TIFF: orientation = %d, want 6", got)
	}
	if got := webpEXIFOrientation([]byte("garbage")); got != 0 {
		t.Errorf("garbage: orientation = %d, want 0", got)
	}
}

// webpEXIFChunkPayload finds the EXIF chunk payload in a stripped webp —
// the assertion side of the rewrite tests.
func webpEXIFChunkPayload(t *testing.T, data []byte) []byte {
	t.Helper()
	pos := webpHeaderLen
	for pos+8 <= len(data) {
		fourcc := string(data[pos : pos+4])
		size := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		end := pos + 8 + size
		if end > len(data) {
			t.Fatalf("broken webp fixture: chunk %q overruns the container", fourcc)
		}
		if fourcc == "EXIF" {
			return data[pos+8 : end]
		}
		pos = end + size%2
	}
	return nil
}

func TestProcess_WebPKeepsOnlyOrientationExif(t *testing.T) {
	t.Parallel()

	// A rotated extended webp with GPS inside the EXIF blob: the output
	// must carry a minimal EXIF chunk with the orientation and nothing
	// else, XMP gone, image chunks intact. The GPS string rides behind the
	// IFD — where real metadata sits, not before the TIFF header.
	gpsExif := append(append(append([]byte{}, exifHeader...), exifTIFF(t, 6, false)...), []byte("GPS:55.75,37.61")...)
	data := riffWebp(t, webpVP8X(t), webpChunk(t, "EXIF", gpsExif), webpChunk(t, "XMP ", []byte("<x/>")))

	got, err := Process(data)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	payload := webpEXIFChunkPayload(t, got.Data)
	if payload == nil {
		t.Fatal("EXIF chunk is gone: the orientation a webp viewer needs died with it")
	}
	if want := orientationEXIF(6); !bytes.Equal(payload, want) {
		t.Errorf("rewritten EXIF payload = %v, want the minimal orientation-only blob %v", payload, want)
	}
	if bytes.Contains(got.Data, []byte("GPS:55.75")) {
		t.Error("GPS payload survived the webp rewrite")
	}
	if bytes.Contains(got.Data, []byte("XMP ")) {
		t.Error("XMP chunk survived the strip")
	}
	if !bytes.Contains(got.Data, []byte("VP8X")) {
		t.Error("VP8X chunk must survive the strip")
	}
	riffSize := len(got.Data) - 8
	if gotRiff := binary.LittleEndian.Uint32(got.Data[4:8]); gotRiff != asUint32(t, riffSize) {
		t.Errorf("RIFF size = %d, want %d", gotRiff, riffSize)
	}
}

func TestProcess_WebPUprightExifChunkIsDropped(t *testing.T) {
	t.Parallel()

	// An orientation-1 webp needs no correction and no metadata: the chunk
	// dies exactly as before the orientation handling existed.
	data := riffWebp(t, webpVP8X(t), webpChunk(t, "EXIF", exifTIFF(t, 1, false)))
	got, err := Process(data)
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if payload := webpEXIFChunkPayload(t, got.Data); payload != nil {
		t.Errorf("EXIF chunk survived: %v", payload)
	}
	if !bytes.Contains(got.Data, []byte("VP8X")) {
		t.Error("VP8X chunk must survive the strip")
	}
}
