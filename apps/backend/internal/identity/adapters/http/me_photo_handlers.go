// The profile photo endpoints (ADR 0065): one image per profile, uploaded
// multipart and streamed back through the backend, readable by the owner
// only.

package http

import (
	"errors"
	"net/http"

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
	defer httpsupport.ClosePhotoBody(r, body)

	httpsupport.WritePhotoBytes(w, r, contentType, size, body)
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
		httpsupport.WritePhotoReadError(w, r, err)
		return
	}
	processed, err := photo.Process(data)
	if err != nil {
		httpsupport.WritePhotoProcessError(w, r, err)
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
