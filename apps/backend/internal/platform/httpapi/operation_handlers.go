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

// OperationHandlers implements the generated operation endpoints.
type OperationHandlers struct {
	svc    *leasesapp.OperationService
	logger *slog.Logger
}

// NewOperationHandlers creates HTTP handlers for the operations API.
func NewOperationHandlers(svc *leasesapp.OperationService, logger *slog.Logger) *OperationHandlers {
	return &OperationHandlers{svc: svc, logger: logger}
}

func (h *OperationHandlers) handleOperationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, leasesapp.ErrInvalidInput):
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	case errors.Is(err, leasesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "operation not found"))
	case errors.Is(err, leasesapp.ErrOperationAlreadyCompleted):
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

// CreateOperation implements POST /properties/{propertyId}/operations.
func (h *OperationHandlers) CreateOperation(w http.ResponseWriter, r *http.Request, propertyId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.OperationCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create operation request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	cmd := leasesapp.CreateOperationCommand{
		PropertyID:    propertyId,
		Type:          string(body.Type),
		Category:      string(body.Category),
		Name:          body.Name,
		AmountKopecks: int64(body.AmountKopecks),
		OperationDate: body.OperationDate.Time,
		LeaseID:       uuidPtrFromOpenAPI(body.LeaseId),
	}
	if body.Comment != nil {
		cmd.Comment = body.Comment
	}

	op, err := h.svc.CreateOperation(r.Context(), ownerID, cmd)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, operationResponse(op))
}

// ListOperationsByProperty implements GET /properties/{propertyId}/operations.
func (h *OperationHandlers) ListOperationsByProperty(w http.ResponseWriter, r *http.Request, propertyId uuid.UUID, params openapi.ListOperationsByPropertyParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var statuses []domain.OperationStatus
	if params.Status != nil {
		statuses = make([]domain.OperationStatus, 0, len(*params.Status))
		for _, s := range *params.Status {
			statuses = append(statuses, domain.OperationStatus(s))
		}
	}

	ops, err := h.svc.ListOperationsByProperty(r.Context(), ownerID, propertyId, statuses)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	items := make([]openapi.OperationResponse, 0, len(ops))
	for _, op := range ops {
		items = append(items, operationResponse(op))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.OperationsResponse{Items: items})
}

// ListOperations implements GET /operations.
func (h *OperationHandlers) ListOperations(w http.ResponseWriter, r *http.Request, params openapi.ListOperationsParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	filter := leasesapp.OperationFilter{}
	if params.Type != nil {
		for _, t := range *params.Type {
			filter.Types = append(filter.Types, domain.OperationType(t))
		}
	}
	if params.Status != nil {
		for _, s := range *params.Status {
			filter.Statuses = append(filter.Statuses, domain.OperationStatus(s))
		}
	}
	if params.Category != nil {
		for _, c := range *params.Category {
			filter.Categories = append(filter.Categories, domain.OperationCategory(c))
		}
	}
	if params.PropertyId != nil {
		filter.PropertyID = *params.PropertyId
	}
	if params.From != nil {
		filter.FromDate = &params.From.Time
	}
	if params.To != nil {
		filter.ToDate = &params.To.Time
	}
	if params.RecurringOperationId != nil {
		filter.RecurringOperationID = *params.RecurringOperationId
	}
	if params.Limit != nil {
		filter.Limit = *params.Limit
	}
	if params.Offset != nil {
		filter.Offset = *params.Offset
	}

	ops, err := h.svc.ListOperations(r.Context(), ownerID, filter)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	items := make([]openapi.OperationResponse, 0, len(ops))
	for _, op := range ops {
		items = append(items, operationResponse(op))
	}
	writeJSON(r.Context(), w, http.StatusOK, openapi.OperationsResponse{Items: items})
}

// GetOperation implements GET /operations/{id}.
func (h *OperationHandlers) GetOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	op, err := h.svc.GetOperation(r.Context(), ownerID, id)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, operationResponse(op))
}

// UpdateOperation implements PATCH /operations/{id}.
func (h *OperationHandlers) UpdateOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.OperationUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update operation request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	cmd := leasesapp.UpdateOperationCommand{
		Type:     ptrString(body.Type),
		Category: ptrString(body.Category),
		Name:     body.Name,
		Comment:  body.Comment,
		LeaseID:  uuidPtrFromOpenAPI(body.LeaseId),
	}
	if body.AmountKopecks != nil {
		cmd.AmountKopecks = new(int64(*body.AmountKopecks))
	}
	if body.OperationDate != nil {
		cmd.OperationDate = new(body.OperationDate.Time)
	}

	op, err := h.svc.UpdateOperation(r.Context(), ownerID, id, cmd)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, operationResponse(op))
}

// DeleteOperation implements DELETE /operations/{id}.
func (h *OperationHandlers) DeleteOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	if err := h.svc.DeleteOperation(r.Context(), ownerID, id); err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CompleteOperation implements POST /operations/{id}/complete.
func (h *OperationHandlers) CompleteOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	op, err := h.svc.CompleteOperation(r.Context(), leasesapp.CompleteOperationCommand{
		OwnerID:     ownerID,
		OperationID: id,
	})
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, operationResponse(op))
}

func operationResponse(op domain.Operation) openapi.OperationResponse {
	resp := openapi.OperationResponse{
		Id:            op.ID,
		OwnerId:       op.OwnerID,
		PropertyId:    op.PropertyID,
		Type:          openapi.OperationType(op.Type),
		Category:      openapi.OperationCategory(op.Category),
		Name:          op.Name,
		AmountKopecks: int(op.AmountKopecks),
		OperationDate: openapi_types.Date{Time: op.OperationDate},
		Status:        openapi.OperationStatus(op.Status),
		IsException:   op.IsException,
		CreatedAt:     op.CreatedAt,
		UpdatedAt:     op.UpdatedAt,
	}
	if op.LeaseID != uuid.Nil {
		resp.LeaseId = &op.LeaseID
	}
	if op.RecurringOperationID != uuid.Nil {
		resp.RecurringOperationId = &op.RecurringOperationID
	}
	if op.Comment != "" {
		resp.Comment = &op.Comment
	}
	return resp
}
