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
	operations *leasesapp.OperationService
	recurring  *leasesapp.RecurringOperationService
	leases     *leasesapp.LeaseService
	logger     *slog.Logger
}

// NewReminderHandlers creates HTTP handlers for the reminders API.
func NewReminderHandlers(
	svc *notificationsapp.ReminderService,
	operations *leasesapp.OperationService,
	recurring *leasesapp.RecurringOperationService,
	leases *leasesapp.LeaseService,
	logger *slog.Logger,
) *ReminderHandlers {
	return &ReminderHandlers{
		svc:        svc,
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
func (h *ReminderHandlers) ListLeaseReminders(w http.ResponseWriter, r *http.Request, leaseId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if _, err := h.leases.GetLease(r.Context(), ownerID, leaseId); err != nil {
		h.handleReminderError(w, r, err, "lease")
		return
	}

	reminders, err := h.svc.ListByLease(r.Context(), ownerID, leaseId, notificationsapp.ListFilter{Limit: 1000})
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

// UpdateReminder implements PATCH /reminders/{reminderId}.
func (h *ReminderHandlers) UpdateReminder(w http.ResponseWriter, r *http.Request, reminderId uuid.UUID) {
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

	reminder, err := h.svc.Reschedule(r.Context(), ownerID, reminderId, body.ReminderDate.Time)
	if err != nil {
		h.handleReminderError(w, r, err, "reminder")
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, reminderResponse(reminder))
}

// DeleteReminder implements DELETE /reminders/{reminderId}.
func (h *ReminderHandlers) DeleteReminder(w http.ResponseWriter, r *http.Request, reminderId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.svc.Cancel(r.Context(), ownerID, reminderId); err != nil {
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
