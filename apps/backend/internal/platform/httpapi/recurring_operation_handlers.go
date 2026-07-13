package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// RecurringOperationHandlers implements the generated recurring operation endpoints.
type RecurringOperationHandlers struct {
	svc    *leasesapp.RecurringOperationService
	logger *slog.Logger
}

// NewRecurringOperationHandlers creates HTTP handlers for the recurring operations API.
func NewRecurringOperationHandlers(svc *leasesapp.RecurringOperationService, logger *slog.Logger) *RecurringOperationHandlers {
	return &RecurringOperationHandlers{svc: svc, logger: logger}
}

func (h *RecurringOperationHandlers) handleRecurringOperationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, leasesapp.ErrInvalidInput), errors.Is(err, domain.ErrInvalidOperationType):
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	case errors.Is(err, leasesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "Серийная операция не найдена"))
	case errors.Is(err, leasesapp.ErrRecurringOperationLeaseCreated):
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", detail))
	case errors.Is(err, leasesapp.ErrArchivedProperty):
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", detail))
	default:
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
	}
}

// CreateRecurringOperation implements POST /properties/{propertyId}/recurring-operations.
func (h *RecurringOperationHandlers) CreateRecurringOperation(w http.ResponseWriter, r *http.Request, propertyId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.RecurringOperationCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create recurring operation request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := leasesapp.CreateRecurringOperationCommand{
		PropertyID:    propertyId,
		Type:          string(body.Type),
		CategoryID:    body.CategoryId,
		Name:          body.Name,
		AmountKopecks: int64(body.AmountKopecks),
		StartDate:     body.StartDate.Time,
	}
	if body.PaymentDay != nil {
		cmd.PaymentDay = *body.PaymentDay
	}
	if body.EndDate != nil {
		cmd.EndDate = new(body.EndDate.Time)
	}
	if body.Comment != nil {
		cmd.Comment = body.Comment
	}
	if body.Periodicity != nil {
		cmd.Periodicity = string(*body.Periodicity)
	}
	if body.ReminderOffsetDays != nil {
		offset := int(*body.ReminderOffsetDays)
		cmd.ReminderOffsetDays = &offset
	}

	rec, err := h.svc.CreateRecurringOperation(r.Context(), ownerID, cmd)
	if err != nil {
		h.handleRecurringOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, recurringOperationResponse(rec))
}

// ListRecurringOperationsByProperty implements GET /properties/{propertyId}/recurring-operations.
func (h *RecurringOperationHandlers) ListRecurringOperationsByProperty(w http.ResponseWriter, r *http.Request, propertyId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	recs, err := h.svc.ListRecurringOperationsByProperty(r.Context(), ownerID, propertyId)
	if err != nil {
		h.handleRecurringOperationError(w, r, err)
		return
	}

	items := make([]openapi.RecurringOperationResponse, 0, len(recs))
	for _, rec := range recs {
		items = append(items, recurringOperationResponse(rec))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.RecurringOperationsResponse{Items: items})
}

// ListRecurringOperations implements GET /recurring-operations.
func (h *RecurringOperationHandlers) ListRecurringOperations(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	recs, err := h.svc.ListRecurringOperations(r.Context(), ownerID)
	if err != nil {
		h.handleRecurringOperationError(w, r, err)
		return
	}

	items := make([]openapi.RecurringOperationResponse, 0, len(recs))
	for _, rec := range recs {
		items = append(items, recurringOperationResponse(rec))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.RecurringOperationsResponse{Items: items})
}

// GetRecurringOperation implements GET /recurring-operations/{id}.
func (h *RecurringOperationHandlers) GetRecurringOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	rec, err := h.svc.GetRecurringOperation(r.Context(), ownerID, id)
	if err != nil {
		h.handleRecurringOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, recurringOperationResponse(rec))
}

// DeleteRecurringOperation implements DELETE /recurring-operations/{id}.
func (h *RecurringOperationHandlers) DeleteRecurringOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.svc.DeleteRecurringOperation(r.Context(), ownerID, id); err != nil {
		h.handleRecurringOperationError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateRecurringOperation implements PATCH /recurring-operations/{id}.
func (h *RecurringOperationHandlers) UpdateRecurringOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.RecurringOperationUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update recurring operation request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := leasesapp.UpdateRecurringOperationCommand{
		Type:       ptrString(body.Type),
		CategoryID: uuidPtrFromOpenAPI(body.CategoryId),
		Name:       body.Name,
		Comment:    body.Comment,
	}
	if body.AmountKopecks != nil {
		cmd.AmountKopecks = new(int64(*body.AmountKopecks))
	}
	if body.StartDate != nil {
		cmd.StartDate = new(body.StartDate.Time)
	}
	if body.PaymentDay != nil {
		cmd.PaymentDay = body.PaymentDay
	}
	if body.EndDate != nil {
		cmd.EndDate = new(body.EndDate.Time)
	}
	if body.Periodicity != nil {
		p := string(*body.Periodicity)
		cmd.Periodicity = &p
	}
	if body.ApplyFromDate != nil {
		cmd.ApplyFromDate = new(body.ApplyFromDate.Time)
	}
	if body.ReminderOffsetDays != nil {
		offset := int(*body.ReminderOffsetDays)
		cmd.ReminderOffsetDays = &offset
	}

	rec, err := h.svc.UpdateRecurringOperation(r.Context(), ownerID, id, cmd)
	if err != nil {
		h.handleRecurringOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, recurringOperationResponse(rec))
}

// PauseRecurringOperation implements POST /recurring-operations/{id}/pause.
func (h *RecurringOperationHandlers) PauseRecurringOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	rec, err := h.svc.PauseRecurringOperation(r.Context(), ownerID, id)
	if err != nil {
		h.handleRecurringOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, recurringOperationResponse(rec))
}

// ResumeRecurringOperation implements POST /recurring-operations/{id}/resume.
func (h *RecurringOperationHandlers) ResumeRecurringOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	rec, err := h.svc.ResumeRecurringOperation(r.Context(), ownerID, id)
	if err != nil {
		h.handleRecurringOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, recurringOperationResponse(rec))
}

func recurringOperationResponse(rec domain.RecurringOperation) openapi.RecurringOperationResponse {
	resp := openapi.RecurringOperationResponse{
		Id:            rec.ID,
		OwnerId:       rec.OwnerID,
		PropertyId:    rec.PropertyID,
		Type:          openapi.OperationType(rec.Type),
		CategoryId:    rec.CategoryID,
		Name:          rec.Name,
		AmountKopecks: int(rec.AmountKopecks),
		StartDate:     openapi_types.Date{Time: rec.StartDate},
		PaymentDay:    rec.PaymentDay,
		Periodicity:   openapi.RecurringOperationResponsePeriodicity(rec.Periodicity),
		Status:        openapi.RecurringOperationResponseStatus(rec.Status),
		CreatedAt:     rec.CreatedAt,
		UpdatedAt:     rec.UpdatedAt,
	}
	if rec.LeaseID != uuid.Nil {
		resp.LeaseId = &rec.LeaseID
	}
	if rec.EndDate != nil {
		resp.EndDate = &openapi_types.Date{Time: *rec.EndDate}
	}
	if rec.Comment != "" {
		resp.Comment = &rec.Comment
	}
	if rec.ReminderOffsetDays != nil {
		offset := openapi.RecurringOperationResponseReminderOffsetDays(*rec.ReminderOffsetDays)
		resp.ReminderOffsetDays = &offset
	}
	return resp
}
