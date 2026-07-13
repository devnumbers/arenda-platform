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

const (
	operationsPaginationDefaultLimit = 100
	operationsPaginationMinLimit     = 1
	operationsPaginationMaxLimit     = 100
	operationsPaginationMinOffset    = 0
	operationsPaginationMaxOffset    = 100000
)

// NewOperationHandlers creates HTTP handlers for the operations API.
func NewOperationHandlers(svc *leasesapp.OperationService, logger *slog.Logger) *OperationHandlers {
	return &OperationHandlers{svc: svc, logger: logger}
}

func (h *OperationHandlers) handleOperationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, leasesapp.ErrInvalidInput), errors.Is(err, domain.ErrInvalidOperationType):
		detail, ok := UserFacingDetail(err)
		if !ok {
			writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
			return
		}
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	case errors.Is(err, leasesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "Операция не найдена"))
	case errors.Is(err, leasesapp.ErrOperationAlreadyCompleted):
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

func normalizeOperationsPagination(limitParam, offsetParam *int) (limit int, offset int, fetchLimit int) {
	limit = operationsPaginationDefaultLimit
	if limitParam != nil {
		limit = *limitParam
	}
	if limit < operationsPaginationMinLimit {
		limit = operationsPaginationMinLimit
	}
	if limit > operationsPaginationMaxLimit {
		limit = operationsPaginationMaxLimit
	}

	if offsetParam != nil {
		offset = *offsetParam
	}
	if offset < operationsPaginationMinOffset {
		offset = operationsPaginationMinOffset
	}
	if offset > operationsPaginationMaxOffset {
		offset = operationsPaginationMaxOffset
	}

	return limit, offset, limit + 1
}

func operationSortFromQuery(sort *openapi.OperationListSort) leasesapp.OperationSort {
	if sort == nil {
		return leasesapp.OperationSortOperationDateDesc
	}
	return leasesapp.NormalizeOperationSort(leasesapp.OperationSort(*sort))
}

// CreateOperation implements POST /properties/{propertyId}/operations.
func (h *OperationHandlers) CreateOperation(w http.ResponseWriter, r *http.Request, propertyId uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.OperationCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create operation request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := leasesapp.CreateOperationCommand{
		PropertyID:    propertyId,
		Type:          string(body.Type),
		CategoryID:    body.CategoryId,
		Name:          body.Name,
		AmountKopecks: int64(body.AmountKopecks),
		OperationDate: body.OperationDate.Time,
		LeaseID:       uuidPtrFromOpenAPI(body.LeaseId),
	}
	if body.Comment != nil {
		cmd.Comment = body.Comment
	}
	if body.ReminderOffsetDays != nil {
		offset := int(*body.ReminderOffsetDays)
		cmd.ReminderOffsetDays = &offset
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	limit, offset, fetchLimit := normalizeOperationsPagination(params.Limit, params.Offset)
	filter := leasesapp.OperationFilter{
		Limit:  fetchLimit,
		Offset: offset,
		Sort:   operationSortFromQuery(params.Sort),
	}
	if params.Status != nil {
		filter.Statuses = make([]domain.OperationStatus, 0, len(*params.Status))
		for _, s := range *params.Status {
			filter.Statuses = append(filter.Statuses, domain.OperationStatus(s))
		}
	}
	if params.Type != nil {
		filter.Types = make([]domain.OperationType, 0, len(*params.Type))
		for _, t := range *params.Type {
			filter.Types = append(filter.Types, domain.OperationType(t))
		}
	}
	if params.CategoryId != nil {
		filter.CategoryIDs = append(filter.CategoryIDs, *params.CategoryId...)
	}
	if params.From != nil {
		filter.FromDate = &params.From.Time
	}
	if params.To != nil {
		filter.ToDate = &params.To.Time
	}

	ops, err := h.svc.ListOperationsByProperty(r.Context(), ownerID, propertyId, filter)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, operationsResponse(ops, limit, offset))
}

// ListOperations implements GET /operations.
func (h *OperationHandlers) ListOperations(w http.ResponseWriter, r *http.Request, params openapi.ListOperationsParams) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	limit, offset, fetchLimit := normalizeOperationsPagination(params.Limit, params.Offset)
	filter := leasesapp.OperationFilter{
		Limit:  fetchLimit,
		Offset: offset,
		Sort:   operationSortFromQuery(params.Sort),
	}
	if params.Type != nil {
		filter.Types = make([]domain.OperationType, 0, len(*params.Type))
		for _, t := range *params.Type {
			filter.Types = append(filter.Types, domain.OperationType(t))
		}
	}
	if params.Status != nil {
		filter.Statuses = make([]domain.OperationStatus, 0, len(*params.Status))
		for _, s := range *params.Status {
			filter.Statuses = append(filter.Statuses, domain.OperationStatus(s))
		}
	}
	if params.CategoryId != nil {
		filter.CategoryIDs = append(filter.CategoryIDs, *params.CategoryId...)
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
	if params.LeaseId != nil {
		filter.LeaseID = *params.LeaseId
	}

	ops, err := h.svc.ListOperations(r.Context(), ownerID, filter)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, operationsResponse(ops, limit, offset))
}

// GetOperation implements GET /operations/{id}.
func (h *OperationHandlers) GetOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.OperationUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update operation request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := leasesapp.UpdateOperationCommand{
		Type:       ptrString(body.Type),
		CategoryID: uuidPtrFromOpenAPI(body.CategoryId),
		Name:       body.Name,
		Comment:    body.Comment,
		LeaseID:    uuidPtrFromOpenAPI(body.LeaseId),
	}
	if body.AmountKopecks != nil {
		cmd.AmountKopecks = new(int64(*body.AmountKopecks))
	}
	if body.OperationDate != nil {
		cmd.OperationDate = new(body.OperationDate.Time)
	}
	if body.ReminderOffsetDays != nil {
		offset := int(*body.ReminderOffsetDays)
		cmd.ReminderOffsetDays = &offset
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
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
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
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

// MarkOperationIncomplete implements POST /operations/{id}/mark-incomplete.
func (h *OperationHandlers) MarkOperationIncomplete(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	op, err := h.svc.MarkOperationIncomplete(r.Context(), leasesapp.MarkOperationIncompleteCommand{
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
		CategoryId:    op.CategoryID,
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
	if op.ReminderOffsetDays != nil {
		offset := openapi.OperationResponseReminderOffsetDays(*op.ReminderOffsetDays)
		resp.ReminderOffsetDays = &offset
	}
	return resp
}

func operationsResponse(ops []domain.Operation, limit int, offset int) openapi.OperationsResponse {
	hasMore := len(ops) > limit
	if hasMore {
		ops = ops[:limit]
	}

	items := make([]openapi.OperationResponse, 0, len(ops))
	for _, op := range ops {
		items = append(items, operationResponse(op))
	}

	resp := openapi.OperationsResponse{
		Items:   items,
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	}
	if hasMore {
		nextOffset := offset + limit
		resp.NextOffset = &nextOffset
	}
	return resp
}
