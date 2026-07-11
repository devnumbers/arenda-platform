package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	adminapp "github.com/nambers/arenda-planform/apps/backend/internal/admin/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// AdminHandlers implements the generated admin endpoints.
type AdminHandlers struct {
	adminService *adminapp.AdminService
	logger       *slog.Logger
}

// NewAdminHandlers creates HTTP handlers for the admin API.
func NewAdminHandlers(adminService *adminapp.AdminService, logger *slog.Logger) *AdminHandlers {
	return &AdminHandlers{adminService: adminService, logger: logger}
}

func (h *AdminHandlers) handleAdminError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, adminapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "Ресурс не найден"))
	case errors.Is(err, adminapp.ErrInvalidFilter):
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

// ListAdminUsers implements GET /admin/users.
func (h *AdminHandlers) ListAdminUsers(w http.ResponseWriter, r *http.Request, params openapi.ListAdminUsersParams) {
	filters := adminapp.AdminUserFilters{Limit: 20, Offset: 0}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}
	if params.Phone != nil {
		filters.Phone = *params.Phone
	}
	if params.Email != nil {
		filters.Email = *params.Email
	}
	if params.Role != nil {
		filters.Role = string(*params.Role)
	}
	if params.SubscriptionStatus != nil {
		filters.SubscriptionStatus = string(*params.SubscriptionStatus)
	}

	views, total, err := h.adminService.ListUsers(r.Context(), filters)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	items := make([]openapi.AdminUser, 0, len(views))
	for _, v := range views {
		items = append(items, adminUserResponse(v))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.AdminUsersResponse{
		Items: items,
		Total: int(total),
	})
}

// GetAdminUser implements GET /admin/users/{id}.
func (h *AdminHandlers) GetAdminUser(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	view, err := h.adminService.GetUser(r.Context(), id)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, adminUserDetailResponse(view))
}

// ListAdminUserProperties implements GET /admin/users/{id}/properties.
func (h *AdminHandlers) ListAdminUserProperties(w http.ResponseWriter, r *http.Request, id uuid.UUID, params openapi.ListAdminUserPropertiesParams) {
	filters := adminapp.AdminPropertyFilters{Limit: 20, Offset: 0}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}
	if params.Status != nil {
		filters.Status = string(*params.Status)
	}

	views, total, err := h.adminService.ListUserProperties(r.Context(), id, filters)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	items := make([]openapi.AdminProperty, 0, len(views))
	for _, v := range views {
		items = append(items, adminPropertyResponse(v))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.AdminPropertiesResponse{
		Items: items,
		Total: int(total),
	})
}

// GetAdminProperty implements GET /admin/properties/{id}.
func (h *AdminHandlers) GetAdminProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	view, err := h.adminService.GetProperty(r.Context(), id)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.AdminPropertyResponse{
		Property: adminPropertyResponse(view),
	})
}

// ListAdminUserLeases implements GET /admin/users/{id}/leases.
func (h *AdminHandlers) ListAdminUserLeases(w http.ResponseWriter, r *http.Request, id uuid.UUID, params openapi.ListAdminUserLeasesParams) {
	filters := adminapp.AdminListFilters{Limit: 20, Offset: 0}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}

	views, total, err := h.adminService.ListUserLeases(r.Context(), id, filters)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	items := make([]openapi.AdminLease, 0, len(views))
	for _, v := range views {
		items = append(items, adminLeaseResponse(v))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.AdminLeasesResponse{
		Items: items,
		Total: int(total),
	})
}

// GetAdminLease implements GET /admin/leases/{id}.
func (h *AdminHandlers) GetAdminLease(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	view, err := h.adminService.GetLease(r.Context(), id)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.AdminLeaseResponse{
		Lease: adminLeaseResponse(view),
	})
}

// ListAdminUserTenantContacts implements GET /admin/users/{id}/tenant-contacts.
func (h *AdminHandlers) ListAdminUserTenantContacts(w http.ResponseWriter, r *http.Request, id uuid.UUID, params openapi.ListAdminUserTenantContactsParams) {
	filters := adminapp.AdminListFilters{Limit: 20, Offset: 0}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}

	views, total, err := h.adminService.ListUserTenantContacts(r.Context(), id, filters)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	items := make([]openapi.AdminTenantContact, 0, len(views))
	for _, v := range views {
		items = append(items, adminTenantContactResponse(v))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.AdminTenantContactsResponse{
		Items: items,
		Total: int(total),
	})
}

// GetAdminTenantContact implements GET /admin/tenant-contacts/{id}.
func (h *AdminHandlers) GetAdminTenantContact(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	view, err := h.adminService.GetTenantContact(r.Context(), id)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.AdminTenantContactResponse{
		Contact: adminTenantContactResponse(view),
	})
}

// ListAdminUserOperations implements GET /admin/users/{id}/operations.
func (h *AdminHandlers) ListAdminUserOperations(w http.ResponseWriter, r *http.Request, id uuid.UUID, params openapi.ListAdminUserOperationsParams) {
	filters := adminapp.AdminOperationFilters{Limit: 20, Offset: 0}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}
	if params.Status != nil {
		filters.Status = string(*params.Status)
	}
	if params.Type != nil {
		filters.Type = string(*params.Type)
	}
	if params.PropertyId != nil {
		filters.PropertyID = *params.PropertyId
	}
	if params.LeaseId != nil {
		filters.LeaseID = *params.LeaseId
	}

	views, total, err := h.adminService.ListUserOperations(r.Context(), id, filters)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	items := make([]openapi.AdminOperation, 0, len(views))
	for _, v := range views {
		items = append(items, adminOperationResponse(v))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.AdminOperationsResponse{
		Items: items,
		Total: int(total),
	})
}

// GetAdminOperation implements GET /admin/operations/{id}.
func (h *AdminHandlers) GetAdminOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	view, err := h.adminService.GetOperation(r.Context(), id)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.AdminOperationResponse{
		Operation: adminOperationResponse(view),
	})
}

func adminUserResponse(view adminapp.AdminUserView) openapi.AdminUser {
	resp := openapi.AdminUser{
		Id:        view.ID,
		Phone:     view.Phone,
		Role:      openapi.AdminUserRole(view.Role),
		CreatedAt: view.CreatedAt,
		UpdatedAt: view.UpdatedAt,
	}
	if view.Name != nil {
		resp.Name = view.Name
	}
	if view.Surname != nil {
		resp.Surname = view.Surname
	}
	if view.Patronymic != nil {
		resp.Patronymic = view.Patronymic
	}
	if view.Email != nil {
		resp.Email = view.Email
	}
	return resp
}

func adminUserDetailResponse(view adminapp.AdminUserDetailView) openapi.AdminUserResponse {
	resp := openapi.AdminUserResponse{
		User: adminUserResponse(view.User),
		Stats: openapi.AdminUserStats{
			ActivePropertiesCount:   int(view.Stats.ActivePropertiesCount),
			ArchivedPropertiesCount: int(view.Stats.ArchivedPropertiesCount),
			LeasesCount:             int(view.Stats.LeasesCount),
			OperationsCount:         int(view.Stats.OperationsCount),
			TenantContactsCount:     int(view.Stats.TenantContactsCount),
		},
	}
	if view.Subscription != nil {
		sub := subscriptionResponse(*view.Subscription)
		resp.Subscription = &sub
	}
	return resp
}

func adminPropertyResponse(view adminapp.AdminPropertyView) openapi.AdminProperty {
	resp := openapi.AdminProperty{
		Id:        view.ID,
		OwnerId:   view.OwnerID,
		Name:      view.Name,
		Type:      openapi.PropertyType(view.Type),
		Address:   view.Address,
		Status:    openapi.PropertyStatus(view.Status),
		Occupancy: openapi.AdminPropertyOccupancy(view.Occupancy),
		CreatedAt: view.CreatedAt,
		UpdatedAt: view.UpdatedAt,
	}
	if view.Description != nil {
		resp.Description = view.Description
	}
	if len(view.Photos) > 0 {
		photos := make([]openapi.PropertyPhoto, 0, len(view.Photos))
		for _, p := range view.Photos {
			photos = append(photos, openapi.PropertyPhoto{Id: p.ID, Url: p.URL})
		}
		resp.Photos = &photos
	}
	return resp
}

func adminLeaseResponse(view adminapp.AdminLeaseView) openapi.AdminLease {
	resp := openapi.AdminLease{
		Id:                   view.ID,
		OwnerId:              view.OwnerID,
		PropertyId:           view.PropertyID,
		Status:               openapi.LeaseStatus(view.Status),
		StartDate:            openapi_types.Date{Time: view.StartDate},
		RentAmountKopecks:    int(view.RentAmountKopecks),
		DepositAmountKopecks: int(view.DepositAmountKopecks),
		PaymentDay:           view.PaymentDay,
		CreatedAt:            view.CreatedAt,
		UpdatedAt:            view.UpdatedAt,
	}
	if view.EndDate != nil {
		resp.EndDate = &openapi_types.Date{Time: *view.EndDate}
	}
	if view.Comment != "" {
		resp.Comment = &view.Comment
	}
	if view.TenantContact != nil {
		contact := adminTenantContactResponse(adminapp.AdminTenantContactView(*view.TenantContact))
		resp.TenantContact = &contact
	}
	return resp
}

func adminTenantContactResponse(view adminapp.AdminTenantContactView) openapi.AdminTenantContact {
	resp := openapi.AdminTenantContact{
		Id:        view.ID,
		OwnerId:   view.OwnerID,
		Name:      view.Name,
		CreatedAt: view.CreatedAt,
		UpdatedAt: view.UpdatedAt,
	}
	if view.Surname != nil {
		resp.Surname = view.Surname
	}
	if view.Patronymic != nil {
		resp.Patronymic = view.Patronymic
	}
	if view.Phone != nil {
		resp.Phone = view.Phone
	}
	if view.Email != nil {
		resp.Email = view.Email
	}
	if view.Comment != nil {
		resp.Comment = view.Comment
	}
	return resp
}

func adminOperationResponse(view adminapp.AdminOperationView) openapi.AdminOperation {
	resp := openapi.AdminOperation{
		Id:            view.ID,
		OwnerId:       view.OwnerID,
		PropertyId:    view.PropertyID,
		Type:          openapi.OperationType(view.Type),
		Category:      openapi.OperationCategory(view.Category),
		Name:          view.Name,
		AmountKopecks: int(view.AmountKopecks),
		OperationDate: openapi_types.Date{Time: view.OperationDate},
		Status:        openapi.OperationStatus(view.Status),
		IsException:   view.IsException,
		CreatedAt:     view.CreatedAt,
		UpdatedAt:     view.UpdatedAt,
	}
	if view.LeaseID != nil {
		resp.LeaseId = view.LeaseID
	}
	if view.RecurringOperationID != nil {
		resp.RecurringOperationId = view.RecurringOperationID
	}
	if view.Comment != nil {
		resp.Comment = view.Comment
	}
	if view.ReminderOffsetDays != nil {
		offset := openapi.AdminOperationReminderOffsetDays(*view.ReminderOffsetDays)
		resp.ReminderOffsetDays = &offset
	}
	return resp
}
