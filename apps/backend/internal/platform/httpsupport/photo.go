// The shared photo-serving helpers of the three photo endpoints (ADR 0065):
// the bytes streaming and the upload-error mapping are one wire contract —
// the contexts keep only their own access-error vocabulary mapping.

package httpsupport

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
)

// WritePhotoBytes sets the serving headers and streams the object: the
// content type is the stored sniffed value, the length is the object's, the
// cache policy is private (cookie-authed bytes never enter shared caches).
// A copy failure mid-stream is unrecoverable for the client anyway — the
// status line is long gone — so it only feeds the log.
func WritePhotoBytes(w http.ResponseWriter, r *http.Request, contentType string, size int64, body io.Reader) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "private, max-age=300")
	if _, err := io.Copy(w, body); err != nil {
		LoggerFromContext(r.Context()).WarnContext(r.Context(),
			"failed to stream photo body", slog.String("error", SanitizeError(err)))
	}
}

// ClosePhotoBody closes the streamed object best-effort: a close failure
// cannot unserve the bytes, it only breaks connection reuse.
func ClosePhotoBody(r *http.Request, body io.Closer) {
	if err := body.Close(); err != nil {
		LoggerFromContext(r.Context()).WarnContext(r.Context(),
			"failed to close photo body", slog.String("error", SanitizeError(err)))
	}
}

// WritePhotoReadError maps the multipart read failures onto the shared wire
// contract: a malformed form and a missing file are the client's 400s.
func WritePhotoReadError(w http.ResponseWriter, r *http.Request, err error) {
	LoggerFromContext(r.Context()).WarnContext(r.Context(),
		"failed to read photo upload", slog.String("error", SanitizeError(err)))
	switch {
	case errors.Is(err, photo.ErrTooLarge):
		WriteProblem(r.Context(), w, http.StatusBadRequest,
			Problem(r.Context(), "Bad request", "Файл больше 5 МиБ"))
	case errors.Is(err, photo.ErrUploadMissing):
		WriteProblem(r.Context(), w, http.StatusBadRequest,
			Problem(r.Context(), "Bad request", "Требуется файл"))
	default: // The remaining multipart-form errors.
		WriteProblem(r.Context(), w, http.StatusBadRequest,
			Problem(r.Context(), "Bad request", "Некорректная форма загрузки файла"))
	}
}

// WritePhotoProcessError maps the upload-seam rejections onto the shared
// wire contract (ADR 0065): the size cap, the format allowlist (SVG
// included) and the pixel bomb are the client's 400s.
func WritePhotoProcessError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, photo.ErrTooLarge):
		WriteProblem(r.Context(), w, http.StatusBadRequest,
			Problem(r.Context(), "Bad request", "Файл больше 5 МиБ"))
	case errors.Is(err, photo.ErrTooManyPixels):
		WriteProblem(r.Context(), w, http.StatusBadRequest,
			Problem(r.Context(), "Bad request", "Изображение слишком большого разрешения"))
	default: // The format allowlist rejection.
		WriteProblem(r.Context(), w, http.StatusBadRequest,
			Problem(r.Context(), "Bad request", "Поддерживаются только изображения JPEG, PNG и WebP"))
	}
}
