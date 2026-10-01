package http

// The operation endpoints of the second payments contracts slice (ticket
// #461): the shared package doc lives in payment_handlers.go.

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// OperationsManager is the consumer-side port of the operation endpoints
// (ADR 0035): the manual creation, the pay-now use case, the two listings,
// the period summary and their global twins. The concrete application
// service satisfies it; the handler tests run against func-backed fakes.
type OperationsManager interface {
	CreateOperation(
		ctx context.Context, actor, propertyID uuid.UUID, cmd application.CreateOperationCommand,
	) (application.OperationListItem, error)
	GetOperation(ctx context.Context, actor, propertyID, operationID uuid.UUID) (application.OperationListItem, error)
	DeleteOperation(ctx context.Context, actor, propertyID, operationID uuid.UUID) error
	PayOperation(ctx context.Context, actor, propertyID, operationID uuid.UUID) (application.OperationListItem, error)
	ListPaymentOperations(
		ctx context.Context, actor, propertyID, paymentID uuid.UUID,
		cmd application.OperationsListQuery,
	) ([]application.OperationListItem, error)
	ListPropertyOperations(
		ctx context.Context, actor, propertyID uuid.UUID,
		cmd application.OperationsListQuery,
	) ([]application.OperationListItem, error)
	SummarizePropertyOperations(
		ctx context.Context, actor, propertyID uuid.UUID,
		cmd application.OperationsSummaryQuery,
	) (application.OperationsSummary, error)
	ListGlobalOperations(
		ctx context.Context, actor uuid.UUID,
		cmd application.GlobalOperationsListQuery,
	) (application.GlobalOperationsPage, error)
	SummarizeGlobalOperations(
		ctx context.Context, actor uuid.UUID,
		cmd application.GlobalOperationsSummaryQuery,
	) (application.OperationsSummary, error)
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

	// The property listing's window is the offset vocabulary — it carries no
	// keyset continuation.
	writeOperations(w, r, items, "")
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

	writeOperations(w, r, items, "")
}

// SummarizePropertyOperations implements GET
// /properties/{propertyId}/operations/summary — the period totals by
// direction plus the per-category breakdown behind the operations screens
// (ticket #473). The overdue view status is computed server-side.
func (h *OperationsHandlers) SummarizePropertyOperations(
	w http.ResponseWriter, r *http.Request, propertyID openapi_types.UUID,
	params openapi.SummarizePropertyOperationsParams,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	cmd, err := summarizeFromParams(params)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}
	summary, err := h.svc.SummarizePropertyOperations(r.Context(), actor, propertyID, cmd)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, operationsSummaryResponse(summary))
}

// ListOperations implements GET /operations — the global «Операции» screen's
// feed (ticket #540): the actor's visible paid operations across properties.
// The propertyIds multi-select is parsed here (a non-uuid entry is the
// contract's 400) and resolved through the view gate in the use case — an
// unknown or non-visible id is the privacy 404.
func (h *OperationsHandlers) ListOperations(w http.ResponseWriter, r *http.Request, params openapi.ListOperationsParams) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	cmd, err := globalOperationsFromListParams(params)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}
	page, err := h.svc.ListGlobalOperations(r.Context(), actor, cmd)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	writeOperationsWithTotal(w, r, page.Items, page.NextCursor, &page.Total)
}

// SummarizeOperations implements GET /operations/summary — the global twin
// of the property summary (ticket #540) over the same visible paid feed.
func (h *OperationsHandlers) SummarizeOperations(w http.ResponseWriter, r *http.Request, params openapi.SummarizeOperationsParams) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	cmd, err := globalOperationsFromSummaryParams(params)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}
	summary, err := h.svc.SummarizeGlobalOperations(r.Context(), actor, cmd)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, operationsSummaryResponse(summary))
}

// CreateOperation implements POST /properties/{propertyId}/operations
// (ticket #569): the manual one-off fact born paid on the owner's today.
// The contract rules — enums, amount bounds, title length, the catalog
// slug — are the application validator's; the transport only folds the body.
func (h *OperationsHandlers) CreateOperation(w http.ResponseWriter, r *http.Request, propertyID openapi_types.UUID) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	var body openapi.CreateOperationRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create operation request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	item, err := h.svc.CreateOperation(r.Context(), actor, propertyID, application.CreateOperationCommand{
		Type:          domain.PaymentType(body.Type),
		Title:         body.Title,
		AmountKopecks: body.AmountKopecks,
		CategorySlug:  body.CategorySlug,
	})
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, operationResponse(item))
}

// GetOperation implements GET /properties/{propertyId}/operations/{operationId}
// — one operation for the operation page. A stranger or a foreign row is the
// privacy 404; the status travels as the server-computed view.
func (h *OperationsHandlers) GetOperation(
	w http.ResponseWriter, r *http.Request, propertyID, operationID openapi_types.UUID,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	item, err := h.svc.GetOperation(r.Context(), actor, propertyID, operationID)
	if err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, operationResponse(item))
}

// DeleteOperation implements DELETE
// /properties/{propertyId}/operations/{operationId}: a planned (the overdue
// debt) or paid operation gets the cancelled tombstone — the fact and the
// debt disappear, the schedule is untouched. A foreign or already-cancelled
// operation is the privacy 404.
func (h *OperationsHandlers) DeleteOperation(
	w http.ResponseWriter, r *http.Request, propertyID, operationID openapi_types.UUID,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	if err := h.svc.DeleteOperation(r.Context(), actor, propertyID, operationID); err != nil {
		h.handleOperationError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
	sort application.OperationSortKey,
	asc bool,
	limit, offset *int,
	search *string,
) application.OperationsListQuery {
	query := application.OperationsListQuery{
		Status:   status,
		DateFrom: datePtrFromWire(dateFrom),
		DateTo:   datePtrFromWire(dateTo),
		// The listing's date key (ticket #992): the planned date unless the
		// request asked for the payment fact.
		Sort: sort,
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

// foldSort decodes the listing's date key (ticket #992): the planned date is
// the contract default; out-of-vocabulary values are contract 400s.
func foldSort[T ~string](sort *T, valid func(T) bool) (application.OperationSortKey, error) {
	if sort == nil {
		return application.SortByDate, nil
	}
	if !valid(*sort) {
		return application.SortByDate, application.ErrInvalidInput
	}
	return application.OperationSortKey(*sort), nil
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
	sort, err := foldSort(params.Sort, openapi.ListPaymentOperationsParamsSort.Valid)
	if err != nil {
		return application.OperationsListQuery{}, err
	}
	asc, err := foldOrder(params.Order, openapi.ListPaymentOperationsParamsOrder.Valid)
	if err != nil {
		return application.OperationsListQuery{}, err
	}
	// The per-rule endpoint has no search parameter in the contract — the
	// title filter is a property-scope (and payments list) concern.
	return newListOperationsQuery(status, params.DateFrom, params.DateTo, sort, asc, params.Limit, params.Offset, nil), nil
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
	sort, err := foldSort(params.Sort, openapi.ListPropertyOperationsParamsSort.Valid)
	if err != nil {
		return application.OperationsListQuery{}, err
	}
	asc, err := foldOrder(params.Order, openapi.ListPropertyOperationsParamsOrder.Valid)
	if err != nil {
		return application.OperationsListQuery{}, err
	}
	typ, err := foldType(params.Type, openapi.ListPropertyOperationsParamsType.Valid)
	if err != nil {
		return application.OperationsListQuery{}, err
	}
	query := newListOperationsQuery(status, params.DateFrom, params.DateTo, sort, asc, params.Limit, params.Offset, params.Search)
	query.Type = typ
	query.Categories = splitCategorySlugs(params.Category)
	return query, nil
}

// summarizeFromParams adapts the summary endpoint's params onto the summary
// request: the same status/period/type vocabulary as the listing, minus the
// pagination — the aggregate runs in SQL. The period reads the listing's
// date key (sort, ticket #992).
func summarizeFromParams(
	params openapi.SummarizePropertyOperationsParams,
) (application.OperationsSummaryQuery, error) {
	status, err := foldStatus(params.Status, openapi.SummarizePropertyOperationsParamsStatus.Valid)
	if err != nil {
		return application.OperationsSummaryQuery{}, err
	}
	sort, err := foldSort(params.Sort, openapi.SummarizePropertyOperationsParamsSort.Valid)
	if err != nil {
		return application.OperationsSummaryQuery{}, err
	}
	typ, err := foldType(params.Type, openapi.SummarizePropertyOperationsParamsType.Valid)
	if err != nil {
		return application.OperationsSummaryQuery{}, err
	}
	return application.OperationsSummaryQuery{
		Status:   status,
		Sort:     sort,
		Type:     typ,
		DateFrom: datePtrFromWire(params.DateFrom),
		DateTo:   datePtrFromWire(params.DateTo),
		Search:   derefString(params.Search),
	}, nil
}

// parsePropertyIDs decodes the propertyIds multi-select: whitespace around
// ids is ignored, empty items are dropped; a non-uuid entry is the contract's
// 400. A missing or empty value is nil — the merged feed.
func parsePropertyIDs(raw *string) ([]uuid.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	var ids []uuid.UUID
	for item := range strings.SplitSeq(*raw, ",") {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, application.ErrInvalidInput
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// globalOperationsFromListParams adapts the global feed's params onto the
// listing request: the property listing's vocabulary minus the status filter
// — paid is the feed's only view status.
func globalOperationsFromListParams(
	params openapi.ListOperationsParams,
) (application.GlobalOperationsListQuery, error) {
	sort, err := foldSort(params.Sort, openapi.ListOperationsParamsSort.Valid)
	if err != nil {
		return application.GlobalOperationsListQuery{}, err
	}
	asc, err := foldOrder(params.Order, openapi.ListOperationsParamsOrder.Valid)
	if err != nil {
		return application.GlobalOperationsListQuery{}, err
	}
	typ, err := foldType(params.Type, openapi.ListOperationsParamsType.Valid)
	if err != nil {
		return application.GlobalOperationsListQuery{}, err
	}
	propertyIDs, err := parsePropertyIDs(params.PropertyIds)
	if err != nil {
		return application.GlobalOperationsListQuery{}, err
	}
	// The global feed's window is the keyset cursor (ticket #597) — the
	// offset vocabulary belongs to the property listings only.
	query := newListOperationsQuery(nil, params.DateFrom, params.DateTo, sort, asc, params.Limit, nil, params.Search)
	return application.GlobalOperationsListQuery{
		PropertyIDs:     propertyIDs,
		DateFrom:        query.DateFrom,
		DateTo:          query.DateTo,
		Limit:           query.Limit,
		Sort:            query.Sort,
		Cursor:          derefString(params.Cursor),
		Search:          query.Search,
		Asc:             query.Asc,
		Type:            typ,
		Categories:      splitCategorySlugs(params.Category),
		IncludeArchived: derefBool(params.IncludeArchived),
	}, nil
}

// globalOperationsFromSummaryParams adapts the global summary's params: the
// property summary's vocabulary plus the propertyIds multi-select and the
// category filter (the breakdown-only narrowing lives in the store). The
// period reads the feed's date key (sort, ticket #992).
func globalOperationsFromSummaryParams(
	params openapi.SummarizeOperationsParams,
) (application.GlobalOperationsSummaryQuery, error) {
	sort, err := foldSort(params.Sort, openapi.SummarizeOperationsParamsSort.Valid)
	if err != nil {
		return application.GlobalOperationsSummaryQuery{}, err
	}
	typ, err := foldType(params.Type, openapi.SummarizeOperationsParamsType.Valid)
	if err != nil {
		return application.GlobalOperationsSummaryQuery{}, err
	}
	propertyIDs, err := parsePropertyIDs(params.PropertyIds)
	if err != nil {
		return application.GlobalOperationsSummaryQuery{}, err
	}
	return application.GlobalOperationsSummaryQuery{
		PropertyIDs:     propertyIDs,
		Sort:            sort,
		DateFrom:        datePtrFromWire(params.DateFrom),
		DateTo:          datePtrFromWire(params.DateTo),
		Search:          derefString(params.Search),
		Type:            typ,
		Categories:      splitCategorySlugs(params.Category),
		IncludeArchived: derefBool(params.IncludeArchived),
	}, nil
}

// derefBool lifts an optional boolean parameter onto its value form; a
// missing parameter is the false no-opt-in value.
func derefBool(b *bool) bool {
	return b != nil && *b
}

// derefString lifts an optional string parameter onto its value form; a
// missing parameter is the empty no-filter value.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// foldType decodes the direction filter onto the domain payment type; a
// missing filter simply means no filter — not an error.
func foldType[T ~string](typ *T, valid func(T) bool) (*domain.PaymentType, error) {
	var lifted *domain.PaymentType
	if typ != nil {
		if !valid(*typ) {
			return lifted, application.ErrInvalidInput
		}
		value := domain.PaymentType(*typ)
		lifted = &value
	}
	return lifted, nil
}

// splitCategorySlugs decodes the comma-separated category slugs parameter:
// whitespace around slugs is ignored, empty items are dropped, and an empty
// or missing value means no filter (nil).
func splitCategorySlugs(raw *string) []string {
	if raw == nil {
		return nil
	}
	var slugs []string
	for item := range strings.SplitSeq(*raw, ",") {
		if slug := strings.TrimSpace(item); slug != "" {
			slugs = append(slugs, slug)
		}
	}
	return slugs
}

// operationsSummaryResponse maps the summary onto the wire response shape.
func operationsSummaryResponse(summary application.OperationsSummary) openapi.OperationsSummaryResponse {
	categories := make([]openapi.OperationsSummaryCategory, 0, len(summary.Categories))
	for _, category := range summary.Categories {
		categories = append(categories, openapi.OperationsSummaryCategory{
			CategorySlug:  category.Slug,
			CategoryLabel: category.Label,
			Type:          openapi.OperationsSummaryCategoryType(category.Type),
			TotalKopecks:  category.TotalKopecks,
		})
	}
	return openapi.OperationsSummaryResponse{
		IncomeTotalKopecks:  summary.IncomeTotalKopecks,
		ExpenseTotalKopecks: summary.ExpenseTotalKopecks,
		Categories:          categories,
	}
}

// writeOperations maps the listed items onto the wire response shape. The
// keyset continuation (ticket #597) travels on the global feed's pages — the
// offset-based property listings pass ” and the wire's nextCursor is null.
func writeOperations(w http.ResponseWriter, r *http.Request, items []application.OperationListItem, nextCursor string) {
	writeOperationsWithTotal(w, r, items, nextCursor, nil)
}

// writeOperationsWithTotal is writeOperations with the scope's match count
// (ticket #599): the global feed's «найдено N» — the property-scoped
// listings sharing the schema pass nil and the wire omits the field.
func writeOperationsWithTotal(
	w http.ResponseWriter, r *http.Request, items []application.OperationListItem, nextCursor string, total *int64,
) {
	out := make([]openapi.OperationResponse, 0, len(items))
	for _, item := range items {
		out = append(out, operationResponse(item))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.OperationsResponse{
		Items:      out,
		NextCursor: httpsupport.StringPtr(nextCursor),
		Total:      total,
	})
}

// operationResponse maps one listed or just-paid operation onto the wire
// response: status is always the server-computed view status; propertyName
// travels only where the listing carried it (the global feed's row label).
func operationResponse(item application.OperationListItem) openapi.OperationResponse {
	op := item.Operation
	var propertyName *string
	if item.PropertyName != "" {
		propertyName = &item.PropertyName
	}
	return openapi.OperationResponse{
		Id:            op.ID,
		PropertyId:    op.PropertyID,
		PropertyName:  propertyName,
		PaymentId:     openAPIUUIDPtr(op.PaymentID),
		Date:          openapi_types.Date{Time: op.Date},
		PaidDate:      httpsupport.DatePtrToOpenAPI(op.PaidDate),
		Status:        openapi.OperationResponseStatus(item.ViewStatus),
		Type:          openapi.OperationResponseType(op.Type),
		Title:         op.Title,
		AmountKopecks: op.AmountKopecks,
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
