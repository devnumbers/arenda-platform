package photo

// EXIF orientation parsing. The orientation lives in a TIFF blob that
// travels inside the metadata of every accepted format: an APP1 marker
// (jpeg), the eXIf ancillary chunk (png) and the EXIF RIFF chunk (webp) —
// in all three the blob's IFD0 carries tag 0x0112. The parsers never fail:
// unreadable or out-of-range metadata means "no orientation", not a broken
// upload — the metadata handling stays best-effort by design (решение
// #1220), and the strip itself is the security boundary, not the parse.

import (
	"bytes"
	"encoding/binary"
)

// exifOrientationTag is the TIFF IFD0 tag carrying the orientation value
// (EXIF spec, tag 0x0112, type SHORT).
const exifOrientationTag = 0x0112

// exifHeader is the payload prefix of a jpeg APP1 marker ("Exif" plus the
// two zero bytes that separate it from the TIFF blob).
var exifHeader = []byte("Exif\x00\x00")

// tiffOrientation reads the orientation (1–8) from a TIFF blob; 0 when the
// blob is absent, truncated or carries no sane orientation entry. Both byte
// orders are read: cameras write II (little-endian) and MM (big-endian).
func tiffOrientation(tf []byte) int {
	if len(tf) < 8 {
		return 0
	}
	var bo binary.ByteOrder
	switch {
	case tf[0] == 'I' && tf[1] == 'I':
		bo = binary.LittleEndian
	case tf[0] == 'M' && tf[1] == 'M':
		bo = binary.BigEndian
	default:
		return 0
	}
	if bo.Uint16(tf[2:4]) != 42 {
		return 0
	}
	// The uint32 → int widening stays bounded: an offset past the blob (or
	// the per-entry bound below) bails out as "no orientation".
	off := int(bo.Uint32(tf[4:8]))
	if off < 8 || off+2 > len(tf) {
		return 0
	}
	entries := int(bo.Uint16(tf[off : off+2]))
	for i := range entries {
		entry := off + 2 + i*12
		if entry+12 > len(tf) {
			return 0
		}
		if bo.Uint16(tf[entry:entry+2]) != exifOrientationTag {
			continue
		}
		// A SHORT with count 1 sits inline in the entry's value field.
		if v := int(bo.Uint16(tf[entry+8 : entry+10])); v >= 1 && v <= 8 {
			return v
		}
		return 0
	}
	return 0
}

// jpegSegment returns the payload of the segment at pos (which points at
// the 0xFF of a marker) and the position of the next marker. The ok
// result is false at the entropy-coded data and at a truncated structure —
// everywhere the walk must stop as "no more segments".
func jpegSegment(data []byte, pos int) (payload []byte, next int, ok bool) {
	if pos+2 > len(data) || data[pos] != 0xFF {
		return nil, pos, false
	}
	marker := data[pos+1]
	switch {
	case marker == 0xFF: // A fill byte before the marker.
		return nil, pos + 1, true
	case marker == 0x01, marker >= 0xD0 && marker <= 0xD7: // Standalone.
		return nil, pos + 2, true
	case marker == 0xDA || marker == 0xD9: // Entropy-coded data follows.
		return nil, pos, false
	}
	if pos+4 > len(data) {
		return nil, pos, false
	}
	segLen := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
	if segLen < 2 {
		return nil, pos, false
	}
	end := pos + 2 + segLen
	if end > len(data) {
		return nil, pos, false
	}
	return data[pos+4 : end], end, true
}

// jpegEXIFOrientation scans the markers before the first scan for the Exif
// APP1 and reads its orientation; 0 when there is none. The walk is bounded
// by the segment lengths themselves — any surprise is "no orientation".
func jpegEXIFOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0
	}
	pos := 2
	for {
		payload, next, ok := jpegSegment(data, pos)
		if !ok {
			return 0
		}
		if bytes.HasPrefix(payload, exifHeader) {
			return tiffOrientation(payload[len(exifHeader):])
		}
		pos = next
	}
}

// pngEXIFOrientation reads the orientation from the eXIf ancillary chunk,
// which carries a bare TIFF blob and must precede the image data chunks.
func pngEXIFOrientation(data []byte) int {
	const headerLen = 8 // The signature.
	pos := headerLen
	for pos+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		fourcc := string(data[pos+4 : pos+8])
		bodyStart := pos + 8
		bodyEnd := bodyStart + length
		if bodyEnd+4 > len(data) {
			return 0
		}
		if fourcc == "eXIf" {
			return tiffOrientation(data[bodyStart:bodyEnd])
		}
		if fourcc == "IDAT" || fourcc == "IEND" {
			// Ancillary chunks live before the image data.
			return 0
		}
		pos = bodyEnd + 4 // Skip the CRC.
	}
	return 0
}

// orientationEXIF builds a minimal Exif payload carrying only the
// orientation tag. It is the rewrite a webp gets instead of its original
// EXIF chunk: webp pixels cannot be re-encoded by the Go toolchain
// (ADR 0065), so the tag itself must survive for the viewer to correct the
// display — while every other entry of the original blob (GPS included)
// still dies with it.
func orientationEXIF(orientation int) []byte {
	if orientation < 1 || orientation > 8 {
		// Unreachable through the parsers: they only return 1–8. The
		// explicit bound is the sanctioned gosec narrowing shape.
		return nil
	}
	tiff := []byte{
		'I', 'I', 42, 0, 8, 0, 0, 0, // Header: little-endian, first IFD at 8.
		1, 0, // One IFD0 entry.
		0x12, 0x01, 3, 0, 1, 0, 0, 0, // Tag 0x0112, type SHORT, count 1.
		byte(orientation), 0, 0, 0, // The inline SHORT value.
		0, 0, 0, 0, // No next IFD.
	}
	return append(append([]byte{}, exifHeader...), tiff...)
}

// webpEXIFOrientation extracts the orientation from a webp EXIF chunk
// payload. The container spec leaves the "Exif\0\0" prefix optional, so
// both shapes are read.
func webpEXIFOrientation(payload []byte) int {
	return tiffOrientation(bytes.TrimPrefix(payload, exifHeader))
}
