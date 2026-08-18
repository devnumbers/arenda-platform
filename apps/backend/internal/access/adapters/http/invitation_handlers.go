package http

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"

	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// InvitationHandlers implements the generated property access invitation
// endpoints (issue #161, T5) plus the participant list, which includes pending
// invitations for managers.
type InvitationHandlers struct {
	svc    *accessapp.InvitationService
	logger *slog.Logger
}

// NewInvitationHandlers creates HTTP handlers for the property access
// invitation API.
func NewInvitationHandlers(svc *accessapp.InvitationService, logger *slog.Logger) *InvitationHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &InvitationHandlers{svc: svc, logger: logger}
}

// ListPropertyAccessMembers implements GET /properties/{propertyId}/access/members.
// The list is owner first, then membership rows, then pending email
// invitations (managers only).
func (h *InvitationHandlers) ListPropertyAccessMembers(w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	members, err := h.svc.ListMembers(r.Context(), actor, propertyID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	items := make([]openapi.PropertyAccessMemberResponse, 0, len(members))
	for _, m := range members {
		items = append(items, memberResponse(m))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PropertyAccessMembersResponse{Items: items})
}

// CreatePropertyAccessInvitation implements POST /properties/{propertyId}/access/invitations.
func (h *InvitationHandlers) CreatePropertyAccessInvitation(w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyAccessInvitationCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "access: decode create invitation request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	role, err := domain.ParseRole(string(body.Role))
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректная роль участника"))
		return
	}

	outcome, err := h.svc.InviteByEmail(r.Context(), actor, propertyID, string(body.Email), role)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	// A single response shape covers both outcomes: an instantly activated
	// membership and a stored pending invitation.
	if outcome.Member != nil {
		httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, membershipResponse(*outcome.Member))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, invitationResponse(*outcome.Invitation))
}

// UpdatePropertyAccessInvitation implements PATCH /properties/{propertyId}/access/invitations/{invitationId}.
func (h *InvitationHandlers) UpdatePropertyAccessInvitation(w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId, invitationID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PropertyAccessMemberUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "access: decode update invitation request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	role, err := domain.ParseRole(string(body.Role))
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректная роль участника"))
		return
	}

	invitation, err := h.svc.ChangeInvitationRole(r.Context(), actor, propertyID, invitationID, role)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, invitationResponse(invitation))
}

// DeletePropertyAccessInvitation implements DELETE /properties/{propertyId}/access/invitations/{invitationId}.
func (h *InvitationHandlers) DeletePropertyAccessInvitation(w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId, invitationID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.svc.CancelInvitation(r.Context(), actor, propertyID, invitationID); err != nil {
		h.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ResendPropertyAccessInvitation implements POST /properties/{propertyId}/access/invitations/{invitationId}/resend.
func (h *InvitationHandlers) ResendPropertyAccessInvitation(w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId, invitationID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.svc.ResendInvitation(r.Context(), actor, propertyID, invitationID); err != nil {
		h.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *InvitationHandlers) handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrMemberNotFound):
		// Privacy: not-found covers both "object does not exist" and "no access".
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Объект не найден"))
	case errors.Is(err, domain.ErrInvitationNotFound):
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Приглашение не найдено"))
	case errors.Is(err, domain.ErrMemberAlreadyExists):
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Пользователь уже является участником"))
	case errors.Is(err, domain.ErrInvitationAlreadyExists):
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Пользователь уже приглашён"))
	case errors.Is(err, domain.ErrPropertyArchived):
		// Same 409 shape as the properties ErrArchivedProperty mapping (issue #163).
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Нельзя добавить участника в архивный объект"))
	case errors.Is(err, domain.ErrCannotAddOwner):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Нельзя добавить владельца объекта"))
	case errors.Is(err, domain.ErrCannotAddSelf):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Нельзя добавить себя участником"))
	case errors.Is(err, domain.ErrInvalidEmail):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректный email"))
	case errors.Is(err, domain.ErrInvalidRole):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректная роль участника"))
	case errors.Is(err, domain.ErrInvitationResendCooldown):
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", resendCooldownDetail(err)))
	default:
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

// resendCooldownDetail builds the human-readable cooldown message with the
// remaining wait rounded up to whole hours.
func resendCooldownDetail(err error) string {
	var cooldown *domain.ResendCooldownError
	if errors.As(err, &cooldown) && cooldown.RetryAfter > 0 {
		hours := int(math.Ceil(cooldown.RetryAfter.Hours()))
		return "Приглашение уже отправлено, повторная отправка будет доступна через " + pluralHours(hours)
	}
	return "Приглашение уже отправлено, повторная отправка пока недоступна"
}

// pluralHours formats a hour count with the correct Russian plural.
func pluralHours(n int) string {
	switch {
	case n%100 >= 11 && n%100 <= 14:
		return fmt.Sprintf("%d часов", n)
	case n%10 == 1:
		return fmt.Sprintf("%d час", n)
	case n%10 >= 2 && n%10 <= 4:
		return fmt.Sprintf("%d часа", n)
	default:
		return fmt.Sprintf("%d часов", n)
	}
}

// membershipResponse maps an instantly activated membership (registered
// invitee) to the shared member response shape.
func membershipResponse(m domain.Membership) openapi.PropertyAccessMemberResponse {
	id := m.ID
	userID := m.UserID
	return openapi.PropertyAccessMemberResponse{
		Id:          &id,
		UserId:      &userID,
		Role:        openapi.PropertyAccessMemberRole(m.Role.String()),
		IsOwner:     false,
		Status:      openapi.PropertyAccessMemberResponseStatus(m.Status.String()),
		SuspendedAt: m.SuspendedAt,
	}
}

// invitationResponse maps a pending invitation to the shared member response
// shape: the invitation id stands in for the membership id, user_id stays
// null and the invitee email is filled.
func invitationResponse(inv domain.Invitation) openapi.PropertyAccessMemberResponse {
	id := inv.ID
	email := openapi_types.Email(inv.Email)
	lastSentAt := inv.LastSentAt
	return openapi.PropertyAccessMemberResponse{
		Id:         &id,
		Email:      &email,
		Role:       openapi.PropertyAccessMemberRole(inv.Role.String()),
		IsOwner:    false,
		Status:     openapi.PropertyAccessMemberResponseStatusPending,
		LastSentAt: &lastSentAt,
	}
}
