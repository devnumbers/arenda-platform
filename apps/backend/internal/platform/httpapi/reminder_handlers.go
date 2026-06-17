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
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", resource+" not found"))
	case errors.Is(err, notificationsapp.ErrInvalidReminderDate):
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", err.Error()))
	case errors.Is(err, notificationsapp.ErrReminderNotPending):
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", err.Error()))
	case errors.Is(err, notificationsapp.ErrConcurrentUpdate):
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "reminder changed concurrently"))
	default:
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
	}
}

// CreateOperationReminder implements POST /properties/{propertyId}/operations/{operationId}/reminders.
func (h *ReminderHandlers) CreateOperationReminder(w http.ResponseWriter, r *http.Request, propertyId, operationId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.ReminderCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create operation reminder request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	op, err := h.operations.GetOperation(r.Context(), ownerID, operationId)
	if err != nil {
		h.handleReminderError(w, r, err, "operation")
		return
	}
	if op.PropertyID != propertyId {
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "operation not found"))
		return
	}

	reminder, err := h.svc.CreateForOperation(r.Context(), leasesapp.ToOperationInfo(op), body.ReminderDate.Time)
	if err != nil {
		h.handleReminderError(w, r, err, "operation")
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, reminderResponse(reminder))
}

// ListOperationReminders implements GET /properties/{propertyId}/operations/{operationId}/reminders.
func (h *ReminderHandlers) ListOperationReminders(w http.ResponseWriter, r *http.Request, propertyId, operationId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	op, err := h.operations.GetOperation(r.Context(), ownerID, operationId)
	if err != nil {
		h.handleReminderError(w, r, err, "operation")
		return
	}
	if op.PropertyID != propertyId {
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "operation not found"))
		return
	}

	reminders, err := h.svc.ListByOwner(r.Context(), ownerID, notificationsapp.ListFilter{Limit: 1000})
	if err != nil {
		h.handleReminderError(w, r, err, "operation")
		return
	}

	items := filterRemindersByOperationID(reminders, operationId)
	writeJSON(r.Context(), w, http.StatusOK, openapi.RemindersResponse{Items: items})
}

// CreateRecurringOperationReminder implements POST /properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders.
func (h *ReminderHandlers) CreateRecurringOperationReminder(w http.ResponseWriter, r *http.Request, propertyId, recurringOperationId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.ReminderCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create recurring operation reminder request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	rec, err := h.recurring.GetRecurringOperation(r.Context(), ownerID, recurringOperationId)
	if err != nil {
		h.handleReminderError(w, r, err, "recurring operation")
		return
	}
	if rec.PropertyID != propertyId {
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "recurring operation not found"))
		return
	}

	created, err := h.recurring.CreateReminder(r.Context(), ownerID, recurringOperationId, body.ReminderDate.Time)
	if err != nil {
		h.handleReminderError(w, r, err, "recurring operation")
		return
	}

	items := make([]openapi.ReminderResponse, 0, len(created))
	for _, rm := range created {
		items = append(items, reminderResponse(rm))
	}
	writeJSON(r.Context(), w, http.StatusCreated, openapi.RemindersResponse{Items: items})
}

// ListRecurringOperationReminders implements GET /properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders.
func (h *ReminderHandlers) ListRecurringOperationReminders(w http.ResponseWriter, r *http.Request, propertyId, recurringOperationId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	rec, err := h.recurring.GetRecurringOperation(r.Context(), ownerID, recurringOperationId)
	if err != nil {
		h.handleReminderError(w, r, err, "recurring operation")
		return
	}
	if rec.PropertyID != propertyId {
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "recurring operation not found"))
		return
	}

	reminders, err := h.svc.ListByOwner(r.Context(), ownerID, notificationsapp.ListFilter{Limit: 1000})
	if err != nil {
		h.handleReminderError(w, r, err, "recurring operation")
		return
	}

	items := filterRemindersByRecurringOperationID(reminders, recurringOperationId)
	writeJSON(r.Context(), w, http.StatusOK, openapi.RemindersResponse{Items: items})
}

// ListLeaseReminders implements GET /leases/{leaseId}/reminders.
func (h *ReminderHandlers) ListLeaseReminders(w http.ResponseWriter, r *http.Request, leaseId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	if _, err := h.leases.GetLease(r.Context(), ownerID, leaseId); err != nil {
		h.handleReminderError(w, r, err, "lease")
		return
	}

	reminders, err := h.svc.ListByOwner(r.Context(), ownerID, notificationsapp.ListFilter{Limit: 1000})
	if err != nil {
		h.handleReminderError(w, r, err, "lease")
		return
	}

	items := filterRemindersByLeaseID(reminders, leaseId)
	writeJSON(r.Context(), w, http.StatusOK, openapi.RemindersResponse{Items: items})
}

// ListReminders implements GET /reminders.
func (h *ReminderHandlers) ListReminders(w http.ResponseWriter, r *http.Request, params openapi.ListRemindersParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	limit := 100
	if params.Limit != nil {
		limit = *params.Limit
		if limit > 1000 {
			limit = 1000
		}
		if limit < 1 {
			limit = 1
		}
	}

	offset := 0
	if params.Offset != nil {
		offset = *params.Offset
		if offset < 0 {
			offset = 0
		}
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.ReminderUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update reminder request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	if err := h.svc.Cancel(r.Context(), ownerID, reminderId); err != nil {
		h.handleReminderError(w, r, err, "reminder")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func reminderResponse(r notificationsdomain.Reminder) openapi.ReminderResponse {
	status := openapi.ReminderResponseStatus(r.Status)
	if r.Status == notificationsdomain.ReminderSending {
		status = openapi.Pending
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

func filterRemindersByOperationID(reminders []notificationsdomain.Reminder, operationID uuid.UUID) []openapi.ReminderResponse {
	items := make([]openapi.ReminderResponse, 0, len(reminders))
	for _, r := range reminders {
		if r.OperationID != nil && *r.OperationID == operationID {
			items = append(items, reminderResponse(r))
		}
	}
	return items
}

func filterRemindersByRecurringOperationID(reminders []notificationsdomain.Reminder, recurringOperationID uuid.UUID) []openapi.ReminderResponse {
	items := make([]openapi.ReminderResponse, 0, len(reminders))
	for _, r := range reminders {
		if r.RecurringOperationID != nil && *r.RecurringOperationID == recurringOperationID {
			items = append(items, reminderResponse(r))
		}
	}
	return items
}

func filterRemindersByLeaseID(reminders []notificationsdomain.Reminder, leaseID uuid.UUID) []openapi.ReminderResponse {
	items := make([]openapi.ReminderResponse, 0, len(reminders))
	for _, r := range reminders {
		if r.LeaseID != nil && *r.LeaseID == leaseID {
			items = append(items, reminderResponse(r))
		}
	}
	return items
}
