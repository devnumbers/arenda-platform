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
	PayOperation(ctx context.Context, actor, propertyID, operationID uuid.UUID) (application.OperationListItem, error)
	ListPaymentOperations(
		ctx context.Context, actor, propertyID, paymentID uuid.UUID,
		cmd application.OperationsListQuery,
	) ([]application.OperationListItem, error)
	ListPropertyOperations(
		ctx context.Context, actor, propertyID uuid.UUID,
		cmd application.OperationsListQuery,
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
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
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
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
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
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	item, err := h.svc.PayOperation(r.Context(), actor, propertyID, operationID)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, operationResponse(item))
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

// newListOperationsQuery folds the shared query-parameter shape onto the
// listing request. The generated enums differ per endpoint, so callers hand
// over already-decoded primitives; the pagination numbers travel raw and are
// bounded by the service (PrepareOperationsQuery) — one validation home.
func newListOperationsQuery(
	status *domain.OperationViewStatus,
	dateFrom, dateTo *openapi_types.Date,
	asc bool,
	limit, offset *int,
	search *string,
) application.OperationsListQuery {
	query := application.OperationsListQuery{
		Status:   status,
		DateFrom: datePtrFromWire(dateFrom),
		DateTo:   datePtrFromWire(dateTo),
		// Zero value = the contract's descending default; an explicit asc
		// parameter is the only thing that can flip it, and only here.
		Asc: asc,
	}
	if limit != nil {
		query.Limit = *limit
	}
	if offset != nil {
		query.Offset = *offset
	}
	if search != nil {
		query.Search = *search
	}
	return query
}

// foldStatus checks a generated status enum against its own vocabulary and
// lifts it into the domain view-status pointer; a missing filter simply means
// no filter — not an error.
func foldStatus[T ~string](status *T, valid func(T) bool) (*domain.OperationViewStatus, error) {
	var lifted *domain.OperationViewStatus
	if status != nil {
		if !valid(*status) {
			return lifted, application.ErrInvalidInput
		}
		value := domain.OperationViewStatus(*status)
		lifted = &value
	}
	return lifted, nil
}

// foldOrder decodes the sort direction: asc when explicitly asked, desc as
// the contract default; out-of-vocabulary values are contract 400s.
func foldOrder[T ~string](order *T, valid func(T) bool) (bool, error) {
	if order == nil {
		return false, nil
	}
	if !valid(*order) {
		return false, application.ErrInvalidInput
	}
	return *order == "asc", nil
}

// listOperationsFromPaymentParams adapts the per-rule endpoint's params onto
// the shared builder.
func listOperationsFromPaymentParams(
	params openapi.ListPaymentOperationsParams,
) (application.OperationsListQuery, error) {
	status, err := foldStatus(params.Status, openapi.ListPaymentOperationsParamsStatus.Valid)
	if err != nil {
		return application.OperationsListQuery{}, err
	}
	asc, err := foldOrder(params.Order, openapi.ListPaymentOperationsParamsOrder.Valid)
	if err != nil {
		return application.OperationsListQuery{}, err
	}
	// The per-rule endpoint has no search parameter in the contract — the
	// title filter is a property-scope (and payments list) concern.
	return newListOperationsQuery(status, params.DateFrom, params.DateTo, asc, params.Limit, params.Offset, nil), nil
}

// listOperationsFromPropertyParams is the property-scope adapter onto the
// same builder.
func listOperationsFromPropertyParams(
	params openapi.ListPropertyOperationsParams,
) (application.OperationsListQuery, error) {
	status, err := foldStatus(params.Status, openapi.ListPropertyOperationsParamsStatus.Valid)
	if err != nil {
		return application.OperationsListQuery{}, err
	}
	asc, err := foldOrder(params.Order, openapi.ListPropertyOperationsParamsOrder.Valid)
	if err != nil {
		return application.OperationsListQuery{}, err
	}
	return newListOperationsQuery(status, params.DateFrom, params.DateTo, asc, params.Limit, params.Offset, params.Search), nil
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
