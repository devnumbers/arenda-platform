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
// the participant page aggregate.
type ParticipantHandlers struct {
	svc    accessapp.ParticipantsManager
	logger *slog.Logger
}

// NewParticipantHandlers creates HTTP handlers for the owner's participants
// API.
func NewParticipantHandlers(svc accessapp.ParticipantsManager, logger *slog.Logger) *ParticipantHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &ParticipantHandlers{svc: svc, logger: logger}
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
