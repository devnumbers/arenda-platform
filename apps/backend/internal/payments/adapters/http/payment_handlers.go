// Package http holds the payments HTTP adapters (ADR 0049 §4): the payment
// rule endpoints of the first contracts slice (#457) — create, list, get,
// partial update, delete with keep_overdue, pause and resume — plus the
// operations listings, «Оплатить сейчас» and PUT favorite of the second
// slice (#461).
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// PaymentManager is the consumer-side port of these handlers (ADR 0035): the
// payment rule use cases. The concrete application service satisfies it; the
// handler tests run against func-backed fakes.
//
//nolint:dupl // the test double's fields mirror this method set — that is what implementing the port is
type PaymentManager interface {
	CreatePayment(ctx context.Context, actor, propertyID uuid.UUID, cmd application.CreatePaymentCommand) (domain.Payment, error)
	ListPayments(ctx context.Context, actor, propertyID uuid.UUID) ([]domain.Payment, error)
	GetPayment(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (domain.Payment, error)
	UpdatePayment(ctx context.Context, actor, propertyID, paymentID uuid.UUID, cmd application.UpdatePaymentCommand) (domain.Payment, error)
	DeletePayment(ctx context.Context, actor, propertyID, paymentID uuid.UUID, keepOverdue bool) error
	PausePayment(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (domain.Payment, error)
	ResumePayment(ctx context.Context, actor, propertyID, paymentID uuid.UUID) (domain.Payment, error)
	SetPaymentFavorite(ctx context.Context, actor, propertyID, paymentID uuid.UUID, favorite bool) (domain.Payment, error)
}

// PaymentHandlers implements the generated payment endpoints.
type PaymentHandlers struct {
	svc    PaymentManager
	logger *slog.Logger
}

// NewPaymentHandlers creates HTTP handlers for the payments API.
func NewPaymentHandlers(svc PaymentManager, logger *slog.Logger) *PaymentHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &PaymentHandlers{svc: svc, logger: logger}
}

// staticPaymentProblems maps the payments application errors with a fixed
// outcome onto their wire status, title and detail (the shared ErrorProblem
// table shape). Invalid input is handled dynamically through the shared
// user-facing table like in every context.
var staticPaymentProblems = []httpsupport.ErrorProblem{
	{
		Err: application.ErrNotFound, Status: http.StatusNotFound,
		Title: httpsupport.ProblemTitleNotFound, Detail: "Не найдено",
	},
	{
		Err: application.ErrForbidden, Status: http.StatusForbidden,
		Title: httpsupport.ProblemTitleForbidden, Detail: "Недостаточно прав для этого действия",
	},
	{
		Err: application.ErrArchivedProperty, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "Нельзя изменить архивный объект",
	},
	{
		Err: application.ErrAlreadyPaused, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "Платёж уже на паузе",
	},
	{
		Err: application.ErrNotPaused, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "Платёж не на паузе",
	},
	{
		Err: application.ErrAlreadyPaid, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "Операция уже оплачена",
	},
}

// writePaymentsError maps an error with a fixed outcome onto the wire via the
// payments problem table, then handles invalid input through its user-facing
// detail. The bool verdict lets each handler turn anything unrecognized into
// its own opaque 500.
func writePaymentsError(ctx context.Context, w http.ResponseWriter, err error) bool {
	if httpsupport.WriteErrorProblem(ctx, w, err, staticPaymentProblems) {
		return true
	}
	if errors.Is(err, application.ErrInvalidInput) {
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			return false
		}
		httpsupport.WriteProblem(ctx, w, http.StatusBadRequest, httpsupport.Problem(ctx, "Bad request", detail))
		return true
	}
	return false
}

// handlePaymentError maps an application error onto the wire contract: the
// fixed table first, invalid input through its user-facing detail, anything
// else an opaque 500.
func (h *PaymentHandlers) handlePaymentError(w http.ResponseWriter, r *http.Request, err error) {
	if writePaymentsError(r.Context(), w, err) {
		return
	}
	h.writeInternal(w, r, err)
}

// CreatePayment implements POST /properties/{propertyId}/payments.
func (h *PaymentHandlers) CreatePayment(w http.ResponseWriter, r *http.Request, propertyID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PaymentCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create payment request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd, err := createCommand(body)
	if err != nil {
		h.handlePaymentError(w, r, err)
		return
	}

	payment, err := h.svc.CreatePayment(r.Context(), actor, propertyID, cmd)
	if err != nil {
		h.handlePaymentError(w, r, err)
		return
	}

	h.respondWithPayment(w, r, payment, http.StatusCreated)
}

// ListPayments implements GET /properties/{propertyId}/payments.
func (h *PaymentHandlers) ListPayments(w http.ResponseWriter, r *http.Request, propertyID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	payments, err := h.svc.ListPayments(r.Context(), actor, propertyID)
	if err != nil {
		h.handlePaymentError(w, r, err)
		return
	}

	items := make([]openapi.PaymentResponse, 0, len(payments))
	for _, payment := range payments {
		resp, err := paymentResponse(payment)
		if err != nil {
			h.writeInternal(w, r, err)
			return
		}
		items = append(items, resp)
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.PaymentsResponse{Items: items})
}

// GetPayment implements GET /properties/{propertyId}/payments/{paymentId}.
func (h *PaymentHandlers) GetPayment(w http.ResponseWriter, r *http.Request, propertyID, paymentID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	payment, err := h.svc.GetPayment(r.Context(), actor, propertyID, paymentID)
	if err != nil {
		h.handlePaymentError(w, r, err)
		return
	}

	h.respondWithPayment(w, r, payment, http.StatusOK)
}

// UpdatePayment implements PATCH /properties/{propertyId}/payments/{paymentId}.
func (h *PaymentHandlers) UpdatePayment(w http.ResponseWriter, r *http.Request, propertyID, paymentID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.PaymentUpdateRequest
	endDateRaw, err := h.decodeUpdateBody(w, r, &body)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update payment request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd, err := updateCommand(body, endDateRaw)
	if err != nil {
		h.handlePaymentError(w, r, err)
		return
	}

	payment, err := h.svc.UpdatePayment(r.Context(), actor, propertyID, paymentID, cmd)
	if err != nil {
		h.handlePaymentError(w, r, err)
		return
	}

	h.respondWithPayment(w, r, payment, http.StatusOK)
}

// DeletePayment implements DELETE /properties/{propertyId}/payments/{paymentId}.
func (h *PaymentHandlers) DeletePayment(
	w http.ResponseWriter, r *http.Request, propertyID, paymentID openapi_types.UUID, params openapi.DeletePaymentParams,
) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	// The wire default is true: the overdue debt survives the rule unless its
	// deletion is the explicit choice (ADR 0049 §4).
	keepOverdue := true
	if params.KeepOverdue != nil {
		keepOverdue = *params.KeepOverdue
	}

	if err := h.svc.DeletePayment(r.Context(), actor, propertyID, paymentID, keepOverdue); err != nil {
		h.handlePaymentError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PausePayment implements POST /properties/{propertyId}/payments/{paymentId}/pause.
func (h *PaymentHandlers) PausePayment(w http.ResponseWriter, r *http.Request, propertyID, paymentID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	payment, err := h.svc.PausePayment(r.Context(), actor, propertyID, paymentID)
	if err != nil {
		h.handlePaymentError(w, r, err)
		return
	}

	h.respondWithPayment(w, r, payment, http.StatusOK)
}

// ResumePayment implements POST /properties/{propertyId}/payments/{paymentId}/resume.
func (h *PaymentHandlers) ResumePayment(w http.ResponseWriter, r *http.Request, propertyID, paymentID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	payment, err := h.svc.ResumePayment(r.Context(), actor, propertyID, paymentID)
	if err != nil {
		h.handlePaymentError(w, r, err)
		return
	}

	h.respondWithPayment(w, r, payment, http.StatusOK)
}

// SetPaymentFavorite implements PUT /properties/{propertyId}/payments/{paymentId}/favorite
// (ticket #461): the atomic favorite star write; the updated rule travels back.
func (h *PaymentHandlers) SetPaymentFavorite(w http.ResponseWriter, r *http.Request, propertyID, paymentID openapi_types.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
			httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	body, err := decodeFavoriteBody(w, r)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode set favorite request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	payment, err := h.svc.SetPaymentFavorite(r.Context(), actor, propertyID, paymentID, body.Favorite)
	if err != nil {
		h.handlePaymentError(w, r, err)
		return
	}

	h.respondWithPayment(w, r, payment, http.StatusOK)
}

// decodeFavoriteBody reads the favorite toggle body strictly. A required bool
// cannot express its own absence in the generated struct ({} and {"favorite":
// false} both decode to false), so a pointer shadow enforces the required flag
// — the same omitempty-vs-null distinction trick as the tri-state endDate.
func decodeFavoriteBody(w http.ResponseWriter, r *http.Request) (openapi.FavoriteUpdateRequest, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpsupport.MaxRequestBodySize))
	if err != nil {
		return openapi.FavoriteUpdateRequest{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var body openapi.FavoriteUpdateRequest
	if err := dec.Decode(&body); err != nil {
		return openapi.FavoriteUpdateRequest{}, err
	}
	var shadow struct {
		Favorite *bool `json:"favorite"`
	}
	if err := json.Unmarshal(raw, &shadow); err != nil {
		return openapi.FavoriteUpdateRequest{}, err
	}
	if shadow.Favorite == nil {
		return openapi.FavoriteUpdateRequest{}, errors.New("payments: favorite flag is required")
	}
	return body, nil
}

// createCommand folds the decoded body into the application command. The
// contract rules — enums, amount bounds, title length, the catalog slug, the
// end date against since — are the payment rule module's single
// responsibility (validateRule); the transport only builds the wire-only
// parts: the recurrence (int weekdays → domain constructors) and the optional
// flag default.
func createCommand(body openapi.PaymentCreateRequest) (application.CreatePaymentCommand, error) {
	recurrence, err := parseRecurrence(body.Recurrence)
	if err != nil {
		return application.CreatePaymentCommand{}, err
	}
	autoPay := false
	if body.AutoPay != nil {
		autoPay = *body.AutoPay
	}
	return application.CreatePaymentCommand{
		Type:          domain.PaymentType(body.Type),
		Title:         body.Title,
		AmountKopecks: body.AmountKopecks,
		Recurrence:    recurrence,
		PaymentForm:   domain.PaymentForm(body.PaymentForm),
		CategorySlug:  body.CategorySlug,
		EndDate:       datePtrFromWire(body.EndDate),
		AutoPay:       autoPay,
	}, nil
}

// decodeUpdateBody reads the update body once and decodes it two ways: the
// strict contract decode (bounded, unknown fields rejected) and a shadow pass
// that preserves the tri-state endDate. The shadow exists because
// encoding/json collapses absent and null onto the same nil pointer for every
// optional field, while a non-pointer json.RawMessage receives the raw "null"
// bytes — that distinction is exactly the PATCH semantics of endDate
// (omit keeps, null clears, a date sets).
func (h *PaymentHandlers) decodeUpdateBody(
	w http.ResponseWriter, r *http.Request, body *openapi.PaymentUpdateRequest,
) (json.RawMessage, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpsupport.MaxRequestBodySize))
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(body); err != nil {
		return nil, err
	}
	var shadow struct {
		EndDate json.RawMessage `json:"endDate"`
	}
	if err := json.Unmarshal(raw, &shadow); err != nil {
		return nil, err
	}
	// A nil raw message means omitted — no change; "null" and a date string
	// both arrive as bytes for endDateUpdateFromWire to resolve.
	return shadow.EndDate, nil
}

// updateCommand folds the PATCH body into the application command and
// resolves the tri-state endDate from its raw wire bytes: omitted keeps (nil
// raw), null clears, a date sets. The contract rules live in the payment
// rule module (validateRule); the transport only builds.
func updateCommand(body openapi.PaymentUpdateRequest, endDateRaw json.RawMessage) (application.UpdatePaymentCommand, error) {
	cmd := application.UpdatePaymentCommand{}
	if len(endDateRaw) > 0 {
		endDate, err := endDateUpdateFromWire(endDateRaw)
		if err != nil {
			return application.UpdatePaymentCommand{}, err
		}
		cmd.EndDate = endDate
	}
	if body.Type != nil {
		cmd.Type = new(domain.PaymentType(*body.Type))
	}
	if body.Title != nil {
		cmd.Title = body.Title
	}
	if body.AmountKopecks != nil {
		cmd.AmountKopecks = body.AmountKopecks
	}
	if body.Recurrence != nil {
		recurrence, err := parseRecurrence(*body.Recurrence)
		if err != nil {
			return application.UpdatePaymentCommand{}, err
		}
		cmd.Recurrence = &recurrence
	}
	if body.PaymentForm != nil {
		cmd.PaymentForm = new(domain.PaymentForm(*body.PaymentForm))
	}
	if body.CategorySlug != nil {
		cmd.CategorySlug = body.CategorySlug
	}
	if body.AutoPay != nil {
		cmd.AutoPay = body.AutoPay
	}
	return cmd, nil
}

// parseRecurrence converts the generated oneOf union into the domain
// recurrence. The wire constraints (weekday range and uniqueness, day and
// month bounds) are re-checked by the domain constructors; any violation —
// from either layer — maps onto the shared invalid-input outcome.
func parseRecurrence(in openapi.Recurrence) (domain.Recurrence, error) {
	value, err := in.ValueByDiscriminator()
	if err != nil {
		return domain.Recurrence{}, application.ErrInvalidInput
	}
	switch v := value.(type) {
	case openapi.RecurrenceDaily:
		return domain.NewDailyRecurrence(), nil
	case openapi.RecurrenceWeekly:
		weekdays := make([]time.Weekday, 0, len(v.Weekdays))
		for _, wd := range v.Weekdays {
			if wd < 0 || wd > 6 {
				return domain.Recurrence{}, application.ErrInvalidInput
			}
			weekdays = append(weekdays, time.Weekday(wd))
		}
		rec, err := domain.NewWeeklyRecurrence(weekdays)
		if err != nil {
			return domain.Recurrence{}, application.ErrInvalidInput
		}
		return rec, nil
	case openapi.RecurrenceMonthly:
		rec, err := domain.NewMonthlyRecurrence(v.DayOfMonth)
		if err != nil {
			return domain.Recurrence{}, application.ErrInvalidInput
		}
		return rec, nil
	case openapi.RecurrenceYearly:
		rec, err := domain.NewYearlyRecurrence(time.Month(v.Month), v.Day)
		if err != nil {
			return domain.Recurrence{}, application.ErrInvalidInput
		}
		return rec, nil
	default:
		return domain.Recurrence{}, application.ErrInvalidInput
	}
}

// endDateUpdateFromWire resolves the tri-state endDate: the raw message is
// either JSON null (clear the end date, open-ended) or a date string (set it).
func endDateUpdateFromWire(raw json.RawMessage) (*application.EndDateUpdate, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, application.ErrInvalidInput
	}
	if bytes.Equal(trimmed, []byte("null")) {
		return &application.EndDateUpdate{Value: nil}, nil
	}
	var date openapi_types.Date
	if err := json.Unmarshal(trimmed, &date); err != nil {
		return nil, application.ErrInvalidInput
	}
	return &application.EndDateUpdate{Value: new(date.Time)}, nil
}

// datePtrFromWire converts an optional request date into a *time.Time.
func datePtrFromWire(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}
	return new(d.Time)
}

// paymentResponse maps the domain rule onto the wire response: the category
// reference is resolved (CategoryView with the «Прочее» fallback), the
// recurrence goes back through its per-kind shape and the pauses travel as
// intervals.
func paymentResponse(p domain.Payment) (openapi.PaymentResponse, error) {
	recurrence, err := recurrenceResponse(p.Recurrence)
	if err != nil {
		return openapi.PaymentResponse{}, err
	}
	pauses := make([]openapi.PauseIntervalView, 0, len(p.Pauses))
	for _, pause := range p.Pauses {
		pauses = append(pauses, openapi.PauseIntervalView{
			FromDate: openapi_types.Date{Time: pause.From},
			ToDate:   httpsupport.DatePtrToOpenAPI(pause.To),
		})
	}
	return openapi.PaymentResponse{
		Id:            p.ID,
		PropertyId:    p.PropertyID,
		Type:          openapi.PaymentResponseType(p.Type),
		Title:         p.Title,
		AmountKopecks: p.AmountKopecks,
		Recurrence:    recurrence,
		Since:         openapi_types.Date{Time: p.Since},
		EndDate:       httpsupport.DatePtrToOpenAPI(p.EndDate),
		AutoPay:       p.AutoPay,
		PaymentForm:   openapi.PaymentResponsePaymentForm(p.PaymentForm),
		Category:      categoryView(p.Category),
		IsFavorite:    p.IsFavorite,
		Pauses:        pauses,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}, nil
}

// recurrenceResponse rebuilds the wire union from the domain recurrence.
func recurrenceResponse(r domain.Recurrence) (openapi.Recurrence, error) {
	var out openapi.Recurrence
	var err error
	switch r.Kind() {
	case domain.RecurrenceDaily:
		err = out.FromRecurrenceDaily(openapi.RecurrenceDaily{Kind: openapi.Daily})
	case domain.RecurrenceWeekly:
		weekdays := make([]int, len(r.Weekdays()))
		for i, wd := range r.Weekdays() {
			weekdays[i] = int(wd)
		}
		err = out.FromRecurrenceWeekly(openapi.RecurrenceWeekly{
			Kind: openapi.Weekly, Weekdays: weekdays,
		})
	case domain.RecurrenceMonthly:
		err = out.FromRecurrenceMonthly(openapi.RecurrenceMonthly{
			Kind: openapi.Monthly, DayOfMonth: r.DayOfMonth(),
		})
	case domain.RecurrenceYearly:
		err = out.FromRecurrenceYearly(openapi.RecurrenceYearly{
			Kind: openapi.Yearly, Month: int(r.Month()), Day: r.Day(),
		})
	default:
		err = fmt.Errorf("payments: unknown recurrence kind %q", r.Kind())
	}
	return out, err
}

// categoryView resolves the category reference for the response: the default
// catalog (label from the code catalog; a slug removed from the catalog falls
// back to «Прочее») or the user category. Icon and color are frontend catalog
// metadata keyed by slug and never travel in the response.
func categoryView(ref domain.CategoryRef) openapi.CategoryView {
	if ref.UserCategoryID != nil {
		id := *ref.UserCategoryID
		return openapi.CategoryView{
			Source: openapi.CategoryViewSource("custom"),
			Id:     &id,
			Label:  ref.SnapshotLabel(),
		}
	}
	view := openapi.CategoryView{
		Source: openapi.CategoryViewSource("default"),
		Label:  ref.SnapshotLabel(),
	}
	if slug := ref.SlugString(); slug != "" {
		view.Slug = &slug
	}
	return view
}

// respondWithPayment maps the rule onto the wire response with the given
// status — 201 from CreatePayment, 200 from the other payment endpoints.
func (h *PaymentHandlers) respondWithPayment(w http.ResponseWriter, r *http.Request, payment domain.Payment, status int) {
	resp, err := paymentResponse(payment)
	if err != nil {
		h.writeInternal(w, r, err)
		return
	}
	httpsupport.WriteJSON(r.Context(), w, status, resp)
}

func (h *PaymentHandlers) writeInternal(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.ErrorContext(r.Context(), "payments response mapping failed",
		slog.String("error", httpsupport.SanitizeError(err)))
	httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
}
