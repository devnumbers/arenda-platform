package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// FreeReminderHandlers implements the generated free-reminder endpoints.
type FreeReminderHandlers struct {
	svc        *notificationsapp.FreeReminderService
	reminders  *notificationsapp.ReminderService
	properties *propertiesapp.PropertyService
	clock      clock.Clock
	logger     *slog.Logger
}

// NewFreeReminderHandlers creates HTTP handlers for the free reminders API.
func NewFreeReminderHandlers(
	svc *notificationsapp.FreeReminderService,
	reminders *notificationsapp.ReminderService,
	properties *propertiesapp.PropertyService,
	clock clock.Clock,
	logger *slog.Logger,
) *FreeReminderHandlers {
	return &FreeReminderHandlers{svc: svc, reminders: reminders, properties: properties, clock: clock, logger: logger}
}

func (h *FreeReminderHandlers) handleFreeReminderError(w http.ResponseWriter, r *http.Request, err error, resource string) {
	switch {
	case errors.Is(err, notificationsapp.ErrNotFound),
		errors.Is(err, propertiesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", freeReminderNotFoundMessage(resource)))
	case errors.Is(err, notificationsapp.ErrInvalidFreeReminderInput),
		errors.Is(err, notificationsapp.ErrInvalidReminderDate):
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

func freeReminderNotFoundMessage(resource string) string {
	switch resource {
	case "property":
		return "Объект недвижимости не найден"
	case "free_reminder":
		return "Напоминание не найдено"
	default:
		return "Не найдено"
	}
}

// CreateFreeReminder implements POST /properties/{propertyId}/free-reminders.
func (h *FreeReminderHandlers) CreateFreeReminder(w http.ResponseWriter, r *http.Request, propertyID uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if _, err := h.properties.GetProperty(r.Context(), ownerID, propertyID); err != nil {
		h.handleFreeReminderError(w, r, err, "property")
		return
	}

	var body openapi.FreeReminderCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create free reminder request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	input := notificationsapp.CreateFreeReminderInput{
		PropertyID:  propertyID,
		Title:       body.Title,
		TriggerAt:   body.TriggerAt,
		Periodicity: notificationsdomain.FreeReminderPeriodicity(body.Periodicity),
	}

	fr, err := h.svc.Create(r.Context(), ownerID, input)
	if err != nil {
		h.handleFreeReminderError(w, r, err, "free_reminder")
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, freeReminderResponse(fr))
}

// ListPropertyFreeReminders implements GET /properties/{propertyId}/free-reminders.
func (h *FreeReminderHandlers) ListPropertyFreeReminders(w http.ResponseWriter, r *http.Request, propertyID uuid.UUID, params openapi.ListPropertyFreeRemindersParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if _, err := h.properties.GetProperty(r.Context(), ownerID, propertyID); err != nil {
		h.handleFreeReminderError(w, r, err, "property")
		return
	}

	limit := 3
	if params.Limit != nil {
		limit = min(*params.Limit, 100)
		limit = max(limit, 1)
	}

	reminders, err := h.svc.ListByProperty(r.Context(), ownerID, propertyID, limit)
	if err != nil {
		h.handleFreeReminderError(w, r, err, "free_reminder")
		return
	}

	items := make([]openapi.FreeReminderResponse, 0, len(reminders))
	for _, fr := range reminders {
		items = append(items, freeReminderResponse(fr))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.FreeRemindersResponse{Items: items})
}

// ListUpcomingFreeReminders implements GET
// /properties/{propertyId}/free-reminders/upcoming. It returns the nearest
// pending occurrences (including periodic ones) instead of templates, for the
// property-page "up to 3 nearest" block.
func (h *FreeReminderHandlers) ListUpcomingFreeReminders(w http.ResponseWriter, r *http.Request, propertyID uuid.UUID, params openapi.ListUpcomingFreeRemindersParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if _, err := h.properties.GetProperty(r.Context(), ownerID, propertyID); err != nil {
		h.handleFreeReminderError(w, r, err, "property")
		return
	}

	limit := 3
	if params.Limit != nil {
		limit = min(*params.Limit, 100)
		limit = max(limit, 1)
	}

	upcoming, err := h.reminders.ListUpcomingFreeRemindersByProperty(r.Context(), ownerID, propertyID, h.clock.Now(), limit)
	if err != nil {
		h.handleFreeReminderError(w, r, err, "free_reminder")
		return
	}

	items := make([]openapi.UpcomingFreeReminderResponse, 0, len(upcoming))
	for _, u := range upcoming {
		items = append(items, upcomingFreeReminderResponse(u))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.UpcomingFreeRemindersResponse{Items: items})
}

// ListFreeReminders implements GET /free-reminders.
func (h *FreeReminderHandlers) ListFreeReminders(w http.ResponseWriter, r *http.Request, params openapi.ListFreeRemindersParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	limit := 100
	if params.Limit != nil {
		limit = min(*params.Limit, 1000)
		limit = max(limit, 1)
	}

	offset := 0
	if params.Offset != nil {
		offset = max(*params.Offset, 0)
	}

	reminders, err := h.svc.ListByOwner(r.Context(), ownerID, limit, offset)
	if err != nil {
		h.handleFreeReminderError(w, r, err, "free_reminder")
		return
	}

	items := make([]openapi.FreeReminderResponse, 0, len(reminders))
	for _, fr := range reminders {
		items = append(items, freeReminderResponse(fr))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.FreeRemindersResponse{Items: items})
}

// GetFreeReminder implements GET /free-reminders/{freeReminderId}.
func (h *FreeReminderHandlers) GetFreeReminder(w http.ResponseWriter, r *http.Request, freeReminderID uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	fr, err := h.svc.Get(r.Context(), ownerID, freeReminderID)
	if err != nil {
		h.handleFreeReminderError(w, r, err, "free_reminder")
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, freeReminderResponse(fr))
}

// UpdateFreeReminder implements PATCH /free-reminders/{freeReminderId}.
func (h *FreeReminderHandlers) UpdateFreeReminder(w http.ResponseWriter, r *http.Request, freeReminderID uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.FreeReminderUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update free reminder request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	input := notificationsapp.UpdateFreeReminderInput{}
	if body.Title != nil {
		input.Title = body.Title
	}
	if body.TriggerAt != nil {
		input.TriggerAt = body.TriggerAt
	}
	if body.Periodicity != nil {
		p := notificationsdomain.FreeReminderPeriodicity(*body.Periodicity)
		input.Periodicity = &p
	}

	fr, err := h.svc.Update(r.Context(), ownerID, freeReminderID, input)
	if err != nil {
		h.handleFreeReminderError(w, r, err, "free_reminder")
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, freeReminderResponse(fr))
}

// DeleteFreeReminder implements DELETE /free-reminders/{freeReminderId}.
func (h *FreeReminderHandlers) DeleteFreeReminder(w http.ResponseWriter, r *http.Request, freeReminderID uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.svc.Delete(r.Context(), ownerID, freeReminderID); err != nil {
		h.handleFreeReminderError(w, r, err, "free_reminder")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func freeReminderResponse(fr notificationsdomain.FreeReminder) openapi.FreeReminderResponse {
	return openapi.FreeReminderResponse{
		Id:          fr.ID,
		OwnerId:     fr.OwnerID,
		PropertyId:  fr.PropertyID,
		Title:       fr.Title,
		TriggerAt:   fr.TriggerAt,
		Periodicity: openapi.FreeReminderResponsePeriodicity(fr.Periodicity),
		CreatedAt:   fr.CreatedAt,
		UpdatedAt:   fr.UpdatedAt,
	}
}

func upcomingFreeReminderResponse(u notificationsdomain.UpcomingFreeReminder) openapi.UpcomingFreeReminderResponse {
	return openapi.UpcomingFreeReminderResponse{
		FreeReminderId: u.FreeReminderID,
		Title:          u.Title,
		PropertyId:     u.PropertyID,
		TriggerAt:      u.TriggerAt,
		Periodicity:    openapi.UpcomingFreeReminderResponsePeriodicity(u.Periodicity),
	}
}
