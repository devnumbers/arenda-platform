package photo

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
)

// ReadMultipartUpload streams the "file" part of a multipart/form-data body
// into memory, bounded by MaxSize. MultipartReader is used instead of
// ParseMultipartForm (gosec G120): the memory bound stays explicit and
// nothing ever spills to temporary files.
func ReadMultipartUpload(r *http.Request) ([]byte, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUploadForm, err)
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, ErrUploadMissing
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUploadForm, err)
		}
		if part.FormName() != photoUploadFieldName {
			if err := part.Close(); err != nil {
				return nil, fmt.Errorf("%w: %w", ErrUploadForm, err)
			}
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, MaxSize+1))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUploadForm, err)
		}
		if int64(len(data)) > MaxSize {
			return nil, newTooLargeError(int64(len(data)), MaxSize)
		}
		return data, nil
	}
}

// NewKey mints the storage key of a new photo (ADR 0065): a flat
// photos/<uuid>.<ext> — the UUID carries no PII, the extension follows the
// sniffed content type. Replacement mints a new key, so the serving ETag
// (the key hash) turns over with every upload.
func NewKey(contentType string) (string, error) {
	ext, ok := extensions[contentType]
	if !ok {
		return "", fmt.Errorf("%w: no extension for %q", ErrUnsupportedFormat, contentType)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate photo key: %w", err)
	}
	return "photos/" + id.String() + ext, nil
}

var extensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}
