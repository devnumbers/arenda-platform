// Package http holds the HTTP adapters for the access bounded context
// (issue #156, T3): property membership management endpoints.
package http

import (
	"errors"
	"log/slog"
	"net/http"

	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// MemberHandlers implements the generated property access member endpoints.
type MemberHandlers struct {
	svc    *accessapp.AccessService
	logger *slog.Logger
}

// NewMemberHandlers creates HTTP handlers for the property access API.
func NewMemberHandlers(svc *accessapp.AccessService, logger *slog.Logger) *MemberHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &MemberHandlers{svc: svc, logger: logger}
}

// CreatePropertyAccessMember implements POST /properties/{propertyId}/access/members.
func (h *MemberHandlers) CreatePropertyAccessMember(w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyAccessMemberCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "access: decode create member request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	role, err := domain.ParseRole(string(body.Role))
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректная роль участника"))
		return
	}

	membership, err := h.svc.AddMember(r.Context(), actor, propertyID, body.UserId, role)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, h.membershipResponse(membership, false))
}

// ListPropertyAccessMembers lives on InvitationHandlers: the participant list
// includes pending email invitations for managers (issue #161, T5).

// UpdatePropertyAccessMember implements PATCH /properties/{propertyId}/access/members/{memberId}.
func (h *MemberHandlers) UpdatePropertyAccessMember(w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId, memberID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyAccessMemberUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "access: decode update member request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	role, err := domain.ParseRole(string(body.Role))
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректная роль участника"))
		return
	}

	membership, err := h.svc.ChangeMemberRole(r.Context(), actor, propertyID, memberID, role)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, h.membershipResponse(membership, false))
}

// DeletePropertyAccessMember implements DELETE /properties/{propertyId}/access/members/{memberId}.
func (h *MemberHandlers) DeletePropertyAccessMember(w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId, memberID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.svc.RevokeMember(r.Context(), actor, propertyID, memberID); err != nil {
		h.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LeaveProperty implements DELETE /properties/{propertyId}/access/members/self.
func (h *MemberHandlers) LeaveProperty(w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.svc.LeaveProperty(r.Context(), actor, propertyID); err != nil {
		h.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MemberHandlers) handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrMemberNotFound):
		// Privacy: not-found covers both "object does not exist" and "no access".
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Объект не найден"))
	case errors.Is(err, domain.ErrMemberAlreadyExists):
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Пользователь уже является участником"))
	case errors.Is(err, domain.ErrPropertyArchived):
		// Same 409 shape as the properties ErrArchivedProperty mapping (issue #163).
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Нельзя добавить участника в архивный объект"))
	case errors.Is(err, domain.ErrCannotAddOwner):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Нельзя добавить владельца объекта"))
	case errors.Is(err, domain.ErrCannotAddSelf):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Нельзя добавить себя участником"))
	case errors.Is(err, domain.ErrCannotRevokeOwner):
		httpsupport.WriteProblem(r.Context(), w, http.StatusForbidden, httpsupport.Problem(r.Context(), "Forbidden", "Нельзя отозвать доступ владельца"))
	case errors.Is(err, domain.ErrCannotLeaveOwnProperty):
		httpsupport.WriteProblem(r.Context(), w, http.StatusForbidden, httpsupport.Problem(r.Context(), "Forbidden", "Владелец не может покинуть свой объект"))
	case errors.Is(err, domain.ErrInvalidRole):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректная роль участника"))
	default:
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

// membershipResponse maps a domain.Membership to a response DTO. The display
// name and email flag are not available from a membership row alone; the owner
// list path uses memberResponse instead.
func (h *MemberHandlers) membershipResponse(m domain.Membership, isOwner bool) openapi.PropertyAccessMemberResponse {
	id := m.ID
	userID := m.UserID
	return openapi.PropertyAccessMemberResponse{
		Id:          &id,
		UserId:      &userID,
		Role:        openapi.PropertyAccessMemberRole(m.Role.String()),
		IsOwner:     isOwner,
		Status:      openapi.PropertyAccessMemberResponseStatus(m.Status.String()),
		SuspendedAt: m.SuspendedAt,
	}
}

// memberResponse maps an application-level participant projection (owner,
// member or pending invitation) to the shared member response shape. It is
// used by the participant list on InvitationHandlers.
func memberResponse(m accessapp.Member) openapi.PropertyAccessMemberResponse {
	resp := openapi.PropertyAccessMemberResponse{
		Role:    openapi.PropertyAccessMemberRole(string(m.Role)),
		IsOwner: m.IsOwner,
	}
	if m.Pending {
		// Pending invitation row (issue #161, T5): no user yet, the invitee
		// email is shown to managers only (the service filters the rows).
		resp.Status = openapi.PropertyAccessMemberResponseStatusPending
		if m.Email != nil {
			email := openapi_types.Email(*m.Email)
			resp.Email = &email
		}
		resp.LastSentAt = m.LastSentAt
		id := m.ID
		resp.Id = &id
		return resp
	}
	userID := m.UserID
	resp.UserId = &userID
	resp.Status = openapi.PropertyAccessMemberResponseStatus(m.Status.String())
	resp.SuspendedAt = m.SuspendedAt
	if m.DisplayName != "" {
		resp.DisplayName = &m.DisplayName
	}
	hasEmail := m.HasEmail
	resp.HasEmail = &hasEmail
	if !m.IsOwner {
		id := m.ID
		resp.Id = &id
	}
	return resp
}
