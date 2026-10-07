// The contact card photo endpoints (ADR 0065): one image per card, uploaded
// multipart and streamed back through the backend. Access follows the card
// (ADR 0054): a bound card is readable by the property's viewers and
// editable by the owner and full-access members; an unbound card belongs to
// the book owner alone.

package http

import (
	"net/http"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	storageshared "github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// GetContactPhoto implements GET /contacts/{contactId}/photo.
func (h *ContactHandlers) GetContactPhoto(w http.ResponseWriter, r *http.Request, contactID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	// The descriptor first: the ETag is the storage key's hash, so a matching
	// If-None-Match answers 304 without opening the object (ADR 0065).
	key, _, err := h.svc.PhotoDescriptor(r.Context(), actor, contactID)
	if err != nil {
		h.handleContactError(w, r, err)
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

	body, size, contentType, _, err := h.svc.OpenContactPhoto(r.Context(), actor, contactID)
	if err != nil {
		h.handleContactError(w, r, err)
		return
	}
	defer httpsupport.ClosePhotoBody(r, body)

	httpsupport.WritePhotoBytes(w, r, contentType, size, body)
}

// UploadContactPhoto implements POST /contacts/{contactId}/photo (ADR 0065):
// multipart through the backend, validated before any storage write — the
// size cap, the magic-byte sniff and the EXIF/GPS strip all run on the
// shared upload seam.
func (h *ContactHandlers) UploadContactPhoto(w http.ResponseWriter, r *http.Request, contactID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
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

	contact, err := h.svc.SetContactPhoto(r.Context(), actor, contactID, processed)
	if err != nil {
		h.handleContactError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, contactResponse(contact))
}

// DeleteContactPhoto implements DELETE /contacts/{contactId}/photo.
func (h *ContactHandlers) DeleteContactPhoto(w http.ResponseWriter, r *http.Request, contactID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if _, err := h.svc.DeleteContactPhoto(r.Context(), actor, contactID); err != nil {
		h.handleContactError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
