package http

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ParticipantHandlers implements the generated owner's-participant endpoints
// (issue #693): the «Ваши участники» list, the participants hub counters and
// the participant page aggregate — plus the aggregate mutations (issue #694):
// the multi-object invite, «Пригласить в объект» and «Отозвать и удалить».
type ParticipantHandlers struct {
	svc       accessapp.ParticipantsManager
	mutations accessapp.ParticipantMutations
	logger    *slog.Logger
}

// NewParticipantHandlers creates HTTP handlers for the owner's participants
// API. Mutations may be nil only for read-only consumers (the read tests).
func NewParticipantHandlers(
	svc accessapp.ParticipantsManager,
	mutations accessapp.ParticipantMutations,
	logger *slog.Logger,
) *ParticipantHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &ParticipantHandlers{svc: svc, mutations: mutations, logger: logger}
}

// ListParticipants implements GET /participants.
func (h *ParticipantHandlers) ListParticipants(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	participants, err := h.svc.ListParticipants(r.Context(), actor)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "access: list participants failed", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	items := make([]openapi.ParticipantResponse, 0, len(participants))
	for _, p := range participants {
		items = append(items, participantResponse(p))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.ParticipantsResponse{Items: items})
}

// GetParticipantsSummary implements GET /participants/summary.
func (h *ParticipantHandlers) GetParticipantsSummary(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	summary, err := h.svc.Summary(r.Context(), actor)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "access: participants summary failed", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.ParticipantsSummaryResponse{
		ParticipantsCount:         summary.ParticipantsCount,
		AccessiblePropertiesCount: summary.AccessiblePropertiesCount,
	})
}

// GetParticipant implements GET /participants/{participantId}.
func (h *ParticipantHandlers) GetParticipant(w http.ResponseWriter, r *http.Request, participantID string) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	participant, err := h.svc.GetParticipant(r.Context(), actor, participantID)
	if err != nil {
		if errors.Is(err, domain.ErrParticipantNotFound) {
			// Privacy: one not-found for unknown identifiers, revoked access
			// (the deep-link 404 policy) and lack of scope.
			httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Участник не найден"))
			return
		}
		h.logger.ErrorContext(r.Context(), "access: get participant failed", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
		return
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, participantResponse(participant))
}

// participantResponse maps the application aggregate to the response DTO. The
// id doubles as the stable deep-link identifier: the user's uuid, or the
// invitee email for a pending row.
func participantResponse(p accessapp.Participant) openapi.ParticipantResponse {
	resp := openapi.ParticipantResponse{
		Id:                        participantIdentifier(p),
		AggregateStatus:           openapi.ParticipantResponseAggregateStatus(p.AggregateStatus.String()),
		AccessiblePropertiesCount: p.AccessibleCount,
		Properties:                make([]openapi.ParticipantPropertyResponse, 0, len(p.Properties)),
	}
	if p.UserID != (uuid.UUID{}) {
		userID := p.UserID
		resp.UserId = &userID
	}
	if p.Email != "" {
		email := openapi_types.Email(p.Email)
		resp.Email = &email
	}
	if p.DisplayName != "" {
		displayName := p.DisplayName
		resp.DisplayName = &displayName
	}
	for _, leg := range p.Properties {
		resp.Properties = append(resp.Properties, openapi.ParticipantPropertyResponse{
			PropertyId: leg.PropertyID,
			Title:      leg.Title,
			Role:       openapi.ParticipantPropertyResponseRole(leg.Role.String()),
			Status:     openapi.ParticipantPropertyResponseStatus(leg.Status.String()),
		})
	}
	return resp
}

// participantIdentifier returns the person's addressable id: the user uuid,
// falling back to the (normalized) invitee email for a pending row.
func participantIdentifier(p accessapp.Participant) string {
	if p.UserID != (uuid.UUID{}) {
		return p.UserID.String()
	}
	return p.Email
}

// InviteParticipant implements POST /participants/invite — the multi-object
// invitation with one role (issue #694).
func (h *ParticipantHandlers) InviteParticipant(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.mutationsActor(w, r)
	if !ok {
		return
	}

	var body openapi.InviteParticipantJSONRequestBody
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.decodeError(w, r, err)
		return
	}
	role, ids, ok := h.parseGrantRequest(w, r, string(body.Role), body.PropertyIds)
	if !ok {
		return
	}

	results, err := h.mutations.Invite(r.Context(), actor, string(body.Email), role, ids)
	if err != nil {
		h.handleMutationError(w, r, err)
		return
	}
	h.writeGrantResults(w, r, results)
}

// AddParticipantProperties implements POST /participants/{participantId}/properties —
// «Пригласить в объект» on the participant page (issue #694).
func (h *ParticipantHandlers) AddParticipantProperties(w http.ResponseWriter, r *http.Request, participantID string) {
	actor, ok := h.mutationsActor(w, r)
	if !ok {
		return
	}

	var body openapi.AddParticipantPropertiesJSONRequestBody
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.decodeError(w, r, err)
		return
	}
	role, ids, ok := h.parseGrantRequest(w, r, string(body.Role), body.PropertyIds)
	if !ok {
		return
	}

	results, err := h.mutations.AddProperties(r.Context(), actor, participantID, role, ids)
	if err != nil {
		h.handleMutationError(w, r, err)
		return
	}
	h.writeGrantResults(w, r, results)
}

// DeleteParticipant implements DELETE /participants/{participantId} —
// «Отозвать и удалить» (issue #694).
func (h *ParticipantHandlers) DeleteParticipant(w http.ResponseWriter, r *http.Request, participantID string) {
	actor, ok := h.mutationsActor(w, r)
	if !ok {
		return
	}

	if err := h.mutations.Remove(r.Context(), actor, participantID); err != nil {
		h.handleMutationError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// mutationsActor resolves the session user or writes the 401; ok=false means
// the response is already written.
func (h *ParticipantHandlers) mutationsActor(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return uuid.UUID{}, false
	}
	return actor, true
}

// decodeError writes the shared bad-request problem for a malformed body.
func (h *ParticipantHandlers) decodeError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.ErrorContext(r.Context(), "access: decode participant mutation request", slog.String("error", httpsupport.SanitizeError(err)))
	httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
		httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
}

// parseGrantRequest validates the shared (role, property_ids) batch fields.
// An ok=false result means the response is already written.
func (h *ParticipantHandlers) parseGrantRequest(
	w http.ResponseWriter, r *http.Request, rawRole string, propertyIDs []openapi_types.UUID,
) (domain.Role, []uuid.UUID, bool) {
	role, err := domain.ParseRole(rawRole)
	if err != nil {
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректная роль участника"))
		return "", nil, false
	}
	if len(propertyIDs) == 0 {
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Укажите хотя бы один объект"))
		return "", nil, false
	}
	// No conversion is needed: the openapi_types.UUID is a uuid.UUID alias.
	ids := make([]uuid.UUID, 0, len(propertyIDs))
	ids = append(ids, propertyIDs...)
	return role, ids, true
}

// writeGrantResults writes the unified per-property batch response.
func (h *ParticipantHandlers) writeGrantResults(w http.ResponseWriter, r *http.Request, results []accessapp.ParticipantGrantResult) {
	items := make([]openapi.ParticipantGrantResult, 0, len(results))
	for _, result := range results {
		items = append(items, grantResultResponse(result))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.ParticipantGrantResultsResponse{Items: items})
}

// grantResultResponse maps an application grant outcome to the response DTO.
func grantResultResponse(r accessapp.ParticipantGrantResult) openapi.ParticipantGrantResult {
	resp := openapi.ParticipantGrantResult{
		PropertyId: r.PropertyID,
		Outcome:    openapi.ParticipantGrantOutcome(r.Outcome.String()),
	}
	if r.MembershipID != (uuid.UUID{}) {
		membershipID := r.MembershipID
		resp.MembershipId = &membershipID
	}
	if r.InvitationID != (uuid.UUID{}) {
		invitationID := r.InvitationID
		resp.InvitationId = &invitationID
	}
	return resp
}

// handleMutationError maps the participant mutation outcomes to problem
// responses. Like the per-property access API, a missing/unmanageable person
// or object is a privacy-preserving 404/4xx, never a 500.
func (h *ParticipantHandlers) handleMutationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrParticipantNotFound):
		// Privacy: one not-found for unknown identifiers, revoked access
		// (the deep-link 404 policy) and lack of scope.
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Участник не найден"))
	case errors.Is(err, domain.ErrCannotAddSelf):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Нельзя добавить себя участником"))
	case errors.Is(err, domain.ErrInvalidEmail):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректный email"))
	case errors.Is(err, domain.ErrInvalidRole):
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректная роль участника"))
	default:
		h.logger.ErrorContext(r.Context(), "access: participant mutation failed", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}
