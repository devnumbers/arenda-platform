// Package http holds the rentals HTTP adapters (ADR 0053 §4, ticket #529):
// the create/list/get/patch/delete endpoints of the rental, the completion
// and the period summary — nested under the property like payments and
// contacts.
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
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	rentalsapp "github.com/nambers/arenda-planform/apps/backend/internal/rentals/application"
	rentalsdomain "github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// RentalManager is the consumer-side port of these handlers (ADR 0035): the
// rental use cases. The concrete application service satisfies it; the
// handler tests run against func-backed fakes that back these methods one to
// one — a test double, not a second contract.
type RentalManager interface {
	// The wire signatures are long — each use case wraps its command and view
	// types; the block is backed one to one by the test double.
	CreateRental(
		ctx context.Context, actor, propertyID uuid.UUID, cmd rentalsapp.CreateRentalCommand,
	) (rentalsapp.RentalView, error)
	ListRentals(ctx context.Context, actor, propertyID uuid.UUID) ([]rentalsapp.RentalView, error)
	GetRental(ctx context.Context, actor, propertyID, rentalID uuid.UUID) (rentalsapp.RentalView, error)
	UpdateRental(
		ctx context.Context, actor, propertyID, rentalID uuid.UUID, cmd rentalsapp.UpdateRentalCommand,
	) (rentalsapp.RentalView, error)
	CompleteRental(
		ctx context.Context, actor, propertyID, rentalID uuid.UUID, cmd rentalsapp.CompleteRentalCommand,
	) (rentalsapp.RentalView, error)
	DeleteRental(ctx context.Context, actor, propertyID, rentalID uuid.UUID) error
	RentalSummary(
		ctx context.Context, actor, propertyID, rentalID uuid.UUID, until *time.Time,
	) (rentalsapp.RentalSummary, error)
}

// RentalHandlers implements the generated rental endpoints.
type RentalHandlers struct {
	svc    RentalManager
	logger *slog.Logger
}

// NewRentalHandlers creates HTTP handlers for the rentals API.
func NewRentalHandlers(svc RentalManager, logger *slog.Logger) *RentalHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &RentalHandlers{svc: svc, logger: logger}
}

// staticRentalProblems maps the rentals application errors with a fixed
// outcome onto their wire status, title and detail (the shared ErrorProblem
// table shape). Invalid input is handled dynamically through the shared
// user-facing table like in every context.
var staticRentalProblems = []httpsupport.ErrorProblem{
	{
		Err: rentalsapp.ErrNotFound, Status: http.StatusNotFound,
		Title: httpsupport.ProblemTitleNotFound, Detail: "Не найдено",
	},
	{
		Err: rentalsapp.ErrForbidden, Status: http.StatusForbidden,
		Title: httpsupport.ProblemTitleForbidden, Detail: "Недостаточно прав для этого действия",
	},
	{
		Err: rentalsapp.ErrArchivedProperty, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "Нельзя изменить архивный объект",
	},
	{
		Err: rentalsapp.ErrPropertyOccupied, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "На объекте уже есть незавершённая аренда",
	},
	{
		Err: rentalsapp.ErrRentalCompleted, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "Аренда уже завершена",
	},
	{
		Err: rentalsapp.ErrRentalStarted, Status: http.StatusConflict,
		Title: httpsupport.ProblemTitleConflict, Detail: "Начавшуюся аренду можно только завершить",
	},
}

// writeRentalsError maps an error with a fixed outcome onto the wire via the
// rentals problem table, then handles invalid input through its user-facing
// detail. The bool verdict lets each handler turn anything unrecognized into
// its own opaque 500.
func writeRentalsError(ctx context.Context, w http.ResponseWriter, err error) bool {
	// The coded maintenance 409 first (ticket #1050): the flat table cannot
	// carry the extension code — the same shape as the properties adapter's
	// coded occupied/suspended mappings. The client needs the code to
	// distinguish the guard from the other conflicts.
	if errors.Is(err, rentalsapp.ErrPropertyMaintenance) {
		httpsupport.WriteProblem(ctx, w, http.StatusConflict,
			httpsupport.ProblemWithCode(ctx,
				httpsupport.ProblemTitleConflict,
				"Объект на ремонте — аренда недоступна",
				"property_maintenance"))
		return true
	}
	if httpsupport.WriteErrorProblem(ctx, w, err, staticRentalProblems) {
		return true
	}
	if errors.Is(err, rentalsapp.ErrInvalidInput) {
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			return false
		}
		httpsupport.WriteProblem(ctx, w, http.StatusBadRequest, httpsupport.Problem(ctx, "Bad request", detail))
		return true
	}
	return false
}

// handleRentalError maps an application error onto the wire contract: the
// fixed table first, invalid input through its user-facing detail, anything
// else an opaque 500.
func (h *RentalHandlers) handleRentalError(w http.ResponseWriter, r *http.Request, err error) {
	if writeRentalsError(r.Context(), w, err) {
		return
	}
	h.logger.ErrorContext(r.Context(), "rentals request failed",
		slog.String("error", httpsupport.SanitizeError(err)))
	httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError,
		httpsupport.Problem(r.Context(), "Internal Server Error", "Внутренняя ошибка сервера"))
}

// CreateRental implements POST /properties/{propertyID}/rentals.
func (h *RentalHandlers) CreateRental(w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	var body openapi.RentalCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create rental request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd, err := createRentalCommand(body)
	if err != nil {
		h.handleRentalError(w, r, err)
		return
	}

	view, err := h.svc.CreateRental(r.Context(), actor, propertyID, cmd)
	if err != nil {
		h.handleRentalError(w, r, err)
		return
	}

	h.respondWithRental(w, r, view, http.StatusCreated)
}

// ListRentals implements GET /properties/{propertyID}/rentals.
func (h *RentalHandlers) ListRentals(w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	views, err := h.svc.ListRentals(r.Context(), actor, propertyID)
	if err != nil {
		h.handleRentalError(w, r, err)
		return
	}

	items := make([]openapi.RentalResponse, 0, len(views))
	for _, view := range views {
		item, err := rentalResponse(view)
		if err != nil {
			h.handleRentalError(w, r, err)
			return
		}
		items = append(items, item)
	}
	h.writeJSON(w, r, http.StatusOK, openapi.RentalsResponse{Items: items})
}

// GetRental implements GET /properties/{propertyID}/rentals/{rentalID}.
func (h *RentalHandlers) GetRental(
	w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId, rentalID openapi_types.UUID,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	view, err := h.svc.GetRental(r.Context(), actor, propertyID, rentalID)
	if err != nil {
		h.handleRentalError(w, r, err)
		return
	}

	h.respondWithRental(w, r, view, http.StatusOK)
}

// UpdateRental implements PATCH /properties/{propertyID}/rentals/{rentalID}.
func (h *RentalHandlers) UpdateRental(
	w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId, rentalID openapi_types.UUID,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	var body openapi.RentalUpdateRequest
	clears, err := h.decodeUpdateBody(w, r, &body)
	if err != nil {
		return
	}

	cmd, err := updateRentalCommand(body, clears)
	if err != nil {
		h.handleRentalError(w, r, err)
		return
	}

	view, err := h.svc.UpdateRental(r.Context(), actor, propertyID, rentalID, cmd)
	if err != nil {
		h.handleRentalError(w, r, err)
		return
	}

	h.respondWithRental(w, r, view, http.StatusOK)
}

// DeleteRental implements DELETE /properties/{propertyID}/rentals/{rentalID}.
func (h *RentalHandlers) DeleteRental(
	w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId, rentalID openapi_types.UUID,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	if err := h.svc.DeleteRental(r.Context(), actor, propertyID, rentalID); err != nil {
		h.handleRentalError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CompleteRental implements POST …/rentals/{rentalID}/complete.
func (h *RentalHandlers) CompleteRental(
	w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId, rentalID openapi_types.UUID,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	var body openapi.RentalCompleteRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode complete rental request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := completeRentalCommand(body)

	view, err := h.svc.CompleteRental(r.Context(), actor, propertyID, rentalID, cmd)
	if err != nil {
		h.handleRentalError(w, r, err)
		return
	}

	h.respondWithRental(w, r, view, http.StatusOK)
}

// GetRentalSummary implements GET …/rentals/{rentalID}/summary.
func (h *RentalHandlers) GetRentalSummary(
	w http.ResponseWriter, r *http.Request, propertyID openapi.PropertyId,
	rentalID openapi_types.UUID, params openapi.GetRentalSummaryParams,
) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	var until *time.Time
	if params.Until != nil {
		until = &params.Until.Time
	}
	summary, err := h.svc.RentalSummary(r.Context(), actor, propertyID, rentalID, until)
	if err != nil {
		h.handleRentalError(w, r, err)
		return
	}

	h.writeJSON(w, r, http.StatusOK, openapi.RentalSummaryResponse{
		From:           dateToOpenAPI(summary.From),
		Until:          dateToOpenAPI(summary.Until),
		IncomeKopecks:  summary.IncomeKopecks,
		ExpenseKopecks: summary.ExpenseKopecks,
		ProfitKopecks:  summary.ProfitKopecks,
	})
}

// updateClears names the nullable fields the request explicitly nulls —
// the tri-state «явный null = очистить» of the PATCH contract (ADR 0053 §4).
type updateClears struct {
	PlannedEndDate bool
	DepositKopecks bool
	Commission     bool
	ContactID      bool
	Comment        bool
}

// decodeUpdateBody decodes the PATCH body and resolves the tri-state fields
// from their raw wire bytes: omitted keeps (no raw), null clears, a value
// sets. A decode failure writes the 400 problem itself and returns an error.
func (h *RentalHandlers) decodeUpdateBody(
	w http.ResponseWriter, r *http.Request, body *openapi.RentalUpdateRequest,
) (updateClears, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, httpsupport.MaxRequestBodySize))
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to read update rental request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return updateClears{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update rental request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return updateClears{}, err
	}
	var shadow struct {
		PlannedEndDate    json.RawMessage `json:"plannedEndDate"`
		DepositKopecks    json.RawMessage `json:"depositKopecks"`
		CommissionKopecks json.RawMessage `json:"commissionKopecks"`
		ContactID         json.RawMessage `json:"contactId"`
		Comment           json.RawMessage `json:"comment"`
	}
	if err := json.Unmarshal(raw, &shadow); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update rental fields",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return updateClears{}, err
	}
	return updateClears{
		PlannedEndDate: isNullJSON(shadow.PlannedEndDate),
		DepositKopecks: isNullJSON(shadow.DepositKopecks),
		Commission:     isNullJSON(shadow.CommissionKopecks),
		ContactID:      isNullJSON(shadow.ContactID),
		Comment:        isNullJSON(shadow.Comment),
	}, nil
}

// isNullJSON reports whether the raw field bytes are an explicit JSON null
// (omitted fields arrive as nil raw).
func isNullJSON(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) == "null"
}

// createRentalCommand folds the create body into the application command.
func createRentalCommand(body openapi.RentalCreateRequest) (rentalsapp.CreateRentalCommand, error) {
	day, err := paymentDayFromWire(body.PaymentDay)
	if err != nil {
		return rentalsapp.CreateRentalCommand{}, err
	}
	cmd := rentalsapp.CreateRentalCommand{
		AmountKopecks:  body.AmountKopecks,
		PaymentDay:     day,
		StartDate:      body.StartDate.Time,
		PlannedEndDate: datePtrFromWire(body.PlannedEndDate),
		Utilities:      rentalsdomain.Utilities(body.Utilities),
		ContactID:      uuidPtrFromWire(body.ContactId),
		Comment:        stringPtrFromWire(body.Comment),
		AutoPay:        body.AutoPay,
	}
	if body.ReminderOffsetDays != nil {
		cmd.ReminderOffsetDays = new(int(*body.ReminderOffsetDays))
	}
	if body.DepositKopecks != nil {
		cmd.DepositKopecks = body.DepositKopecks
	}
	if body.CommissionKopecks != nil {
		cmd.CommissionKopecks = body.CommissionKopecks
	}
	return cmd, nil
}

// updateRentalCommand folds the PATCH body into the application command; the
// tri-state nullable fields ride the clears verdict.
func updateRentalCommand(body openapi.RentalUpdateRequest, clears updateClears) (rentalsapp.UpdateRentalCommand, error) {
	cmd, err := paymentTermsFromUpdate(body, clears)
	if err != nil {
		return rentalsapp.UpdateRentalCommand{}, err
	}
	rentalTermsFromUpdate(body, clears, &cmd)
	return cmd, nil
}

// paymentTermsFromUpdate folds the payment-synced terms — the fields the
// managed rent payment carries. The payment day decode is the only
// fallible step.
func paymentTermsFromUpdate(
	body openapi.RentalUpdateRequest, clears updateClears,
) (rentalsapp.UpdateRentalCommand, error) {
	var cmd rentalsapp.UpdateRentalCommand
	if body.AmountKopecks != nil {
		cmd.AmountKopecks = body.AmountKopecks
	}
	if body.AutoPay != nil {
		cmd.AutoPay = body.AutoPay
	}
	if body.PaymentDay != nil {
		day, err := paymentDayFromWire(*body.PaymentDay)
		if err != nil {
			return rentalsapp.UpdateRentalCommand{}, err
		}
		cmd.PaymentDay = &day
	}
	if body.PlannedEndDate != nil {
		cmd.PlannedEndDate = &rentalsapp.DateUpdate{Value: datePtrFromWire(body.PlannedEndDate)}
	} else if clears.PlannedEndDate {
		cmd.PlannedEndDate = &rentalsapp.DateUpdate{}
	}
	return cmd, nil
}

// rentalTermsFromUpdate folds the rentals-only terms (the records the
// payment does not carry); the tri-state nullable fields ride the clears.
func rentalTermsFromUpdate(body openapi.RentalUpdateRequest, clears updateClears, cmd *rentalsapp.UpdateRentalCommand) {
	if body.Utilities != nil {
		utilities := rentalsdomain.Utilities(*body.Utilities)
		cmd.Utilities = &utilities
	}
	if body.DepositKopecks != nil {
		cmd.DepositKopecks = &rentalsapp.AmountUpdate{Value: body.DepositKopecks}
	} else if clears.DepositKopecks {
		cmd.DepositKopecks = &rentalsapp.AmountUpdate{}
	}
	if body.CommissionKopecks != nil {
		cmd.CommissionKopecks = &rentalsapp.AmountUpdate{Value: body.CommissionKopecks}
	} else if clears.Commission {
		cmd.CommissionKopecks = &rentalsapp.AmountUpdate{}
	}
	if body.ContactId != nil {
		cmd.ContactID = &rentalsapp.ContactUpdate{Value: uuidPtrFromWire(body.ContactId)}
	} else if clears.ContactID {
		cmd.ContactID = &rentalsapp.ContactUpdate{}
	}
	if body.Comment != nil {
		cmd.Comment = &rentalsapp.StringUpdate{Value: body.Comment}
	} else if clears.Comment {
		cmd.Comment = &rentalsapp.StringUpdate{}
	}
}

// completeRentalCommand folds the completion body into the application
// command; the contract's required amountKopecks makes «комментарий без
// суммы» unrepresentable here.
func completeRentalCommand(body openapi.RentalCompleteRequest) rentalsapp.CompleteRentalCommand {
	cmd := rentalsapp.CompleteRentalCommand{
		CompletedDate: body.CompletedDate.Time,
	}
	if body.DepositReturn != nil {
		ret := &rentalsapp.DepositReturn{AmountKopecks: body.DepositReturn.AmountKopecks}
		if body.DepositReturn.Comment != nil {
			ret.Comment = body.DepositReturn.Comment
		}
		cmd.DepositReturn = ret
	}
	return cmd
}

// paymentDayFromWire converts the generated union onto the domain payment
// day: the integer 1..31 or the "last" marker. The domain constructors
// re-check the bounds; any violation maps onto the shared invalid-input
// outcome.
func paymentDayFromWire(in openapi.RentalPaymentDay) (rentalsdomain.PaymentDay, error) {
	if day, err := in.AsRentalPaymentDay0(); err == nil {
		return rentalsdomain.NewPaymentDay(day)
	}
	marker, err := in.AsRentalPaymentDay1()
	if err != nil {
		return rentalsdomain.PaymentDay{}, rentalsapp.ErrInvalidInput
	}
	if marker == "last" {
		return rentalsdomain.NewLastPaymentDay(), nil
	}
	return rentalsdomain.PaymentDay{}, rentalsapp.ErrInvalidInput
}

// paymentDayToWire encodes the domain payment day onto the generated union:
// the last-day marker travels as "last", its day as 1..30 (31 is the same
// behaviour, решение №5 — the marker is the canonical spelling).
func paymentDayToWire(day rentalsdomain.PaymentDay) (openapi.RentalPaymentDay, error) {
	var out openapi.RentalPaymentDay
	if day.IsLast() {
		if err := out.FromRentalPaymentDay1("last"); err != nil {
			return out, fmt.Errorf("encode payment day: %w", err)
		}
		return out, nil
	}
	if err := out.FromRentalPaymentDay0(day.Day()); err != nil {
		return out, fmt.Errorf("encode payment day: %w", err)
	}
	return out, nil
}

// datePtrFromWire converts the wire date pointer onto the domain's
// UTC-midnight calendar date; a nil pointer stays nil.
func datePtrFromWire(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}

// uuidPtrFromWire passes the optional contact reference through.
func uuidPtrFromWire(u *openapi_types.UUID) *uuid.UUID {
	return u
}

// stringPtrFromWire passes the optional comment through.
func stringPtrFromWire(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// respondWithRental writes the assembled view as the wire response.
func (h *RentalHandlers) respondWithRental(
	w http.ResponseWriter, r *http.Request, view rentalsapp.RentalView, status int,
) {
	response, err := rentalResponse(view)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to encode rental view",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError,
			httpsupport.Problem(r.Context(), "Internal Server Error", "Внутренняя ошибка сервера"))
		return
	}
	h.writeJSON(w, r, status, response)
}

// rentalResponse maps the assembled view onto the wire contract.
func rentalResponse(view rentalsapp.RentalView) (openapi.RentalResponse, error) {
	rental := view.Rental
	response := openapi.RentalResponse{
		Id:                   rental.ID,
		PropertyId:           rental.PropertyID,
		Status:               openapi.RentalResponseStatus(view.Status),
		StartDate:            dateToOpenAPI(rental.StartDate),
		PlannedEndDate:       httpsupport.DatePtrToOpenAPI(rental.PlannedEndDate),
		CompletedDate:        httpsupport.DatePtrToOpenAPI(rental.CompletedDate),
		Utilities:            openapi.RentalResponseUtilities(rental.Utilities),
		DepositKopecks:       rental.DepositKopecks,
		CommissionKopecks:    rental.CommissionKopecks,
		DepositReturnKopecks: rental.DepositReturnKopecks,
		DepositReturnComment: rental.DepositReturnComment,
		Tenant:               tenantToWire(rental.Tenant),
		Comment:              rental.Comment,
		Progress: openapi.RentalProgress{
			PaidMonths:      view.Progress.PaidMonths,
			TotalMonths:     view.Progress.TotalMonths,
			MonthsRemaining: view.Progress.MonthsRemaining,
			OverdueMonths:   view.Progress.OverdueMonths,
		},
		Today:     dateToOpenAPI(view.Today),
		CreatedAt: rental.CreatedAt,
	}
	paymentDay, err := paymentDayToWire(view.Payment.PaymentDay)
	if err != nil {
		return openapi.RentalResponse{}, err
	}
	// Ревизия #1161: у завершённой аренды платёж удалён — идентификатора
	// нет (wire null), а условия рендерятся из Архива условий; uuid.Nil —
	// маркер отсутствия живой ссылки.
	var paymentID *openapi_types.UUID
	if view.Payment.PaymentID != uuid.Nil {
		paymentID = &view.Payment.PaymentID
	}
	response.RentPayment = openapi.RentalPaymentView{
		PaymentId:     paymentID,
		AmountKopecks: view.Payment.AmountKopecks,
		PaymentDay:    paymentDay,
		AutoPay:       view.Payment.AutoPay,
		NextPayment:   nextPaymentToWire(view.NextPayment, view.Today),
	}
	if view.Payment.ReminderOffsetDays != nil {
		offset := openapi.RentalPaymentViewReminderOffsetDays(*view.Payment.ReminderOffsetDays)
		response.RentPayment.ReminderOffsetDays = &offset
	}
	return response, nil
}

// tenantToWire maps the embedded tenant; nil stays null — «Контакта нет».
func tenantToWire(tenant *rentalsdomain.TenantContact) *openapi.TenantView {
	if tenant == nil {
		return nil
	}
	return &openapi.TenantView{
		ContactId: tenant.ContactID,
		FirstName: tenant.FirstName,
		LastName:  tenant.LastName,
		Phone:     tenant.Phone,
	}
}

// nextPaymentToWire maps the single future planned occurrence; the days
// until count against the view's today (the owner's calendar date).
func nextPaymentToWire(next *rentalsapp.PlannedOccurrence, today time.Time) *openapi.RentalNextPayment {
	if next == nil {
		return nil
	}
	return &openapi.RentalNextPayment{
		OperationId:   next.OperationID,
		Date:          dateToOpenAPI(next.Date),
		AmountKopecks: next.AmountKopecks,
		DaysUntil:     int(next.Date.Sub(today).Hours() / 24),
	}
}

// writeJSON encodes the response body.
func (h *RentalHandlers) writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to encode rentals response",
			slog.String("error", httpsupport.SanitizeError(err)))
	}
}

// dateToOpenAPI converts a calendar date onto the generated openapi Date.
func dateToOpenAPI(t time.Time) openapi_types.Date {
	return openapi_types.Date{Time: t}
}
