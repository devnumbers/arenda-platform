package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/google/uuid"
	leaseshttp "github.com/nambers/arenda-planform/apps/backend/internal/leases/adapters/http"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// PropertyHandlers implements the generated property endpoints.
type PropertyHandlers struct {
	svc              *propertiesapp.PropertyService
	addressSuggester propertiesapp.AddressSuggester
	opSvc            *leasesapp.OperationService
	leaseSvc         *leasesapp.LeaseService
	exportSvc        *leasesapp.ExportService
	contactSvc       *propertiesapp.PropertyContactService
	logger           *slog.Logger
	presenter        *leaseshttp.LeasePresenter
	clock            clock.Clock
	tzResolver       sharedtz.OwnerTimezoneResolver
}

// NewPropertyHandlers creates HTTP handlers for the properties API.
func NewPropertyHandlers(svc *propertiesapp.PropertyService, addressSuggester propertiesapp.AddressSuggester, tenantContactSvc *leasesapp.TenantContactService, opSvc *leasesapp.OperationService, leaseSvc *leasesapp.LeaseService, exportSvc *leasesapp.ExportService, contactSvc *propertiesapp.PropertyContactService, logger *slog.Logger, clk clock.Clock, tzResolver sharedtz.OwnerTimezoneResolver) *PropertyHandlers {
	return &PropertyHandlers{svc: svc, addressSuggester: addressSuggester, opSvc: opSvc, leaseSvc: leaseSvc, exportSvc: exportSvc, contactSvc: contactSvc, logger: logger, presenter: leaseshttp.NewLeasePresenter(tenantContactSvc), clock: clk, tzResolver: tzResolver}
}

func (h *PropertyHandlers) handlePropertyError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, propertiesapp.ErrInvalidInput):
		var attrErrs *propertiesapp.AttributesValidationError
		if errors.As(err, &attrErrs) {
			fieldErrors := make([]openapi.ProblemError, 0, len(attrErrs.Errors))
			for _, e := range attrErrs.Errors {
				fieldErrors = append(fieldErrors, openapi.ProblemError{Field: e.Field, Detail: e.Reason})
			}
			httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.ProblemWithFieldErrors(r.Context(), "Bad request", "Некорректные характеристики объекта", fieldErrors))
			return
		}
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", detail))
	case errors.Is(err, propertiesapp.ErrNotFound), errors.Is(err, leasesapp.ErrNotFound):
		httpsupport.WriteProblem(w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Объект не найден"))
	case errors.Is(err, propertiesapp.ErrForbidden):
		httpsupport.WriteProblem(w, http.StatusForbidden, httpsupport.Problem(r.Context(), "Forbidden", "Недостаточно прав для этого действия"))
	case errors.Is(err, propertiesapp.ErrAccessSuspended):
		// The suspended recipient gets a distinguishable 403 so the frontend
		// can show the honest "tariff limit exceeded" screen (T9, issue #158).
		httpsupport.WriteProblem(w, http.StatusForbidden, httpsupport.ProblemWithCode(r.Context(), "Forbidden", "Доступ к объекту приостановлен: превышен лимит объектов по тарифу", "membership_suspended"))
	case errors.Is(err, propertiesapp.ErrLimitExceeded):
		httpsupport.WriteProblem(w, http.StatusPaymentRequired, httpsupport.Problem(r.Context(), "Limit exceeded", "Превышен лимит активных объектов"))
	case errors.Is(err, propertiesapp.ErrArchivedProperty):
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Нельзя изменить архивный объект"))
	case errors.Is(err, propertiesapp.ErrAlreadyArchived):
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Объект уже в архиве"))
	case errors.Is(err, propertiesapp.ErrNotArchived):
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Объект не в архиве"))
	case errors.Is(err, propertiesapp.ErrPropertyHasOpenLease):
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "У объекта есть открытая аренда"))
	case errors.Is(err, propertiesapp.ErrPhotoLimitReached):
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Достигнут лимит фотографий объекта"))
	case isInvalidStatusTransition(err), errors.Is(err, propertiesapp.ErrInvalidTransition):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", detail))
	case errors.Is(err, leasesapp.ErrTenantContactNotFound):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", detail))
	default:
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

// CreateProperty implements POST /properties.
func (h *PropertyHandlers) CreateProperty(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create property request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
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

	resp, err := h.propertyResponse(r.Context(), actor, property, leasesdomain.Lease{})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, resp)
}

// ListProperties implements GET /properties.
func (h *PropertyHandlers) ListProperties(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	properties, err := h.svc.ListProperties(r.Context(), actor)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	items := make([]openapi.PropertyResponse, 0, len(properties))
	for _, property := range properties {
		resp, err := h.propertyResponse(r.Context(), actor, property, leasesdomain.Lease{})
		if err != nil {
			h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", httpsupport.SanitizeError(err)))
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		items = append(items, resp)
	}

	// Report how many shared objects are hidden from the recipient by a tariff
	// slot shortage (suspended memberships), for a footnote in the UI (issue
	// #158, T4). Owners and recipients within their limit get zero.
	hidden, err := h.svc.HiddenSharedCount(r.Context(), actor)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to count hidden shared properties", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
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
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	properties, err := h.svc.ListArchivedProperties(r.Context(), actor)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	items := make([]openapi.PropertyResponse, 0, len(properties))
	for _, property := range properties {
		resp, err := h.propertyResponse(r.Context(), actor, property, leasesdomain.Lease{})
		if err != nil {
			h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", httpsupport.SanitizeError(err)))
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		items = append(items, resp)
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PropertiesResponse{Items: items})
}

// GetProperty implements GET /properties/{id}.
func (h *PropertyHandlers) GetProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	property, activeLease, err := h.svc.GetPropertyWithOpenLease(r.Context(), actor, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	resp, err := h.propertyResponse(r.Context(), actor, property, activeLease)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// UpdateProperty implements PATCH /properties/{id}.
func (h *PropertyHandlers) UpdateProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update property request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
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

	resp, err := h.propertyResponse(r.Context(), actor, property, leasesdomain.Lease{})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// DeleteProperty implements DELETE /properties/{id}.
func (h *PropertyHandlers) DeleteProperty(w http.ResponseWriter, r *http.Request, id openapi_types.UUID, params openapi.DeletePropertyParams) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	mode, err := domain.ParseDeletePropertyMode(string(params.Mode))
	if err != nil {
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректный режим удаления"))
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
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	property, err := h.svc.ArchiveProperty(r.Context(), actor, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	resp, err := h.propertyResponse(r.Context(), actor, property, leasesdomain.Lease{})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// UnarchiveProperty implements POST /properties/{id}/unarchive.
func (h *PropertyHandlers) UnarchiveProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	property, err := h.svc.UnarchiveProperty(r.Context(), actor, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	resp, err := h.propertyResponse(r.Context(), actor, property, leasesdomain.Lease{})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to build property response", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// ListPropertyLeases implements GET /properties/{id}/leases.
func (h *PropertyHandlers) ListPropertyLeases(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	leases, err := h.svc.ListPropertyLeases(r.Context(), actor, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	contacts, err := h.presenter.TenantContactIDs(r.Context(), actor, leases)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	loc, err := h.tzResolver.Resolve(r.Context(), actor)
	if err != nil {
		h.handlePropertyError(w, r, fmt.Errorf("resolve owner timezone: %w", err))
		return
	}
	asOf := timeutil.DateIn(h.clock.Now(), loc)
	scheduleIndex, err := h.leaseSvc.LeasePaymentScheduleIndex(r.Context(), actor, leases, asOf)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	items := make([]openapi.LeaseResponse, 0, len(leases))
	for _, lease := range leases {
		schedule := scheduleIndex[lease.ID]
		resp, err := h.presenter.LeaseResponse(r.Context(), actor, lease, contacts, schedule.OverdueSince, schedule.NextPaymentDate, schedule.HasOverdue)
		if err != nil {
			h.handlePropertyError(w, r, err)
			return
		}
		items = append(items, resp)
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PropertyLeasesResponse{Items: items})
}

// GetPropertyOperationsSummary implements GET /properties/{id}/operations/summary.
func (h *PropertyHandlers) GetPropertyOperationsSummary(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	summary, err := h.opSvc.GetPropertyOperationsSummary(r.Context(), actor, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PropertyOperationsSummaryResponse{
		MonthlyProfitKopecks:  int(summary.MonthlyProfitKopecks),
		AllTimeProfitKopecks:  int(summary.AllTimeProfitKopecks),
		AllTimeIncomeKopecks:  int(summary.AllTimeIncomeKopecks),
		AllTimeExpenseKopecks: int(summary.AllTimeExpenseKopecks),
		OverdueRentCount:      summary.OverdueRentCount,
		OverdueTotalCount:     summary.OverdueTotalCount,
		NextPaymentDate:       httpsupport.DatePtrToOpenAPI(summary.NextPaymentDate),
	})
}

// ExportPropertyData implements GET /properties/{id}/export.
func (h *PropertyHandlers) ExportPropertyData(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	file, err := h.exportSvc.ExportProperty(r.Context(), actor, id)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", exportASCIIFilename(file.Filename), url.PathEscape(file.Filename)))
	_, _ = w.Write(file.Content)
}

// exportASCIIFilename replaces non-ASCII runes with underscores for the
// legacy filename= parameter of Content-Disposition.
func exportASCIIFilename(name string) string {
	return strings.Map(func(r rune) rune {
		if r > unicode.MaxASCII {
			return '_'
		}
		return r
	}, name)
}

// UploadPropertyPhoto implements POST /properties/{propertyId}/photos.
func (h *PropertyHandlers) UploadPropertyPhoto(w http.ResponseWriter, r *http.Request, propertyID uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	//nolint:gosec // 6 MiB memory bound for multipart form parsing; file size validated by the service.
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to parse multipart form", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректная форма загрузки файла"))
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	file, header, err := r.FormFile("file")
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to get file from form", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Требуется файл"))
		return
	}
	defer func() { _ = file.Close() }()

	property, err := h.svc.AddPropertyPhoto(r.Context(), actor, propertyID, file, header.Filename, header.Header.Get("Content-Type"), header.Size)
	if err != nil {
		h.handlePropertyError(w, r, err)
		return
	}

	if len(property.Photos) == 0 {
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), errors.New("uploaded photo not found")))
		return
	}

	uploaded := property.Photos[len(property.Photos)-1]
	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, openapi.PropertyPhoto{Id: uploaded.ID, Url: uploaded.URL})
}

// DeletePropertyPhoto implements DELETE /properties/{propertyId}/photos/{photoId}.
func (h *PropertyHandlers) DeletePropertyPhoto(w http.ResponseWriter, r *http.Request, propertyID, photoID uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
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
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyContactCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create property contact request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
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
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
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
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
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
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyContactUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update property contact request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
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
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
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
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	suggestions, err := h.addressSuggester.SuggestAddresses(r.Context(), params.Query)
	if err != nil {
		if errors.Is(err, propertiesapp.ErrInvalidInput) {
			httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректный запрос адреса"))
			return
		}
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}

	resp := make([]openapi.AddressSuggestion, 0, len(suggestions))
	for _, s := range suggestions {
		city := s.City
		resp = append(resp, openapi.AddressSuggestion{Value: s.Value, City: &city})
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.AddressSuggestionsResponse{Suggestions: resp})
}

func (h *PropertyHandlers) propertyResponse(ctx context.Context, actor uuid.UUID, property domain.Property, activeLease leasesdomain.Lease) (openapi.PropertyResponse, error) {
	resp := openapi.PropertyResponse{
		Id:               property.ID,
		Name:             property.Name,
		Type:             openapi.PropertyType(property.Type),
		Address:          property.Address,
		Status:           openapi.PropertyStatus(property.Status),
		Occupancy:        openapi.PropertyResponseOccupancy(property.Occupancy),
		OverdueRentCount: property.OverdueRentCount,
		MembersCount:     property.MembersCount,
		CreatedAt:        property.CreatedAt,
		UpdatedAt:        property.UpdatedAt,
	}
	resp.Attributes = propertyAttributesResponse(property.Attributes)
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
		leaseResp, err := h.presenter.LeaseResponse(ctx, actor, activeLease, nil, nil, nil, false)
		if err != nil {
			return openapi.PropertyResponse{}, fmt.Errorf("map active lease: %w", err)
		}
		resp.ActiveLease = &leaseResp
	}
	return resp, nil
}

// propertyAttributesPtr converts an optional generated PropertyAttributes value
// to the application-layer pointer-to-map. nil means "field omitted from PATCH";
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
