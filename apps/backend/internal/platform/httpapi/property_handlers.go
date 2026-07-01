package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
)

// PropertyHandlers implements the generated property endpoints.
type PropertyHandlers struct {
	svc              *propertiesapp.PropertyService
	addressSuggester propertiesapp.AddressSuggester
	opSvc            *leasesapp.OperationService
	logger           *slog.Logger
	presenter        *leasePresenter
}

// NewPropertyHandlers creates HTTP handlers for the properties API.
func NewPropertyHandlers(svc *propertiesapp.PropertyService, addressSuggester propertiesapp.AddressSuggester, tenantContactSvc *leasesapp.TenantContactService, opSvc *leasesapp.OperationService, logger *slog.Logger) *PropertyHandlers {
	return &PropertyHandlers{svc: svc, addressSuggester: addressSuggester, opSvc: opSvc, logger: logger, presenter: newLeasePresenter(tenantContactSvc)}
}

func (h *PropertyHandlers) handlePropertyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, propertiesapp.ErrInvalidInput):
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	case errors.Is(err, propertiesapp.ErrNotFound), errors.Is(err, leasesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "property not found"))
	case errors.Is(err, propertiesapp.ErrLimitExceeded):
		writeProblem(w, http.StatusPaymentRequired, problem(r.Context(), "Limit exceeded", "active property limit exceeded"))
	case errors.Is(err, propertiesapp.ErrArchivedProperty):
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "cannot modify an archived property"))
	case errors.Is(err, propertiesapp.ErrAlreadyArchived):
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "property is already archived"))
	case errors.Is(err, propertiesapp.ErrNotArchived):
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "property is not archived"))
	case errors.Is(err, propertiesapp.ErrPropertyHasOpenLease):
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "property has an open lease"))
	case errors.Is(err, propertiesapp.ErrPhotoLimitReached):
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "property photo limit reached"))
	case isInvalidStatusTransition(err), errors.Is(err, propertiesapp.ErrInvalidTransition):
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", detail))
	case errors.Is(err, leasesapp.ErrTenantContactNotFound):
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	default:
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
	}
}

// CreateProperty implements POST /properties.
func (h *PropertyHandlers) CreateProperty(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.PropertyCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create property request", slog.String("error", sanitizeError(err)))
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

	resp, err := h.propertyResponse(r.Context(), ownerID, property, leasesdomain.Lease{})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}
	writeJSON(r.Context(), w, http.StatusCreated, resp)
}

// ListProperties implements GET /properties.
func (h *PropertyHandlers) ListProperties(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	properties, err := h.svc.ListProperties(r.Context(), ownerID)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	items := make([]openapi.PropertyResponse, 0, len(properties))
	for _, property := range properties {
		resp, err := h.propertyResponse(r.Context(), ownerID, property, leasesdomain.Lease{})
		if err != nil {
			h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", sanitizeError(err)))
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		items = append(items, resp)
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.PropertiesResponse{Items: items})
}

// ListArchivedProperties implements GET /properties/archive.
func (h *PropertyHandlers) ListArchivedProperties(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	properties, err := h.svc.ListArchivedProperties(r.Context(), ownerID)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	items := make([]openapi.PropertyResponse, 0, len(properties))
	for _, property := range properties {
		resp, err := h.propertyResponse(r.Context(), ownerID, property, leasesdomain.Lease{})
		if err != nil {
			h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", sanitizeError(err)))
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		items = append(items, resp)
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.PropertiesResponse{Items: items})
}

// GetProperty implements GET /properties/{id}.
func (h *PropertyHandlers) GetProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	property, activeLease, err := h.svc.GetPropertyWithOpenLease(r.Context(), ownerID, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	resp, err := h.propertyResponse(r.Context(), ownerID, property, activeLease)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, resp)
}

// UpdateProperty implements PATCH /properties/{id}.
func (h *PropertyHandlers) UpdateProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.PropertyUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update property request", slog.String("error", sanitizeError(err)))
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

	resp, err := h.propertyResponse(r.Context(), ownerID, property, leasesdomain.Lease{})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, resp)
}

// ArchiveProperty implements POST /properties/{id}/archive.
func (h *PropertyHandlers) ArchiveProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	property, err := h.svc.ArchiveProperty(r.Context(), ownerID, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	resp, err := h.propertyResponse(r.Context(), ownerID, property, leasesdomain.Lease{})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, resp)
}

// UnarchiveProperty implements POST /properties/{id}/unarchive.
func (h *PropertyHandlers) UnarchiveProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	property, err := h.svc.UnarchiveProperty(r.Context(), ownerID, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	resp, err := h.propertyResponse(r.Context(), ownerID, property, leasesdomain.Lease{})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, resp)
}

// ListPropertyLeases implements GET /properties/{id}/leases.
func (h *PropertyHandlers) ListPropertyLeases(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	leases, err := h.svc.ListPropertyLeases(r.Context(), ownerID, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	contacts, err := h.presenter.tenantContactIDs(r.Context(), ownerID, leases)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	items := make([]openapi.LeaseResponse, 0, len(leases))
	for _, lease := range leases {
		resp, err := h.presenter.leaseResponse(r.Context(), ownerID, lease, contacts)
		if err != nil {
			h.handlePropertyError(w, r, err)
			return
		}
		items = append(items, resp)
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.PropertyLeasesResponse{Items: items})
}

// GetPropertyOperationsSummary implements GET /properties/{id}/operations/summary.
func (h *PropertyHandlers) GetPropertyOperationsSummary(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	summary, err := h.opSvc.GetPropertyOperationsSummary(r.Context(), ownerID, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.PropertyOperationsSummaryResponse{
		MonthlyProfitKopecks: int(summary.MonthlyProfitKopecks),
		AllTimeProfitKopecks: int(summary.AllTimeProfitKopecks),
		OverdueRentCount:     summary.OverdueRentCount,
		OverdueTotalCount:    summary.OverdueTotalCount,
		NextPaymentDate:      datePtrToOpenAPI(summary.NextPaymentDate),
	})
}

// UploadPropertyPhoto implements POST /properties/{propertyId}/photos.
func (h *PropertyHandlers) UploadPropertyPhoto(w http.ResponseWriter, r *http.Request, propertyId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	//nolint:gosec // 6 MiB memory bound for multipart form parsing; file size validated by the service.
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to parse multipart form", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid multipart form"))
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	file, header, err := r.FormFile("file")
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to get file from form", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "file is required"))
		return
	}
	defer func() { _ = file.Close() }()

	property, err := h.svc.AddPropertyPhoto(r.Context(), ownerID, propertyId, file, header.Filename, header.Header.Get("Content-Type"), header.Size)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	if len(property.Photos) == 0 {
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), errors.New("uploaded photo not found")))
		return
	}

	uploaded := property.Photos[len(property.Photos)-1]
	writeJSON(r.Context(), w, http.StatusCreated, openapi.PropertyPhoto{Id: uploaded.ID, Url: uploaded.URL})
}

// DeletePropertyPhoto implements DELETE /properties/{propertyId}/photos/{photoId}.
func (h *PropertyHandlers) DeletePropertyPhoto(w http.ResponseWriter, r *http.Request, propertyId, photoId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	if err := h.svc.DeletePropertyPhoto(r.Context(), ownerID, propertyId, photoId); err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetAddressSuggestions implements GET /dadata/suggestions/address.
func (h *PropertyHandlers) GetAddressSuggestions(w http.ResponseWriter, r *http.Request, params openapi.GetAddressSuggestionsParams) {
	_, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	suggestions, err := h.addressSuggester.SuggestAddresses(r.Context(), params.Query)
	if err != nil {
		if errors.Is(err, propertiesapp.ErrInvalidInput) {
			writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid address query"))
			return
		}
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}

	resp := make([]openapi.AddressSuggestion, 0, len(suggestions))
	for _, s := range suggestions {
		city := s.City
		resp = append(resp, openapi.AddressSuggestion{Value: s.Value, City: &city})
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.AddressSuggestionsResponse{Suggestions: resp})
}

func (h *PropertyHandlers) propertyResponse(ctx context.Context, ownerID uuid.UUID, property domain.Property, activeLease leasesdomain.Lease) (openapi.PropertyResponse, error) {
	resp := openapi.PropertyResponse{
		Id:        property.ID,
		Name:      property.Name,
		Type:      openapi.PropertyType(property.Type),
		Address:   property.Address,
		Status:    openapi.PropertyStatus(property.Status),
		Occupancy: openapi.PropertyResponseOccupancy(property.Occupancy),
		CreatedAt: property.CreatedAt,
		UpdatedAt: property.UpdatedAt,
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
	if activeLease.ID != uuid.Nil {
		leaseResp, err := h.presenter.leaseResponse(ctx, ownerID, activeLease, nil)
		if err != nil {
			return openapi.PropertyResponse{}, fmt.Errorf("map active lease: %w", err)
		}
		resp.ActiveLease = &leaseResp
	}
	return resp, nil
}

func ptrString[T ~string](v *T) *string {
	if v == nil {
		return nil
	}
	s := string(*v)
	return &s
}

func isInvalidStatusTransition(err error) bool {
	var target *propertiesapp.InvalidStatusTransitionError
	return errors.As(err, &target)
}
