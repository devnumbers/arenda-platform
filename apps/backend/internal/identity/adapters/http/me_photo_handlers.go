// The profile photo endpoints (ADR 0065): one image per profile, uploaded
// multipart and streamed back through the backend, readable by the owner
// only.

package http

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	storageshared "github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// GetMePhoto implements GET /me/photo.
func (h *AuthHandlers) GetMePhoto(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	// The descriptor first: the ETag is the storage key's hash, so a matching
	// If-None-Match answers 304 without opening the object (ADR 0065).
	key, _, err := h.profilePhoto.PhotoDescriptor(r.Context(), userID)
	if err != nil {
		h.writePhotoError(w, r, err)
		return
	}
	// The validator and the cache policy travel with every answer — the 200
	// carries them so the client can revalidate, the 304 so the cached copy
	// stays honest.
	etag := storageshared.ETagOf(key)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, max-age=300")
	if httpsupport.ETagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	body, size, contentType, _, err := h.profilePhoto.OpenPhoto(r.Context(), userID)
	if err != nil {
		h.writePhotoError(w, r, err)
		return
	}
	defer h.closePhotoBody(r, body)

	h.writePhotoBytes(w, r, contentType, size, body)
}

// UploadMePhoto implements POST /me/photo.
func (h *AuthHandlers) UploadMePhoto(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	data, err := photo.ReadMultipartUpload(r)
	if err != nil {
		h.writePhotoReadError(w, r, err)
		return
	}
	processed, err := photo.Process(data)
	if err != nil {
		h.writePhotoProcessError(w, r, err)
		return
	}

	user, err := h.profilePhoto.SetProfilePhoto(r.Context(), userID, processed)
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound,
				httpsupport.Problem(r.Context(), "Not found", "Профиль не найден"))
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	resp := meResponse(user)
	if h.meEnricher != nil {
		if err := h.meEnricher(r.Context(), userID, &resp); err != nil {
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// DeleteMePhoto implements DELETE /me/photo.
func (h *AuthHandlers) DeleteMePhoto(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if _, err := h.profilePhoto.DeleteProfilePhoto(r.Context(), userID); err != nil {
		if errors.Is(err, application.ErrPhotoNotFound) || errors.Is(err, application.ErrNotFound) {
			httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound,
				httpsupport.Problem(r.Context(), "Not found", "У профиля нет фото"))
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writePhotoError maps the photo-serving failures onto the wire: a missing
// photo (and a missing profile) is the privacy-preserving 404, everything
// else is the generic 500.
func (h *AuthHandlers) writePhotoError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, application.ErrPhotoNotFound) || errors.Is(err, application.ErrNotFound) {
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound,
			httpsupport.Problem(r.Context(), "Not found", "У профиля нет фото"))
		return
	}
	httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
}

// writePhotoReadError maps the multipart read failures: a malformed form and
// a missing file are the client's 400s.
func (h *AuthHandlers) writePhotoReadError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.WarnContext(r.Context(), "failed to read photo upload", slog.String("error", httpsupport.SanitizeError(err)))
	switch {
	case errors.Is(err, photo.ErrTooLarge):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Файл больше 5 МиБ"))
	case errors.Is(err, photo.ErrUploadMissing):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Требуется файл"))
	default: // The remaining multipart-form errors.
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректная форма загрузки файла"))
	}
}

// writePhotoProcessError maps the upload-seam rejections (ADR 0065): the
// size cap, the format allowlist (SVG included) and the pixel bomb are the
// client's 400s.
func (h *AuthHandlers) writePhotoProcessError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, photo.ErrTooLarge):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Файл больше 5 МиБ"))
	case errors.Is(err, photo.ErrTooManyPixels):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Изображение слишком большого разрешения"))
	default: // The format allowlist rejection.
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Поддерживаются только изображения JPEG, PNG и WebP"))
	}
}

// closePhotoBody closes the streamed object best-effort: a close failure
// cannot unserve the bytes, it only breaks connection reuse.
func (h *AuthHandlers) closePhotoBody(r *http.Request, body io.Closer) {
	if err := body.Close(); err != nil {
		h.logger.WarnContext(r.Context(), "failed to close photo body", slog.String("error", httpsupport.SanitizeError(err)))
	}
}

// writePhotoBytes sets the serving headers and streams the object: the
// content type is the stored sniffed value, the length is the object's, the
// cache policy is private (cookie-authed bytes never enter shared caches).
// A copy failure mid-stream is unrecoverable for the client anyway — the
// status line is long gone — so it only feeds the log.
func (h *AuthHandlers) writePhotoBytes(w http.ResponseWriter, r *http.Request, contentType string, size int64, body io.Reader) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "private, max-age=300")
	if _, err := io.Copy(w, body); err != nil {
		h.logger.WarnContext(r.Context(), "failed to stream photo body", slog.String("error", httpsupport.SanitizeError(err)))
	}
}
