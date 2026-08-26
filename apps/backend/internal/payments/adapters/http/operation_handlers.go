package http

// The operation endpoints of the second payments contracts slice (ticket
// #461): the shared package doc lives in payment_handlers.go.

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// OperationsManager is the consumer-side port of the operation endpoints
// (ADR 0035): the pay-now use case and the two listings. The concrete
// application service satisfies it; the handler tests run against func-backed
// fakes.
type OperationsManager interface {
	PayOperation(ctx context.Context, actor, propertyID, operationID uuid.UUID) (domain.Operation, error)
	ListPaymentOperations(
		ctx context.Context, actor, propertyID, paymentID uuid.UUID,
		cmd application.ListOperationsCommand,
	) ([]application.OperationListItem, error)
	ListPropertyOperations(
		ctx context.Context, actor, propertyID uuid.UUID,
		cmd application.ListOperationsCommand,
	) ([]application.OperationListItem, error)
}

// OperationsHandlers implements the generated operation endpoints.
type OperationsHandlers struct {
	svc    OperationsManager
	logger *slog.Logger
}

// NewOperationsHandlers creates HTTP handlers for the operations API.
func NewOperationsHandlers(svc OperationsManager, logger *slog.Logger) *OperationsHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &OperationsHandlers{svc: svc, logger: logger}
}

// ListPaymentOperations implements
// GET /properties/{propertyId}/payments/{paymentId}/operations. The overdue
// view status is computed server-side; clients never need the owner's
// timezone.
func (h *OperationsHandlers) ListPaymentOperations(
	w http.ResponseWriter, r *http.Request, propertyID, paymentID openapi_types.UUID,
	params openapi.ListPaymentOperationsParams,
) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	cmd, err := listOperationsFromPaymentParams(params)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}
	items, err := h.svc.ListPaymentOperations(r.Context(), actor, propertyID, paymentID, cmd)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	writeOperations(w, r, items)
}

// ListPropertyOperations implements GET /properties/{propertyId}/operations —
// the property's operations across its rules (the overdue section uses
// status=overdue).
func (h *OperationsHandlers) ListPropertyOperations(
	w http.ResponseWriter, r *http.Request, propertyID openapi_types.UUID,
	params openapi.ListPropertyOperationsParams,
) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	cmd, err := listOperationsFromPropertyParams(params)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}
	items, err := h.svc.ListPropertyOperations(r.Context(), actor, propertyID, cmd)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	writeOperations(w, r, items)
}

// PayOperation implements POST /properties/{propertyId}/operations/{operationId}/pay:
// planned → paid with paid_date = today in the owner's timezone; a repeated
// pay is the contract's 409. The schedule does not shift.
func (h *OperationsHandlers) PayOperation(
	w http.ResponseWriter, r *http.Request, propertyID, operationID openapi_types.UUID,
) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	op, err := h.svc.PayOperation(r.Context(), actor, propertyID, operationID)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	resp := operationResponse(application.OperationListItem{
		Operation: op,
		// The pay result is paid by contract — no recomputation needed.
		ViewStatus: domain.ViewStatusPaid,
	})
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// handleOperationError maps an application error onto the wire contract via
// the shared payments table plus the operation-specific outcomes; anything
// unrecognized is an opaque 500.
func (h *OperationsHandlers) handleOperationError(w http.ResponseWriter, r *http.Request, err error) {
	logger := h.logger
	if writePaymentsError(r.Context(), w, err) {
		return
	}
	logger.ErrorContext(r.Context(), "payments operation use case failed",
		slog.String("error", httpsupport.SanitizeError(err)))
	httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError,
		httpsupport.InternalError(r.Context(), err))
}

// listOperationsFromPaymentParams folds the generated per-rule list params
// into the application command; out-of-vocabulary values are contract 400s.
func listOperationsFromPaymentParams(params openapi.ListPaymentOperationsParams) (application.ListOperationsCommand, error) {
	cmd := application.ListOperationsCommand{Desc: true}
	if params.Status != nil {
		if !params.Status.Valid() {
			return cmd, application.ErrInvalidInput
		}
		cmd.Status = new(domain.OperationViewStatus(*params.Status))
	}
	cmd.DateFrom = datePtrFromWire(params.DateFrom)
	cmd.DateTo = datePtrFromWire(params.DateTo)
	if params.Order != nil {
		if !params.Order.Valid() {
			return cmd, application.ErrInvalidInput
		}
		cmd.Desc = *params.Order != openapi.Asc // Contract default: anything but asc sorts descending.
	}
	if params.Limit != nil {
		cmd.Limit = *params.Limit
	}
	if params.Offset != nil {
		cmd.Offset = *params.Offset
	}
	return cmd, nil
}

// listOperationsFromPropertyParams is the property-scope twin of
// listOperationsFromPaymentParams.
func listOperationsFromPropertyParams(params openapi.ListPropertyOperationsParams) (application.ListOperationsCommand, error) {
	cmd := application.ListOperationsCommand{Desc: true}
	if params.Status != nil {
		if !params.Status.Valid() {
			return cmd, application.ErrInvalidInput
		}
		cmd.Status = new(domain.OperationViewStatus(*params.Status))
	}
	cmd.DateFrom = datePtrFromWire(params.DateFrom)
	cmd.DateTo = datePtrFromWire(params.DateTo)
	if params.Order != nil {
		if !params.Order.Valid() {
			return cmd, application.ErrInvalidInput
		}
		cmd.Desc = *params.Order != openapi.ListPropertyOperationsParamsOrderAsc
	}
	if params.Limit != nil {
		cmd.Limit = *params.Limit
	}
	if params.Offset != nil {
		cmd.Offset = *params.Offset
	}
	return cmd, nil
}

// writeOperations maps the listed items onto the wire response shape.
func writeOperations(w http.ResponseWriter, r *http.Request, items []application.OperationListItem) {
	out := make([]openapi.OperationResponse, 0, len(items))
	for _, item := range items {
		out = append(out, operationResponse(item))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.OperationsResponse{Items: out})
}

// operationResponse maps one listed or just-paid operation onto the wire
// response: status is always the server-computed view status.
func operationResponse(item application.OperationListItem) openapi.OperationResponse {
	op := item.Operation
	var paymentForm *openapi.OperationResponsePaymentForm
	if op.PaymentForm != nil {
		form := openapi.OperationResponsePaymentForm(*op.PaymentForm)
		paymentForm = &form
	}
	return openapi.OperationResponse{
		Id:            op.ID,
		PropertyId:    op.PropertyID,
		PaymentId:     openAPIUUIDPtr(op.PaymentID),
		Date:          openapi_types.Date{Time: op.Date},
		PaidDate:      httpsupport.DatePtrToOpenAPI(op.PaidDate),
		Status:        openapi.OperationResponseStatus(item.ViewStatus),
		Type:          openapi.OperationResponseType(op.Type),
		Title:         op.Title,
		AmountKopecks: op.AmountKopecks,
		PaymentForm:   paymentForm,
		CategoryLabel: op.CategoryLabel,
		CategorySlug:  copyStringPtr(op.CategorySlug),
	}
}

// openAPIUUIDPtr converts an optional domain UUID into the wire pointer form.
func openAPIUUIDPtr(u *uuid.UUID) *openapi_types.UUID {
	if u == nil {
		return nil
	}
	copied := *u
	return &copied
}

// copyStringPtr copies an optional string without aliasing the domain value.
func copyStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}
