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
	if params.Sort != nil {
		filters.Sort = *params.Sort
	}
	if params.Order != nil {
		filters.Order = string(*params.Order)
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
	if params.Sort != nil {
		filters.Sort = *params.Sort
	}
	if params.Order != nil {
		filters.Order = string(*params.Order)
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

// ListAdminProperties implements GET /admin/properties.
func (h *AdminHandlers) ListAdminProperties(w http.ResponseWriter, r *http.Request, params openapi.ListAdminPropertiesParams) {
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
	if params.Q != nil {
		filters.Q = *params.Q
	}
	if params.OwnerId != nil {
		filters.OwnerID = *params.OwnerId
	}
	if params.Sort != nil {
		filters.Sort = *params.Sort
	}
	if params.Order != nil {
		filters.Order = string(*params.Order)
	}

	views, total, err := h.adminService.ListProperties(r.Context(), filters)
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
	filters := adminapp.AdminLeaseFilters{Limit: 20, Offset: 0}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}
	if params.Sort != nil {
		filters.Sort = *params.Sort
	}
	if params.Order != nil {
		filters.Order = string(*params.Order)
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

// ListAdminLeases implements GET /admin/leases.
func (h *AdminHandlers) ListAdminLeases(w http.ResponseWriter, r *http.Request, params openapi.ListAdminLeasesParams) {
	filters := adminapp.AdminLeaseFilters{Limit: 20, Offset: 0}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}
	if params.Status != nil {
		filters.Status = string(*params.Status)
	}
	if params.PropertyId != nil {
		filters.PropertyID = *params.PropertyId
	}
	if params.OwnerId != nil {
		filters.OwnerID = *params.OwnerId
	}
	if params.Sort != nil {
		filters.Sort = *params.Sort
	}
	if params.Order != nil {
		filters.Order = string(*params.Order)
	}

	views, total, err := h.adminService.ListLeases(r.Context(), filters)
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
	filters := adminapp.AdminTenantContactFilters{Limit: 20, Offset: 0}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}
	if params.Sort != nil {
		filters.Sort = *params.Sort
	}
	if params.Order != nil {
		filters.Order = string(*params.Order)
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

// ListAdminTenantContacts implements GET /admin/tenant-contacts.
func (h *AdminHandlers) ListAdminTenantContacts(w http.ResponseWriter, r *http.Request, params openapi.ListAdminTenantContactsParams) {
	filters := adminapp.AdminTenantContactFilters{Limit: 20, Offset: 0}
	if params.Limit != nil {
		filters.Limit = *params.Limit
	}
	if params.Offset != nil {
		filters.Offset = *params.Offset
	}
	if params.Q != nil {
		filters.Q = *params.Q
	}
	if params.OwnerId != nil {
		filters.OwnerID = *params.OwnerId
	}
	if params.Sort != nil {
		filters.Sort = *params.Sort
	}
	if params.Order != nil {
		filters.Order = string(*params.Order)
	}

	views, total, err := h.adminService.ListTenantContacts(r.Context(), filters)
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
	if params.Sort != nil {
		filters.Sort = *params.Sort
	}
	if params.Order != nil {
		filters.Order = string(*params.Order)
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

// ListAdminOperations implements GET /admin/operations.
func (h *AdminHandlers) ListAdminOperations(w http.ResponseWriter, r *http.Request, params openapi.ListAdminOperationsParams) {
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
	if params.OwnerId != nil {
		filters.OwnerID = *params.OwnerId
	}
	if params.Q != nil {
		filters.Q = *params.Q
	}
	if params.Sort != nil {
		filters.Sort = *params.Sort
	}
	if params.Order != nil {
		filters.Order = string(*params.Order)
	}

	views, total, err := h.adminService.ListOperations(r.Context(), filters)
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

// GetAdminStats implements GET /admin/stats.
func (h *AdminHandlers) GetAdminStats(w http.ResponseWriter, r *http.Request) {
	view, err := h.adminService.GetStats(r.Context())
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, adminStatsResponse(view))
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
	if view.SubscriptionStatus != "" {
		status := openapi.SubscriptionStatus(view.SubscriptionStatus)
		resp.SubscriptionStatus = &status
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
		Id:         view.ID,
		OwnerId:    view.OwnerID,
		OwnerPhone: view.OwnerPhone,
		Name:       view.Name,
		Type:       openapi.PropertyType(view.Type),
		Address:    view.Address,
		Status:     openapi.PropertyStatus(view.Status),
		Occupancy:  openapi.AdminPropertyOccupancy(view.Occupancy),
		CreatedAt:  view.CreatedAt,
		UpdatedAt:  view.UpdatedAt,
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
		PropertyName:         view.PropertyName,
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
		contact := adminTenantContactResponse(adminapp.AdminTenantContactView{TenantContact: *view.TenantContact})
		resp.TenantContact = &contact
	}
	return resp
}

func adminTenantContactResponse(view adminapp.AdminTenantContactView) openapi.AdminTenantContact {
	resp := openapi.AdminTenantContact{
		Id:         view.ID,
		OwnerId:    view.OwnerID,
		OwnerPhone: view.OwnerPhone,
		Name:       view.Name,
		CreatedAt:  view.CreatedAt,
		UpdatedAt:  view.UpdatedAt,
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
		PropertyName:  view.PropertyName,
		Type:          openapi.OperationType(view.Type),
		CategoryId:    view.CategoryID,
		CategoryName:  view.CategoryName,
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

func adminStatsResponse(view adminapp.AdminStatsView) openapi.AdminStats {
	recentUsers := make([]openapi.AdminStatsRecentUser, 0, len(view.RecentUsers))
	for _, u := range view.RecentUsers {
		item := openapi.AdminStatsRecentUser{
			Id:        u.ID,
			Phone:     u.Phone,
			CreatedAt: u.CreatedAt,
		}
		if u.Name != nil {
			item.Name = u.Name
		}
		if u.Surname != nil {
			item.Surname = u.Surname
		}
		recentUsers = append(recentUsers, item)
	}

	recentPayments := make([]openapi.AdminStatsRecentPayment, 0, len(view.RecentPayments))
	for _, p := range view.RecentPayments {
		recentPayments = append(recentPayments, openapi.AdminStatsRecentPayment{
			Id:            p.ID,
			UserId:        p.UserID,
			UserPhone:     p.UserPhone,
			AmountKopecks: p.AmountKopecks,
			Status:        openapi.SubscriptionPaymentStatus(p.Status),
			CreatedAt:     p.CreatedAt,
		})
	}

	return openapi.AdminStats{
		UsersTotal:                           int(view.UsersTotal),
		UsersNewLast30d:                      int(view.UsersNewLast30d),
		SubscriptionsActive:                  int(view.SubscriptionsActive),
		PropertiesActive:                     int(view.PropertiesActive),
		PropertiesArchived:                   int(view.PropertiesArchived),
		LeasesTotal:                          int(view.LeasesTotal),
		OperationsTotal:                      int(view.OperationsTotal),
		PaymentsSucceededTotalKopecksLast30d: view.PaymentsSucceededTotalKopecksLast30d,
		PaymentsFailedCountLast30d:           int(view.PaymentsFailedCountLast30d),
		PaymentsRefundedCountLast30d:         int(view.PaymentsRefundedCountLast30d),
		RecentUsers:                          recentUsers,
		RecentPayments:                       recentPayments,
	}
}
