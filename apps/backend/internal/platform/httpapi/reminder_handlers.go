package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
)

// ReminderHandlers implements the generated reminder endpoints.
type ReminderHandlers struct {
	svc        *notificationsapp.ReminderService
	calendar   *notificationsapp.CalendarService
	operations *leasesapp.OperationService
	recurring  *leasesapp.RecurringOperationService
	leases     *leasesapp.LeaseService
	logger     *slog.Logger
}

// NewReminderHandlers creates HTTP handlers for the reminders API.
func NewReminderHandlers(
	svc *notificationsapp.ReminderService,
	calendar *notificationsapp.CalendarService,
	operations *leasesapp.OperationService,
	recurring *leasesapp.RecurringOperationService,
	leases *leasesapp.LeaseService,
	logger *slog.Logger,
) *ReminderHandlers {
	return &ReminderHandlers{
		svc:        svc,
		calendar:   calendar,
		operations: operations,
		recurring:  recurring,
		leases:     leases,
		logger:     logger,
	}
}

func (h *ReminderHandlers) handleReminderError(w http.ResponseWriter, r *http.Request, err error, resource string) {
	switch {
	case errors.Is(err, notificationsapp.ErrNotFound),
		errors.Is(err, leasesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", notFoundMessage(resource)))
	case errors.Is(err, notificationsapp.ErrInvalidReminderDate),
		errors.Is(err, notificationsapp.ErrReminderNotPending),
		errors.Is(err, leasesapp.ErrInvalidInput):
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	case errors.Is(err, notificationsapp.ErrConcurrentUpdate):
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "Напоминание изменено одновременно"))
	default:
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
	}
}

// ListLeaseReminders implements GET /leases/{leaseId}/reminders.
func (h *ReminderHandlers) ListLeaseReminders(w http.ResponseWriter, r *http.Request, leaseID uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if _, err := h.leases.GetLease(r.Context(), ownerID, leaseID); err != nil {
		h.handleReminderError(w, r, err, "lease")
		return
	}

	reminders, err := h.svc.ListByLease(r.Context(), ownerID, leaseID, notificationsapp.ListFilter{Limit: 1000})
	if err != nil {
		h.handleReminderError(w, r, err, "lease")
		return
	}

	items := make([]openapi.ReminderResponse, 0, len(reminders))
	for _, rm := range reminders {
		items = append(items, reminderResponse(rm))
	}
	writeJSON(r.Context(), w, http.StatusOK, openapi.RemindersResponse{Items: items})
}

// ListReminders implements GET /reminders.
func (h *ReminderHandlers) ListReminders(w http.ResponseWriter, r *http.Request, params openapi.ListRemindersParams) {
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

	reminders, err := h.svc.ListByOwner(r.Context(), ownerID, notificationsapp.ListFilter{Limit: limit, Offset: offset})
	if err != nil {
		h.handleReminderError(w, r, err, "reminder")
		return
	}

	items := make([]openapi.ReminderResponse, 0, len(reminders))
	for _, rm := range reminders {
		items = append(items, reminderResponse(rm))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.RemindersResponse{Items: items})
}

// ListCalendarReminders implements GET /reminders/calendar.
func (h *ReminderHandlers) ListCalendarReminders(w http.ResponseWriter, r *http.Request, params openapi.ListCalendarRemindersParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if !params.To.After(params.From.Time) {
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Диапазон дат некорректен: 'to' должно быть позже 'from'"))
		return
	}

	items, err := h.calendar.ListCalendar(r.Context(), ownerID, params.From.Time, params.To.Time)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to list calendar reminders", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
		return
	}

	resp := make([]openapi.CalendarReminderItem, len(items))
	for i, item := range items {
		resp[i] = calendarReminderResponse(item)
	}
	writeJSON(r.Context(), w, http.StatusOK, openapi.CalendarRemindersResponse{Items: resp})
}

// UpdateReminder implements PATCH /reminders/{reminderId}.
func (h *ReminderHandlers) UpdateReminder(w http.ResponseWriter, r *http.Request, reminderID uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.ReminderUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update reminder request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	reminder, err := h.svc.Reschedule(r.Context(), ownerID, reminderID, body.ReminderDate.Time)
	if err != nil {
		h.handleReminderError(w, r, err, "reminder")
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, reminderResponse(reminder))
}

// DeleteReminder implements DELETE /reminders/{reminderId}.
func (h *ReminderHandlers) DeleteReminder(w http.ResponseWriter, r *http.Request, reminderID uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.svc.Cancel(r.Context(), ownerID, reminderID); err != nil {
		h.handleReminderError(w, r, err, "reminder")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func notFoundMessage(resource string) string {
	switch resource {
	case "lease":
		return "Аренда не найдена"
	case "reminder":
		return "Напоминание не найдено"
	default:
		return "Не найдено"
	}
}

func reminderResponse(r notificationsdomain.Reminder) openapi.ReminderResponse {
	status := openapi.ReminderResponseStatus(r.Status)
	if r.Status == notificationsdomain.ReminderSending {
		status = openapi.ReminderResponseStatusPending
	}

	return openapi.ReminderResponse{
		Id:                   r.ID,
		OwnerId:              r.OwnerID,
		TargetType:           openapi.ReminderResponseTargetType(r.TargetType),
		OperationId:          r.OperationID,
		RecurringOperationId: r.RecurringOperationID,
		LeaseId:              r.LeaseID,
		PropertyId:           r.PropertyID,
		EventType:            openapi.ReminderResponseEventType(r.EventType),
		Status:               status,
		ScheduledAt:          r.ScheduledAt,
		SentAt:               r.SentAt,
		FailedAttempts:       r.FailedAttempts,
		NextAttemptAt:        r.NextAttemptAt,
		MessageTitle:         r.MessageTitle,
		MessageBody:          r.MessageBody,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
}

func calendarReminderResponse(item notificationsdomain.CalendarReminder) openapi.CalendarReminderItem {
	resp := openapi.CalendarReminderItem{
		Id:             item.ID,
		Type:           openapi.CalendarReminderItemType(item.Type),
		ScheduledAt:    item.ScheduledAt,
		Title:          item.Title,
		PropertyId:     item.PropertyID,
		PropertyName:   item.PropertyName,
		HasProperty:    item.HasProperty,
		OperationId:    item.OperationID,
		LeaseId:        item.LeaseID,
		FreeReminderId: item.FreeReminderID,
	}

	if item.Status != nil {
		s := openapi.CalendarReminderItemStatus(*item.Status)
		resp.Status = &s
	}
	if item.EventType != nil {
		e := openapi.CalendarReminderItemEventType(*item.EventType)
		resp.EventType = &e
	}
	if item.Periodicity != nil {
		p := openapi.CalendarReminderItemPeriodicity(*item.Periodicity)
		resp.Periodicity = &p
	}

	return resp
}
