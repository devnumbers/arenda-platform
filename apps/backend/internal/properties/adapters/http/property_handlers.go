// Package http holds the properties HTTP adapters: property lifecycle endpoints with photos and contacts.
package http

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// PropertyHandlers implements the generated property endpoints.
type PropertyHandlers struct {
	svc              *propertiesapp.PropertyService
	addressSuggester propertiesapp.AddressSuggester
	contactSvc       *propertiesapp.PropertyContactService
	logger           *slog.Logger
	clock            clock.Clock
}

// NewPropertyHandlers creates HTTP handlers for the properties API.
func NewPropertyHandlers(
	svc *propertiesapp.PropertyService,
	addressSuggester propertiesapp.AddressSuggester,
	contactSvc *propertiesapp.PropertyContactService,
	logger *slog.Logger,
	clk clock.Clock,
) *PropertyHandlers {
	return &PropertyHandlers{
		svc:              svc,
		addressSuggester: addressSuggester,
		contactSvc:       contactSvc,
		logger:           logger,
		clock:            clk,
	}
}

// problemTitle constants name the RFC 7807 titles reused by the static
// property error mappings below (the mapping table literals are not function
// calls, so the repeated titles need named constants).
const (
	problemTitleConflict  = "Conflict"
	problemTitleForbidden = "Forbidden"
	problemTitleNotFound  = "Not found"
)

// propertyProblem is the wire mapping of a property application error with a
// fixed outcome: a status plus problem details. The code field carries the
// extension code of the single coded mapping (the suspended membership, T9).
type propertyProblem struct {
	status int
	title  string
	detail string
	code   string
}

// staticPropertyProblems maps the property application errors whose wire
// outcome is fixed onto their status and problem details, grouped by outcome:
// access outcomes (issue #156 T3, T9 — the privacy-preserving 404, the
// edit-permission 403, and the suspended membership's distinguishable coded
// 403), the tariff limit, and the domain lifecycle conflicts. Errors whose
// details are derived at runtime (invalid input, transitions, tenant
// contacts) are handled by handlePropertyError's dynamic branches instead.
var staticPropertyProblems = []struct {
	err  error
	prob propertyProblem
}{
	// Access outcomes.
	{propertiesapp.ErrNotFound, propertyProblem{
		status: http.StatusNotFound, title: problemTitleNotFound, detail: "Объект не найден",
	}},
	{propertiesapp.ErrForbidden, propertyProblem{
		status: http.StatusForbidden, title: problemTitleForbidden, detail: "Недостаточно прав для этого действия",
	}},
	{propertiesapp.ErrAccessSuspended, propertyProblem{
		status: http.StatusForbidden, title: problemTitleForbidden,
		detail: "Доступ к объекту приостановлен: превышен лимит объектов по тарифу", code: "membership_suspended",
	}},
	// Tariff limit.
	{propertiesapp.ErrLimitExceeded, propertyProblem{
		status: http.StatusPaymentRequired, title: "Limit exceeded", detail: "Превышен лимит активных объектов",
	}},
	// Domain lifecycle conflicts.
	{propertiesapp.ErrArchivedProperty, propertyProblem{
		status: http.StatusConflict, title: problemTitleConflict, detail: "Нельзя изменить архивный объект",
	}},
	{propertiesapp.ErrAlreadyArchived, propertyProblem{
		status: http.StatusConflict, title: problemTitleConflict, detail: "Объект уже в архиве",
	}},
	{propertiesapp.ErrNotArchived, propertyProblem{
		status: http.StatusConflict, title: problemTitleConflict, detail: "Объект не в архиве",
	}},
	{propertiesapp.ErrPhotoLimitReached, propertyProblem{
		status: http.StatusConflict, title: problemTitleConflict, detail: "Достигнут лимит фотографий объекта",
	}},
}

// handlePropertyError maps an application error of the property endpoints onto
// the wire contract: a fixed mapping first, then the dynamic ones whose detail
// derives from the error itself.
func (h *PropertyHandlers) handlePropertyError(w http.ResponseWriter, r *http.Request, err error) {
	if prob, ok := lookupStaticPropertyProblem(err); ok {
		writePropertyProblem(w, r, prob)
		return
	}
	switch {
	case errors.Is(err, propertiesapp.ErrInvalidInput):
		writeInvalidInputProblem(w, r, err)
	case isInvalidStatusTransition(err), errors.Is(err, propertiesapp.ErrInvalidTransition):
		writeUserFacingProblem(w, r, err, http.StatusConflict, "Conflict")
	default:
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

// lookupStaticPropertyProblem resolves an error to its fixed wire outcome. The
// table is walked with errors.Is so wrapped sentinels keep matching; ok is
// false for the errors whose details are derived dynamically.
func lookupStaticPropertyProblem(err error) (propertyProblem, bool) {
	for _, m := range staticPropertyProblems {
		if errors.Is(err, m.err) {
			return m.prob, true
		}
	}
	return propertyProblem{}, false
}

// writePropertyProblem writes a fixed property outcome; the coded variant (the
// suspended membership, T9, issue #158) is the only one carrying an extension
// code.
func writePropertyProblem(w http.ResponseWriter, r *http.Request, prob propertyProblem) {
	if prob.code != "" {
		httpsupport.WriteProblem(r.Context(), w, prob.status,
			httpsupport.ProblemWithCode(r.Context(), prob.title, prob.detail, prob.code))
		return
	}
	httpsupport.WriteProblem(r.Context(), w, prob.status, httpsupport.Problem(r.Context(), prob.title, prob.detail))
}

// writeUserFacingProblem maps an error whose user-facing message comes from
// httpsupport.UserFacingDetail; without a user-facing detail the response
// degrades to the opaque internal-error 500.
func writeUserFacingProblem(w http.ResponseWriter, r *http.Request, err error, status int, title string) {
	detail, ok := httpsupport.UserFacingDetail(err)
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteProblem(r.Context(), w, status, httpsupport.Problem(r.Context(), title, detail))
}

// writeInvalidInputProblem maps invalid input: catalog attribute failures
// become per-field problem errors, other invalid-input errors carry their
// user-facing detail.
func writeInvalidInputProblem(w http.ResponseWriter, r *http.Request, err error) {
	var attrErrs *propertiesapp.AttributesValidationError
	if !errors.As(err, &attrErrs) {
		writeUserFacingProblem(w, r, err, http.StatusBadRequest, "Bad request")
		return
	}
	fieldErrors := make([]openapi.ProblemError, 0, len(attrErrs.Errors))
	for _, e := range attrErrs.Errors {
		fieldErrors = append(fieldErrors, openapi.ProblemError{Field: e.Field, Detail: e.Reason})
	}
	httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
		httpsupport.ProblemWithFieldErrors(r.Context(), "Bad request", "Некорректные характеристики объекта", fieldErrors))
}

// CreateProperty implements POST /properties.
func (h *PropertyHandlers) CreateProperty(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create property request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	description := ""
	if body.Description != nil {
		description = *body.Description
	}

	var attrs map[string]any
	if body.Attributes != nil {
		attrs = map[string]any(*body.Attributes)
	}

	cmd := propertiesapp.CreatePropertyCommand{
		Name:        body.Name,
		Type:        string(body.Type),
		Address:     body.Address,
		Description: description,
		Attributes:  attrs,
	}

	property, err := h.svc.CreateProperty(r.Context(), actor, cmd)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	h.respondWithProperty(w, r, property, http.StatusCreated)
}

// ListProperties implements GET /properties.
func (h *PropertyHandlers) ListProperties(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	properties, err := h.svc.ListProperties(r.Context(), actor)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	items := make([]openapi.PropertyResponse, 0, len(properties))
	for _, property := range properties {
		items = append(items, h.propertyResponse(property))
	}

	// Report how many shared objects are hidden from the recipient by a tariff
	// slot shortage (suspended memberships), for a footnote in the UI (issue
	// #158, T4). Owners and recipients within their limit get zero.
	hidden, err := h.svc.HiddenSharedCount(r.Context(), actor)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to count hidden shared properties", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	hiddenSharedCount := hidden

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PropertiesResponse{
		Items:             items,
		HiddenSharedCount: &hiddenSharedCount,
	})
}

// ListArchivedProperties implements GET /properties/archive.
func (h *PropertyHandlers) ListArchivedProperties(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	properties, err := h.svc.ListArchivedProperties(r.Context(), actor)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	items := make([]openapi.PropertyResponse, 0, len(properties))
	for _, property := range properties {
		items = append(items, h.propertyResponse(property))
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PropertiesResponse{Items: items})
}

// GetProperty implements GET /properties/{id}.
func (h *PropertyHandlers) GetProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	property, err := h.svc.GetProperty(r.Context(), actor, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	h.respondWithProperty(w, r, property, http.StatusOK)
}

// UpdateProperty implements PATCH /properties/{id}.
func (h *PropertyHandlers) UpdateProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update property request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := propertiesapp.UpdatePropertyCommand{
		Name:        body.Name,
		Type:        httpsupport.PtrString(body.Type),
		Address:     body.Address,
		Description: body.Description,
		Attributes:  propertyAttributesPtr(body.Attributes),
		Status:      httpsupport.PtrString(body.Status),
	}

	property, err := h.svc.UpdateProperty(r.Context(), actor, id, cmd)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	h.respondWithProperty(w, r, property, http.StatusOK)
}

// DeleteProperty implements DELETE /properties/{id}.
func (h *PropertyHandlers) DeleteProperty(
	w http.ResponseWriter, r *http.Request, id openapi_types.UUID, params openapi.DeletePropertyParams,
) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	mode, err := domain.ParseDeletePropertyMode(string(params.Mode))
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректный режим удаления"))
		return
	}

	if err := h.svc.DeleteProperty(r.Context(), actor, id, mode); err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ArchiveProperty implements POST /properties/{id}/archive.
func (h *PropertyHandlers) ArchiveProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	property, err := h.svc.ArchiveProperty(r.Context(), actor, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	h.respondWithProperty(w, r, property, http.StatusOK)
}

// UnarchiveProperty implements POST /properties/{id}/unarchive.
func (h *PropertyHandlers) UnarchiveProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	property, err := h.svc.UnarchiveProperty(r.Context(), actor, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	h.respondWithProperty(w, r, property, http.StatusOK)
}

// UploadPropertyPhoto implements POST /properties/{propertyId}/photos.
func (h *PropertyHandlers) UploadPropertyPhoto(w http.ResponseWriter, r *http.Request, propertyID uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	filename, contentType, data, err := readPhotoUpload(r)
	if err != nil {
		h.rejectPhotoUpload(w, r, err)
		return
	}

	property, err := h.svc.AddPropertyPhoto(r.Context(), actor, propertyID, bytes.NewReader(data), filename, contentType, int64(len(data)))
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	if len(property.Photos) == 0 {
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError,
			httpsupport.InternalError(r.Context(), errors.New("uploaded photo not found")))
		return
	}

	uploaded := property.Photos[len(property.Photos)-1]
	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, openapi.PropertyPhoto{Id: uploaded.ID, Url: uploaded.URL})
}

// rejectPhotoUpload maps readPhotoUpload failures onto the same wire contract
// the previous ParseMultipartForm/FormFile parsing produced.
func (h *PropertyHandlers) rejectPhotoUpload(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, propertiesapp.ErrInvalidInput):
		// An oversized file takes the service's own invalid-input mapping so
		// the response body is identical wherever the size check fires.
		h.handlePropertyError(w, r, err)
	case errors.Is(err, errPhotoUploadMissing):
		h.logger.ErrorContext(r.Context(), "failed to get file from form", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Требуется файл"))
	default:
		h.logger.ErrorContext(r.Context(), "failed to parse multipart form", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректная форма загрузки файла"))
	}
}

var (
	// Marks a body that is not a well-formed multipart form: wrong content
	// type, broken framing or an aborted read.
	errPhotoUploadForm = errors.New("malformed photo upload form")
	// Marks a multipart form without a "file" part.
	errPhotoUploadMissing = errors.New("photo upload form has no file field")
)

// photoUploadFieldName is the multipart form field carrying the photo bytes.
const photoUploadFieldName = "file"

// readPhotoUpload streams the "file" part of a multipart/form-data body into
// memory, bounded by the application's photo size limit. MultipartReader is
// used instead of ParseMultipartForm (gosec G120): the memory bound stays
// explicit and nothing ever spills to temporary files.
func readPhotoUpload(r *http.Request) (filename, contentType string, data []byte, err error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return "", "", nil, fmt.Errorf("%w: %w", errPhotoUploadForm, err)
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return "", "", nil, errPhotoUploadMissing
		}
		if err != nil {
			return "", "", nil, fmt.Errorf("%w: %w", errPhotoUploadForm, err)
		}
		if part.FormName() != photoUploadFieldName {
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(part, propertiesapp.MaxPhotoSize+1))
		if readErr != nil {
			return "", "", nil, fmt.Errorf("%w: %w", errPhotoUploadForm, readErr)
		}
		if int64(len(data)) > propertiesapp.MaxPhotoSize {
			return "", "", nil, propertiesapp.NewPhotoTooLargeError(int64(len(data)))
		}
		return part.FileName(), part.Header.Get("Content-Type"), data, nil
	}
}

// DeletePropertyPhoto implements DELETE /properties/{propertyId}/photos/{photoId}.
func (h *PropertyHandlers) DeletePropertyPhoto(w http.ResponseWriter, r *http.Request, propertyID, photoID uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.svc.DeletePropertyPhoto(r.Context(), actor, propertyID, photoID); err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreatePropertyContact implements POST /properties/{propertyId}/contacts.
func (h *PropertyHandlers) CreatePropertyContact(w http.ResponseWriter, r *http.Request, propertyID uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyContactCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create property contact request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	contact, err := h.contactSvc.CreatePropertyContact(r.Context(), actor, propertyID, propertiesapp.CreatePropertyContactCommand{
		Name:  body.Name,
		Phone: body.Phone,
	})
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, h.propertyContactResponse(contact))
}

// ListPropertyContacts implements GET /properties/{propertyId}/contacts.
func (h *PropertyHandlers) ListPropertyContacts(w http.ResponseWriter, r *http.Request, propertyID uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	contacts, err := h.contactSvc.ListPropertyContacts(r.Context(), actor, propertyID)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	items := make([]openapi.PropertyContactResponse, 0, len(contacts))
	for _, c := range contacts {
		items = append(items, h.propertyContactResponse(c))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PropertyContactsResponse{Items: items})
}

// GetPropertyContact implements GET /properties/{propertyId}/contacts/{contactId}.
func (h *PropertyHandlers) GetPropertyContact(w http.ResponseWriter, r *http.Request, propertyID, contactID uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	contact, err := h.contactSvc.GetPropertyContact(r.Context(), actor, propertyID, contactID)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, h.propertyContactResponse(contact))
}

// UpdatePropertyContact implements PATCH /properties/{propertyId}/contacts/{contactId}.
func (h *PropertyHandlers) UpdatePropertyContact(w http.ResponseWriter, r *http.Request, propertyID, contactID uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyContactUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update property contact request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	contact, err := h.contactSvc.UpdatePropertyContact(r.Context(), actor, propertyID, contactID, propertiesapp.UpdatePropertyContactCommand{
		Name:  body.Name,
		Phone: body.Phone,
	})
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, h.propertyContactResponse(contact))
}

// DeletePropertyContact implements DELETE /properties/{propertyId}/contacts/{contactId}.
func (h *PropertyHandlers) DeletePropertyContact(w http.ResponseWriter, r *http.Request, propertyID, contactID uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.contactSvc.DeletePropertyContact(r.Context(), actor, propertyID, contactID); err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PropertyHandlers) propertyContactResponse(c domain.PropertyContact) openapi.PropertyContactResponse {
	return openapi.PropertyContactResponse{
		Id:         c.ID,
		PropertyId: c.PropertyID,
		Name:       c.Name,
		Phone:      c.Phone,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

// GetAddressSuggestions implements GET /dadata/suggestions/address.
func (h *PropertyHandlers) GetAddressSuggestions(w http.ResponseWriter, r *http.Request, params openapi.GetAddressSuggestionsParams) {
	_, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	suggestions, err := h.addressSuggester.SuggestAddresses(r.Context(), params.Query)
	if err != nil {
		if errors.Is(err, propertiesapp.ErrInvalidInput) {
			httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
				httpsupport.Problem(r.Context(), "Bad request", "Некорректный запрос адреса"))
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	resp := make([]openapi.AddressSuggestion, 0, len(suggestions))
	for _, s := range suggestions {
		city := s.City
		resp = append(resp, openapi.AddressSuggestion{Value: s.Value, City: &city})
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.AddressSuggestionsResponse{Suggestions: resp})
}

func (h *PropertyHandlers) propertyResponse(property domain.Property) openapi.PropertyResponse {
	resp := openapi.PropertyResponse{
		Id:           property.ID,
		Name:         property.Name,
		Type:         openapi.PropertyType(property.Type),
		Address:      property.Address,
		Status:       openapi.PropertyStatus(property.Status),
		MembersCount: property.MembersCount,
		CreatedAt:    property.CreatedAt,
		UpdatedAt:    property.UpdatedAt,
	}
	resp.Attributes = propertyAttributesResponse(property.Attributes)
	if property.AccessRole != "" {
		resp.Access = &openapi.PropertyAccessContext{
			Role: openapi.PropertyAccessContextRole(property.AccessRole),
		}
		if property.OwnerName != "" {
			resp.Access.OwnerName = &property.OwnerName
		}
	}
	if property.Description != "" {
		resp.Description = &property.Description
	}
	if len(property.Photos) > 0 {
		photos := make([]openapi.PropertyPhoto, 0, len(property.Photos))
		for _, p := range property.Photos {
			photos = append(photos, openapi.PropertyPhoto{Id: p.ID, Url: p.URL})
		}
		resp.Photos = &photos
	}
	return resp
}

// respondWithProperty maps the property through the presenter and writes the
// response with the given status code — 201 from CreateProperty, 200 from the
// other property endpoints.
func (h *PropertyHandlers) respondWithProperty(
	w http.ResponseWriter, r *http.Request, property domain.Property, status int,
) {
	httpsupport.WriteJSON(r.Context(), w, status, h.propertyResponse(property))
}

// Converts an optional generated PropertyAttributes value to the
// application-layer pointer-to-map. A nil value means "field omitted from PATCH";
// a non-nil pointer (even to an empty map) means "full replacement".
func propertyAttributesPtr(v *openapi.PropertyAttributes) *map[string]any {
	if v == nil {
		return nil
	}
	m := map[string]any(*v)
	return &m
}

// propertyAttributesResponse converts domain attributes to the response model.
// A nil set becomes an empty object so the required field is never omitted.
func propertyAttributesResponse(attrs domain.Attributes) openapi.PropertyAttributes {
	if attrs == nil {
		return openapi.PropertyAttributes{}
	}
	return openapi.PropertyAttributes(attrs)
}

func isInvalidStatusTransition(err error) bool {
	var target *propertiesapp.InvalidStatusTransitionError
	return errors.As(err, &target)
}
