// Package http holds the admin HTTP adapters: read-only oversight endpoints over users, properties,
// contacts, stats and audit logs.
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	adminapp "github.com/nambers/arenda-planform/apps/backend/internal/admin/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
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
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Ресурс не найден"))
	case errors.Is(err, adminapp.ErrInvalidFilter):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", detail))
	default:
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

// ListAdminUsers implements GET /admin/users.
func (h *AdminHandlers) ListAdminUsers(w http.ResponseWriter, r *http.Request, params openapi.ListAdminUsersParams) {
	filters := adminapp.AdminUserFilters{Limit: 20, Offset: 0}
	httpsupport.OptInt(&filters.Limit, params.Limit)
	httpsupport.OptInt(&filters.Offset, params.Offset)
	httpsupport.OptString(&filters.Phone, params.Phone)
	httpsupport.OptString(&filters.Email, params.Email)
	httpsupport.OptString(&filters.Role, params.Role)
	httpsupport.OptString(&filters.SubscriptionStatus, params.SubscriptionStatus)
	httpsupport.OptString(&filters.Sort, params.Sort)
	httpsupport.OptString(&filters.Order, params.Order)

	httpsupport.RespondAdminList(w, r,
		func(ctx context.Context) ([]adminapp.AdminUserView, int64, error) {
			return h.adminService.ListUsers(ctx, filters)
		},
		h.handleAdminError,
		adminUserResponse,
	)
}

// GetAdminUser implements GET /admin/users/{id}.
func (h *AdminHandlers) GetAdminUser(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	view, err := h.adminService.GetUser(r.Context(), id)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, adminUserDetailResponse(view))
}

// ListAdminUserProperties implements GET /admin/users/{id}/properties.
func (h *AdminHandlers) ListAdminUserProperties(
	w http.ResponseWriter, r *http.Request, id uuid.UUID, params openapi.ListAdminUserPropertiesParams,
) {
	filters := adminapp.AdminPropertyFilters{Limit: 20, Offset: 0}
	httpsupport.OptInt(&filters.Limit, params.Limit)
	httpsupport.OptInt(&filters.Offset, params.Offset)
	httpsupport.OptString(&filters.Status, params.Status)
	httpsupport.OptString(&filters.Sort, params.Sort)
	httpsupport.OptString(&filters.Order, params.Order)

	httpsupport.RespondAdminList(w, r,
		func(ctx context.Context) ([]adminapp.AdminPropertyView, int64, error) {
			return h.adminService.ListUserProperties(ctx, id, filters)
		},
		h.handleAdminError,
		adminPropertyResponse,
	)
}

// ListAdminProperties implements GET /admin/properties.
func (h *AdminHandlers) ListAdminProperties(w http.ResponseWriter, r *http.Request, params openapi.ListAdminPropertiesParams) {
	filters := adminapp.AdminPropertyFilters{Limit: 20, Offset: 0}
	httpsupport.OptInt(&filters.Limit, params.Limit)
	httpsupport.OptInt(&filters.Offset, params.Offset)
	httpsupport.OptString(&filters.Status, params.Status)
	httpsupport.OptString(&filters.Q, params.Q)
	if params.OwnerId != nil {
		filters.OwnerID = *params.OwnerId
	}
	httpsupport.OptString(&filters.Sort, params.Sort)
	httpsupport.OptString(&filters.Order, params.Order)

	httpsupport.RespondAdminList(w, r,
		func(ctx context.Context) ([]adminapp.AdminPropertyView, int64, error) {
			return h.adminService.ListProperties(ctx, filters)
		},
		h.handleAdminError,
		adminPropertyResponse,
	)
}

// GetAdminProperty implements GET /admin/properties/{id}.
func (h *AdminHandlers) GetAdminProperty(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	view, err := h.adminService.GetProperty(r.Context(), id)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.AdminPropertyResponse{
		Property: adminPropertyResponse(view),
	})
}

// ListAdminPropertyContacts implements GET /admin/property-contacts (the
// wire path keeps the historical name; the reads come from the contacts
// context, ADR 0051).
func (h *AdminHandlers) ListAdminPropertyContacts(w http.ResponseWriter, r *http.Request, params openapi.ListAdminPropertyContactsParams) {
	filters := adminapp.AdminContactFilters{Limit: 20, Offset: 0}
	httpsupport.OptInt(&filters.Limit, params.Limit)
	httpsupport.OptInt(&filters.Offset, params.Offset)
	if params.PropertyId != nil {
		filters.PropertyID = *params.PropertyId
	}

	httpsupport.RespondAdminList(w, r,
		func(ctx context.Context) ([]adminapp.AdminContactView, int64, error) {
			return h.adminService.ListContacts(ctx, filters)
		},
		h.handleAdminError,
		adminContactResponse,
	)
}

// GetAdminStats implements GET /admin/stats.
func (h *AdminHandlers) GetAdminStats(w http.ResponseWriter, r *http.Request) {
	view, err := h.adminService.GetStats(r.Context())
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, adminStatsResponse(view))
}

// ListAdminAuditLogs implements GET /admin/audit-logs.
func (h *AdminHandlers) ListAdminAuditLogs(w http.ResponseWriter, r *http.Request, params openapi.ListAdminAuditLogsParams) {
	filters := adminapp.AdminAuditLogFilters{Limit: 20, Offset: 0}
	httpsupport.OptInt(&filters.Limit, params.Limit)
	httpsupport.OptInt(&filters.Offset, params.Offset)
	if params.ActorId != nil {
		filters.ActorID = *params.ActorId
	}
	httpsupport.OptString(&filters.Action, params.Action)
	httpsupport.OptString(&filters.EntityType, params.EntityType)
	filters.DateFrom, filters.DateTo = auditLogDateRange(params.DateFrom, params.DateTo)
	httpsupport.OptString(&filters.Sort, params.Sort)
	httpsupport.OptString(&filters.Order, params.Order)

	httpsupport.RespondAdminList(w, r,
		func(ctx context.Context) ([]adminapp.AdminAuditLogView, int64, error) {
			return h.adminService.ListAuditLogs(ctx, filters)
		},
		h.handleAdminError,
		adminAuditLogResponse,
	)
}

// GetAdminAuditLog implements GET /admin/audit-logs/{id}.
func (h *AdminHandlers) GetAdminAuditLog(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	view, err := h.adminService.GetAuditLog(r.Context(), id)
	if err != nil {
		h.handleAdminError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.AdminAuditLogResponse{
		AuditLog: adminAuditLogResponse(view),
	})
}

// ListAdminUserAuditLogs implements GET /admin/users/{id}/audit-logs.
func (h *AdminHandlers) ListAdminUserAuditLogs(
	w http.ResponseWriter, r *http.Request, id uuid.UUID, params openapi.ListAdminUserAuditLogsParams,
) {
	filters := adminapp.AdminAuditLogFilters{Limit: 20, Offset: 0}
	httpsupport.OptInt(&filters.Limit, params.Limit)
	httpsupport.OptInt(&filters.Offset, params.Offset)
	httpsupport.OptString(&filters.Action, params.Action)
	httpsupport.OptString(&filters.EntityType, params.EntityType)
	filters.DateFrom, filters.DateTo = auditLogDateRange(params.DateFrom, params.DateTo)
	httpsupport.OptString(&filters.Sort, params.Sort)
	httpsupport.OptString(&filters.Order, params.Order)

	httpsupport.RespondAdminList(w, r,
		func(ctx context.Context) ([]adminapp.AdminAuditLogView, int64, error) {
			return h.adminService.ListUserAuditLogs(ctx, id, filters)
		},
		h.handleAdminError,
		adminAuditLogResponse,
	)
}

// The auditLogDateRange helper converts the date_from/date_to query params to
// timestamptz filter bounds. The date_from bound is inclusive (00:00:00 UTC);
// date_to is converted to an exclusive upper bound by adding 24 hours. Absent
// params yield zero times, which disable the filter.
func auditLogDateRange(from, to *openapi_types.Date) (start, end time.Time) {
	var dateFrom, dateTo time.Time
	if from != nil {
		dateFrom = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	}
	if to != nil {
		dateTo = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
	}
	return dateFrom, dateTo
}

func adminAuditLogResponse(view adminapp.AdminAuditLogView) openapi.AdminAuditLog {
	resp := openapi.AdminAuditLog{
		Id:        view.ID,
		ActorRole: openapi.AdminAuditLogActorRole(view.ActorRole),
		Action:    view.Action,
		Context:   view.Context,
		CreatedAt: view.CreatedAt,
	}
	if view.ActorID != nil {
		resp.ActorId = view.ActorID
	}
	if view.EntityType != nil {
		resp.EntityType = view.EntityType
	}
	if view.EntityID != nil {
		resp.EntityId = view.EntityID
	}
	if view.RequestID != nil {
		resp.RequestId = view.RequestID
	}
	if view.IP != nil {
		resp.Ip = view.IP
	}
	return resp
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
		},
	}
	if view.Subscription != nil {
		sub := httpsupport.SubscriptionResponse(*view.Subscription)
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
		CreatedAt:  view.CreatedAt,
		UpdatedAt:  view.UpdatedAt,
	}
	if view.Attributes != nil {
		resp.Attributes = openapi.PropertyAttributes(view.Attributes)
	} else {
		resp.Attributes = openapi.PropertyAttributes{}
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

func adminContactResponse(view adminapp.AdminContactView) openapi.AdminPropertyContact {
	return openapi.AdminPropertyContact{
		Id:         view.ID,
		PropertyId: view.PropertyID,
		Name:       view.Name,
		Phone:      view.Phone,
		CreatedAt:  view.CreatedAt,
		UpdatedAt:  view.UpdatedAt,
	}
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
		PaymentsSucceededTotalKopecksLast30d: view.PaymentsSucceededTotalKopecksLast30d,
		PaymentsFailedCountLast30d:           int(view.PaymentsFailedCountLast30d),
		PaymentsRefundedCountLast30d:         int(view.PaymentsRefundedCountLast30d),
		RecentUsers:                          recentUsers,
		RecentPayments:                       recentPayments,
	}
}
