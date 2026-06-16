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

func (h *RecurringOperationHandlers) ownerIDFromContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return uuid.UUID{}, false
	}
	return userID, true
}

func (h *RecurringOperationHandlers) handleRecurringOperationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, leasesapp.ErrInvalidInput):
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", err.Error()))
	case errors.Is(err, leasesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "recurring operation not found"))
	default:
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
	}
}

// CreateRecurringOperation implements POST /properties/{propertyId}/recurring-operations.
func (h *RecurringOperationHandlers) CreateRecurringOperation(w http.ResponseWriter, r *http.Request, propertyId uuid.UUID) {
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	var body openapi.RecurringOperationCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create recurring operation request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	cmd := leasesapp.CreateRecurringOperationCommand{
		PropertyID:    propertyId,
		Type:          string(body.Type),
		Category:      string(body.Category),
		AmountKopecks: int64(body.AmountKopecks),
		StartDate:     body.StartDate.Time,
		PaymentDay:    body.PaymentDay,
	}
	if body.EndDate != nil {
		t := body.EndDate.Time
		cmd.EndDate = &t
	}
	if body.Comment != nil {
		cmd.Comment = body.Comment
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
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
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

// GetRecurringOperation implements GET /recurring-operations/{id}.
func (h *RecurringOperationHandlers) GetRecurringOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	rec, err := h.svc.GetRecurringOperation(r.Context(), ownerID, id)
	if err != nil {
		h.handleRecurringOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, recurringOperationResponse(rec))
}

// UpdateRecurringOperation implements PATCH /recurring-operations/{id}.
func (h *RecurringOperationHandlers) UpdateRecurringOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	var body openapi.RecurringOperationUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update recurring operation request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	cmd := leasesapp.UpdateRecurringOperationCommand{
		Type:     ptrString(body.Type),
		Category: ptrString(body.Category),
		Comment:  body.Comment,
	}
	if body.AmountKopecks != nil {
		v := int64(*body.AmountKopecks)
		cmd.AmountKopecks = &v
	}
	if body.StartDate != nil {
		t := body.StartDate.Time
		cmd.StartDate = &t
	}
	if body.PaymentDay != nil {
		cmd.PaymentDay = body.PaymentDay
	}
	if body.EndDate != nil {
		t := body.EndDate.Time
		cmd.EndDate = &t
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
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
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
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
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
		Category:      openapi.OperationCategory(rec.Category),
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
	return resp
}
