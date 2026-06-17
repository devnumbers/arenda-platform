package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	notificationsapp "github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	notificationsdomain "github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// ReminderHandlers implements the generated reminder endpoints.
type ReminderHandlers struct {
	svc        *notificationsapp.ReminderService
	operations *leasesapp.OperationService
	recurring  *leasesapp.RecurringOperationService
	leases     *leasesapp.LeaseService
	logger     *slog.Logger
	clock      clock.Clock
}

// NewReminderHandlers creates HTTP handlers for the reminders API.
func NewReminderHandlers(
	svc *notificationsapp.ReminderService,
	operations *leasesapp.OperationService,
	recurring *leasesapp.RecurringOperationService,
	leases *leasesapp.LeaseService,
	logger *slog.Logger,
	clock clock.Clock,
) *ReminderHandlers {
	return &ReminderHandlers{
		svc:        svc,
		operations: operations,
		recurring:  recurring,
		leases:     leases,
		logger:     logger,
		clock:      clock,
	}
}

func (h *ReminderHandlers) handleReminderError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, notificationsapp.ErrNotFound),
		errors.Is(err, leasesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", err.Error()))
	case errors.Is(err, notificationsapp.ErrInvalidReminderDate):
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", err.Error()))
	case errors.Is(err, notificationsapp.ErrReminderNotPending):
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", err.Error()))
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
		h.handleReminderError(w, r, err)
		return
	}
	if op.PropertyID != propertyId {
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "operation not found"))
		return
	}

	reminder, err := h.svc.CreateForOperation(r.Context(), leasesapp.ToOperationInfo(op), body.ReminderDate.Time)
	if err != nil {
		h.handleReminderError(w, r, err)
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
		h.handleReminderError(w, r, err)
		return
	}
	if op.PropertyID != propertyId {
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "operation not found"))
		return
	}

	reminders, err := h.svc.ListByOwner(r.Context(), ownerID, notificationsapp.ListFilter{Limit: 1000})
	if err != nil {
		h.handleReminderError(w, r, err)
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
		h.handleReminderError(w, r, err)
		return
	}
	if rec.PropertyID != propertyId {
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "recurring operation not found"))
		return
	}

	ops, err := h.recurring.ListOperationsByRecurringOperation(r.Context(), ownerID, recurringOperationId)
	if err != nil {
		h.handleReminderError(w, r, err)
		return
	}

	futureOps := filterFutureOperations(ops, h.clock.Now())
	if len(futureOps) == 0 {
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "no future operations for reminder"))
		return
	}

	earliest := futureOps[0]
	for _, op := range futureOps {
		if op.OperationDate.Before(earliest.OperationDate) {
			earliest = op
		}
	}

	offsetDays := notificationsdomain.ReminderOffset(earliest.OperationDate, body.ReminderDate.Time)
	if offsetDays < 0 {
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "reminder date must be on or before the earliest future operation date"))
		return
	}

	if err := h.recurring.SetReminderOffset(r.Context(), ownerID, recurringOperationId, offsetDays); err != nil {
		h.handleReminderError(w, r, err)
		return
	}

	reminders, err := h.svc.ListByOwner(r.Context(), ownerID, notificationsapp.ListFilter{Limit: 1000})
	if err != nil {
		h.handleReminderError(w, r, err)
		return
	}

	created := filterRemindersByRecurringOperationID(reminders, recurringOperationId)
	if len(created) == 0 {
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), errors.New("no reminders created after setting offset")))
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, created[0])
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
		h.handleReminderError(w, r, err)
		return
	}
	if rec.PropertyID != propertyId {
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "recurring operation not found"))
		return
	}

	reminders, err := h.svc.ListByOwner(r.Context(), ownerID, notificationsapp.ListFilter{Limit: 1000})
	if err != nil {
		h.handleReminderError(w, r, err)
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
		h.handleReminderError(w, r, err)
		return
	}

	reminders, err := h.svc.ListByOwner(r.Context(), ownerID, notificationsapp.ListFilter{Limit: 1000})
	if err != nil {
		h.handleReminderError(w, r, err)
		return
	}

	items := filterRemindersByLeaseID(reminders, leaseId)
	writeJSON(r.Context(), w, http.StatusOK, openapi.RemindersResponse{Items: items})
}

// ListReminders implements GET /reminders.
func (h *ReminderHandlers) ListReminders(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	reminders, err := h.svc.ListByOwner(r.Context(), ownerID, notificationsapp.ListFilter{Limit: 100, Offset: 0})
	if err != nil {
		h.handleReminderError(w, r, err)
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
		h.handleReminderError(w, r, err)
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
		h.handleReminderError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func reminderResponse(r notificationsdomain.Reminder) openapi.ReminderResponse {
	return openapi.ReminderResponse{
		Id:                   r.ID,
		OwnerId:              r.OwnerID,
		TargetType:           openapi.ReminderResponseTargetType(r.TargetType),
		OperationId:          r.OperationID,
		RecurringOperationId: r.RecurringOperationID,
		LeaseId:              r.LeaseID,
		EventType:            openapi.ReminderResponseEventType(r.EventType),
		Status:               openapi.ReminderResponseStatus(r.Status),
		ScheduledAt:          r.ScheduledAt,
		SentAt:               r.SentAt,
		MessageTitle:         r.MessageTitle,
		MessageBody:          r.MessageBody,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
}

func filterRemindersByOperationID(reminders []notificationsdomain.Reminder, operationID uuid.UUID) []openapi.ReminderResponse {
	items := make([]openapi.ReminderResponse, 0)
	for _, r := range reminders {
		if r.OperationID != nil && *r.OperationID == operationID {
			items = append(items, reminderResponse(r))
		}
	}
	return items
}

func filterRemindersByRecurringOperationID(reminders []notificationsdomain.Reminder, recurringOperationID uuid.UUID) []openapi.ReminderResponse {
	items := make([]openapi.ReminderResponse, 0)
	for _, r := range reminders {
		if r.RecurringOperationID != nil && *r.RecurringOperationID == recurringOperationID {
			items = append(items, reminderResponse(r))
		}
	}
	return items
}

func filterRemindersByLeaseID(reminders []notificationsdomain.Reminder, leaseID uuid.UUID) []openapi.ReminderResponse {
	items := make([]openapi.ReminderResponse, 0)
	for _, r := range reminders {
		if r.LeaseID != nil && *r.LeaseID == leaseID {
			items = append(items, reminderResponse(r))
		}
	}
	return items
}

func filterFutureOperations(ops []leasesdomain.Operation, now time.Time) []leasesdomain.Operation {
	today := now.UTC().Truncate(24 * time.Hour)
	out := make([]leasesdomain.Operation, 0, len(ops))
	for _, op := range ops {
		if !op.OperationDate.UTC().Truncate(24*time.Hour).Before(today) {
			out = append(out, op)
		}
	}
	return out
}
