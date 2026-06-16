package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

// PropertyHandlers implements the generated property endpoints.
type PropertyHandlers struct {
	svc    *propertiesapp.PropertyService
	logger *slog.Logger
}

// NewPropertyHandlers creates HTTP handlers for the properties API.
func NewPropertyHandlers(svc *propertiesapp.PropertyService, logger *slog.Logger) *PropertyHandlers {
	return &PropertyHandlers{svc: svc, logger: logger}
}

func (h *PropertyHandlers) ownerIDFromContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return uuid.UUID{}, false
	}
	return userID, true
}

func (h *PropertyHandlers) handlePropertyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, propertiesapp.ErrInvalidInput):
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", err.Error()))
	case errors.Is(err, propertiesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "property not found"))
	case errors.Is(err, propertiesapp.ErrLimitExceeded):
		writeProblem(w, http.StatusPaymentRequired, problem(r.Context(), "Limit exceeded", "active property limit exceeded"))
	case errors.Is(err, propertiesapp.ErrInvalidTransition):
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", err.Error()))
	default:
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
	}
}

// CreateProperty implements POST /properties.
func (h *PropertyHandlers) CreateProperty(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	var body openapi.PropertyCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create property request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	description := ""
	if body.Description != nil {
		description = *body.Description
	}

	cmd := propertiesapp.CreatePropertyCommand{
		Name:        body.Name,
		Type:        string(body.Type),
		Address:     body.Address,
		Description: description,
	}

	property, err := h.svc.CreateProperty(r.Context(), ownerID, cmd)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, propertyResponse(property))
}

// ListProperties implements GET /properties.
func (h *PropertyHandlers) ListProperties(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	properties, err := h.svc.ListProperties(r.Context(), ownerID)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	items := make([]openapi.PropertyResponse, 0, len(properties))
	for _, property := range properties {
		items = append(items, propertyResponse(property))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.PropertiesResponse{Items: items})
}

// GetProperty implements GET /properties/{id}.
func (h *PropertyHandlers) GetProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	property, err := h.svc.GetProperty(r.Context(), ownerID, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, propertyResponse(property))
}

// UpdateProperty implements PATCH /properties/{id}.
func (h *PropertyHandlers) UpdateProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	var body openapi.PropertyUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update property request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	cmd := propertiesapp.UpdatePropertyCommand{
		Name:        body.Name,
		Type:        ptrString(body.Type),
		Address:     body.Address,
		Description: body.Description,
		Status:      ptrString(body.Status),
	}

	property, err := h.svc.UpdateProperty(r.Context(), ownerID, id, cmd)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, propertyResponse(property))
}

// ArchiveProperty implements POST /properties/{id}/archive.
func (h *PropertyHandlers) ArchiveProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	property, err := h.svc.ArchiveProperty(r.Context(), ownerID, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, propertyResponse(property))
}

// UnarchiveProperty implements POST /properties/{id}/unarchive.
func (h *PropertyHandlers) UnarchiveProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	property, err := h.svc.UnarchiveProperty(r.Context(), ownerID, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, propertyResponse(property))
}

func propertyResponse(property domain.Property) openapi.PropertyResponse {
	resp := openapi.PropertyResponse{
		Id:        property.ID,
		Name:      property.Name,
		Type:      openapi.PropertyResponseType(property.Type),
		Address:   property.Address,
		Status:    openapi.PropertyResponseStatus(property.Status),
		CreatedAt: property.CreatedAt,
		UpdatedAt: property.UpdatedAt,
	}
	if property.Description != "" {
		resp.Description = &property.Description
	}
	return resp
}

func ptrString[T ~string](v *T) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}
