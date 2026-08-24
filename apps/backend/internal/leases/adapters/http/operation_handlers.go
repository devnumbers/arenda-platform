package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	propertiesdomain "github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// propertyStatusesByID maps every owner property, including archived ones, to
// its status so operation responses can expose the property status.
func propertyStatusesByID(
	ctx context.Context,
	svc *propertiesapp.PropertyService,
	actor uuid.UUID,
) (map[uuid.UUID]propertiesdomain.PropertyStatus, error) {
	active, err := svc.ListProperties(ctx, actor)
	if err != nil {
		return nil, err
	}
	archived, err := svc.ListArchivedProperties(ctx, actor)
	if err != nil {
		return nil, err
	}
	statuses := make(map[uuid.UUID]propertiesdomain.PropertyStatus, len(active)+len(archived))
	for _, p := range active {
		statuses[p.ID] = p.Status
	}
	for _, p := range archived {
		statuses[p.ID] = p.Status
	}
	return statuses, nil
}

// OperationHandlers implements the generated operation endpoints.
type OperationHandlers struct {
	svc        *leasesapp.OperationService
	categories *leasesapp.CategoryService
	properties *propertiesapp.PropertyService
	logger     *slog.Logger
}

const (
	operationsPaginationDefaultLimit = 100
	operationsPaginationMinLimit     = 1
	operationsPaginationMaxLimit     = 100
	operationsPaginationMinOffset    = 0
	operationsPaginationMaxOffset    = 100000
)

// NewOperationHandlers creates HTTP handlers for the operations API.
func NewOperationHandlers(
	svc *leasesapp.OperationService,
	categories *leasesapp.CategoryService,
	properties *propertiesapp.PropertyService,
	logger *slog.Logger,
) *OperationHandlers {
	return &OperationHandlers{svc: svc, categories: categories, properties: properties, logger: logger}
}

func (h *OperationHandlers) handleOperationError(w http.ResponseWriter, r *http.Request, err error) {
	handleLeaseOperationError(w, r, err, leasesapp.ErrOperationAlreadyCompleted, "Операция не найдена")
}

// handleLeaseOperationError maps the leases domain errors shared by the
// operation and recurring operation endpoints to problem details. The
// conflict error is the endpoint-specific conflict sentinel, and notFoundDetail
// is the 404 detail.
func handleLeaseOperationError(w http.ResponseWriter, r *http.Request, err, conflictErr error, notFoundDetail string) {
	switch {
	case errors.Is(err, leasesapp.ErrInvalidInput), errors.Is(err, domain.ErrInvalidOperationType):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", detail))
	case errors.Is(err, leasesapp.ErrNotFound):
		httpsupport.WriteProblem(r.Context(), w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", notFoundDetail))
	case errors.Is(err, leasesapp.ErrForbidden):
		httpsupport.WriteProblem(r.Context(), w, http.StatusForbidden,
			httpsupport.Problem(r.Context(), "Forbidden", "Недостаточно прав для этого действия"))
	case errors.Is(err, conflictErr), errors.Is(err, leasesapp.ErrArchivedProperty):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", detail))
	default:
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

func normalizeOperationsPagination(limitParam, offsetParam *int) (limit, offset, fetchLimit int) {
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

// ListOperations implements GET /operations.
func (h *OperationHandlers) ListOperations(w http.ResponseWriter, r *http.Request, params openapi.ListOperationsParams) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
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
	if params.ExcludeArchivedProperties != nil {
		filter.ExcludeArchivedProperties = *params.ExcludeArchivedProperties
	}

	ops, err := h.svc.ListOperations(r.Context(), actor, filter)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	h.respondWithOperations(w, r, actor, ops, limit, offset)
}

// GetOperation implements GET /operations/{id}.
func (h *OperationHandlers) GetOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	op, err := h.svc.GetOperation(r.Context(), actor, id)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	h.respondWithOperation(w, r, actor, op)
}

// UpdateOperation implements PATCH /operations/{id}.
func (h *OperationHandlers) UpdateOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.OperationUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update operation request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := leasesapp.UpdateOperationCommand{
		Type:       httpsupport.PtrString(body.Type),
		CategoryID: uuidPtrFromOpenAPI(body.CategoryId),
		Name:       body.Name,
		Comment:    body.Comment,
		LeaseID:    uuidPtrFromOpenAPI(body.LeaseId),
	}
	if body.AmountKopecks != nil {
		cmd.AmountKopecks = new(*body.AmountKopecks)
	}
	if body.OperationDate != nil {
		cmd.OperationDate = new(body.OperationDate.Time)
	}
	if body.ReminderOffsetDays != nil {
		offset := int(*body.ReminderOffsetDays)
		cmd.ReminderOffsetDays = &offset
	}

	op, err := h.svc.UpdateOperation(r.Context(), actor, id, cmd)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	h.respondWithOperation(w, r, actor, op)
}

// DeleteOperation implements DELETE /operations/{id}.
func (h *OperationHandlers) DeleteOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	if err := h.svc.DeleteOperation(r.Context(), actor, id); err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CompleteOperation implements POST /operations/{id}/complete.
func (h *OperationHandlers) CompleteOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	op, err := h.svc.CompleteOperation(r.Context(), leasesapp.CompleteOperationCommand{
		Actor:       actor,
		OperationID: id,
	})
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	h.respondWithOperation(w, r, actor, op)
}

// MarkOperationIncomplete implements POST /operations/{id}/mark-incomplete.
func (h *OperationHandlers) MarkOperationIncomplete(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	op, err := h.svc.MarkOperationIncomplete(r.Context(), leasesapp.MarkOperationIncompleteCommand{
		Actor:       actor,
		OperationID: id,
	})
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	h.respondWithOperation(w, r, actor, op)
}

// MoveOperation implements POST /operations/{id}/move.
func (h *OperationHandlers) MoveOperation(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.OperationMoveRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode move operation request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	op, err := h.svc.MoveOperation(r.Context(), actor, id, leasesapp.MoveOperationCommand{PropertyID: body.PropertyId})
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	h.respondWithOperation(w, r, actor, op)
}

// respondWithOperation enriches op with category names and property statuses
// and writes it; enrichment errors map through the shared operation error
// handler.
func (h *OperationHandlers) respondWithOperation(w http.ResponseWriter, r *http.Request, actor uuid.UUID, op domain.Operation) {
	names, err := categoryNamesByID(r.Context(), h.categories, actor)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	statuses, err := propertyStatusesByID(r.Context(), h.properties, actor)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, operationResponse(op, names, statuses))
}

// respondWithOperations enriches a fetched page of operations and writes the
// operations response envelope with pagination metadata.
func (h *OperationHandlers) respondWithOperations(
	w http.ResponseWriter,
	r *http.Request,
	actor uuid.UUID,
	ops []domain.Operation,
	limit, offset int,
) {
	names, err := categoryNamesByID(r.Context(), h.categories, actor)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	statuses, err := propertyStatusesByID(r.Context(), h.properties, actor)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, operationsResponse(ops, names, statuses, limit, offset))
}

func operationResponse(
	op domain.Operation,
	categoryNames map[uuid.UUID]string,
	propertyStatuses map[uuid.UUID]propertiesdomain.PropertyStatus,
) openapi.OperationResponse {
	resp := openapi.OperationResponse{
		Id:            op.ID,
		OwnerId:       op.OwnerID,
		PropertyId:    domain.PropertyIDPtr(op.PropertyID),
		Type:          openapi.OperationType(op.Type),
		CategoryId:    op.CategoryID,
		CategoryName:  categoryNames[op.CategoryID],
		Name:          op.Name,
		AmountKopecks: op.AmountKopecks,
		OperationDate: openapi_types.Date{Time: op.OperationDate},
		Status:        openapi.OperationStatus(op.Status),
		IsException:   op.IsException,
		CreatedAt:     op.CreatedAt,
		UpdatedAt:     op.UpdatedAt,
	}
	if status, ok := propertyStatuses[op.PropertyID]; ok {
		propertyStatus := openapi.PropertyStatus(status)
		resp.PropertyStatus = &propertyStatus
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

func operationsResponse(
	ops []domain.Operation,
	categoryNames map[uuid.UUID]string,
	propertyStatuses map[uuid.UUID]propertiesdomain.PropertyStatus,
	limit, offset int,
) openapi.OperationsResponse {
	hasMore := len(ops) > limit
	if hasMore {
		ops = ops[:limit]
	}

	items := make([]openapi.OperationResponse, 0, len(ops))
	for _, op := range ops {
		items = append(items, operationResponse(op, categoryNames, propertyStatuses))
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
