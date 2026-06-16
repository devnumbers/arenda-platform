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

func (h *OperationHandlers) ownerIDFromContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return uuid.UUID{}, false
	}
	return userID, true
}

func (h *OperationHandlers) handleOperationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, leasesapp.ErrInvalidInput):
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", err.Error()))
	case errors.Is(err, leasesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "operation not found"))
	default:
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
	}
}

// CreateOperation implements POST /properties/{propertyId}/operations.
func (h *OperationHandlers) CreateOperation(w http.ResponseWriter, r *http.Request, propertyId uuid.UUID) {
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	var body openapi.OperationCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create operation request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	cmd := leasesapp.CreateOperationCommand{
		PropertyID:    propertyId,
		Type:          string(body.Type),
		Category:      string(body.Category),
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
func (h *OperationHandlers) ListOperationsByProperty(w http.ResponseWriter, r *http.Request, propertyId uuid.UUID) {
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	ops, err := h.svc.ListOperationsByProperty(r.Context(), ownerID, propertyId)
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
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
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
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	var body openapi.OperationUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update operation request", slog.String("error", err.Error()))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	cmd := leasesapp.UpdateOperationCommand{
		Type:     ptrString(body.Type),
		Category: ptrString(body.Category),
		Comment:  body.Comment,
		LeaseID:  uuidPtrFromOpenAPI(body.LeaseId),
	}
	if body.AmountKopecks != nil {
		v := int64(*body.AmountKopecks)
		cmd.AmountKopecks = &v
	}
	if body.OperationDate != nil {
		t := body.OperationDate.Time
		cmd.OperationDate = &t
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
	ownerID, ok := h.ownerIDFromContext(w, r)
	if !ok {
		return
	}

	if err := h.svc.DeleteOperation(r.Context(), ownerID, id); err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func operationResponse(op domain.Operation) openapi.OperationResponse {
	resp := openapi.OperationResponse{
		Id:                   op.ID,
		OwnerId:              op.OwnerID,
		PropertyId:           op.PropertyID,
		Type:                 openapi.OperationType(op.Type),
		Category:             openapi.OperationCategory(op.Category),
		AmountKopecks:        int(op.AmountKopecks),
		OperationDate:        openapi_types.Date{Time: op.OperationDate},
		IsException:          op.IsException,
		CreatedAt:            op.CreatedAt,
		UpdatedAt:            op.UpdatedAt,
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
