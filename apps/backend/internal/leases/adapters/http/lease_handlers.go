package http

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// LeaseHandlers implements the generated lease and tenant contact endpoints.
type LeaseHandlers struct {
	leaseSvc         *leasesapp.LeaseService
	tenantContactSvc *leasesapp.TenantContactService
	logger           *slog.Logger
	presenter        *LeasePresenter
	clock            clock.Clock
	tzResolver       sharedtz.OwnerTimezoneResolver
}

// NewLeaseHandlers creates HTTP handlers for the leases API.
func NewLeaseHandlers(leaseSvc *leasesapp.LeaseService, tenantContactSvc *leasesapp.TenantContactService, logger *slog.Logger, clk clock.Clock, tzResolver sharedtz.OwnerTimezoneResolver) *LeaseHandlers {
	return &LeaseHandlers{
		leaseSvc:         leaseSvc,
		tenantContactSvc: tenantContactSvc,
		logger:           logger,
		presenter:        NewLeasePresenter(tenantContactSvc),
		clock:            clk,
		tzResolver:       tzResolver,
	}
}

func (h *LeaseHandlers) handleLeaseError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, leasesapp.ErrInvalidInput):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", detail))
	case errors.Is(err, leasesapp.ErrNotFound):
		httpsupport.WriteProblem(w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Аренда не найдена"))
	case errors.Is(err, leasesapp.ErrForbidden):
		httpsupport.WriteProblem(w, http.StatusForbidden, httpsupport.Problem(r.Context(), "Forbidden", "Недостаточно прав для этого действия"))
	case errors.Is(err, leasesapp.ErrPropertyNotAvailable):
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "Объект недоступен для аренды"))
	case errors.Is(err, leasesapp.ErrOpenLeaseExists):
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", "У объекта уже есть открытая аренда"))
	case errors.Is(err, leasesapp.ErrAlreadyCompleted),
		errors.Is(err, leasesapp.ErrArchivedLease),
		errors.Is(err, leasesapp.ErrInvalidTransition),
		isLeaseInvalidStatusTransition(err):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", detail))
	case errors.Is(err, leasesapp.ErrTenantContactNotFound):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", detail))
	default:
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

// CreateLease implements POST /leases.
func (h *LeaseHandlers) CreateLease(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.LeaseCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create lease request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := leasesapp.CreateLeaseCommand{
		PropertyID:        body.PropertyId,
		TenantContactID:   uuidPtrFromOpenAPI(body.TenantContactId),
		StartDate:         body.StartDate.Time,
		EndDate:           datePtrFromOpenAPI(body.EndDate),
		RentAmountKopecks: int64(body.RentAmountKopecks),
		PaymentDay:        body.PaymentDay,
	}
	if body.DepositAmountKopecks != nil {
		cmd.DepositAmountKopecks = int64(*body.DepositAmountKopecks)
	}
	if body.Comment != nil {
		cmd.Comment = *body.Comment
	}

	lease, err := h.leaseSvc.CreateLease(r.Context(), actor, cmd)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	resp, err := h.presenter.LeaseResponse(r.Context(), actor, lease, nil, nil, nil, false)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, resp)
}

// ListLeases implements GET /leases.
func (h *LeaseHandlers) ListLeases(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	leases, err := h.leaseSvc.ListLeases(r.Context(), actor)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	contacts, err := h.presenter.TenantContactIDs(r.Context(), actor, leases)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	loc, err := h.tzResolver.Resolve(r.Context(), actor)
	if err != nil {
		h.handleLeaseError(w, r, fmt.Errorf("resolve owner timezone: %w", err))
		return
	}
	asOf := timeutil.DateIn(h.clock.Now(), loc)
	scheduleIndex, err := h.leaseSvc.LeasePaymentScheduleIndex(r.Context(), actor, leases, asOf)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	items := make([]openapi.LeaseResponse, 0, len(leases))
	for _, lease := range leases {
		schedule := scheduleIndex[lease.ID]
		resp, err := h.presenter.LeaseResponse(r.Context(), actor, lease, contacts, schedule.OverdueSince, schedule.NextPaymentDate, schedule.HasOverdue)
		if err != nil {
			h.handleLeaseError(w, r, err)
			return
		}
		items = append(items, resp)
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.LeasesResponse{Items: items})
}

// GetLease implements GET /leases/{id}.
func (h *LeaseHandlers) GetLease(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	lease, err := h.leaseSvc.GetLease(r.Context(), actor, id)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	loc, err := h.tzResolver.Resolve(r.Context(), actor)
	if err != nil {
		h.handleLeaseError(w, r, fmt.Errorf("resolve owner timezone: %w", err))
		return
	}
	asOf := timeutil.DateIn(h.clock.Now(), loc)
	scheduleIndex, err := h.leaseSvc.LeasePaymentScheduleIndex(r.Context(), actor, []leasesdomain.Lease{lease}, asOf)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}
	schedule := scheduleIndex[lease.ID]

	resp, err := h.presenter.LeaseResponse(r.Context(), actor, lease, nil, schedule.OverdueSince, schedule.NextPaymentDate, schedule.HasOverdue)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// UpdateLease implements PATCH /leases/{id}.
func (h *LeaseHandlers) UpdateLease(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.LeaseUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update lease request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := leasesapp.UpdateLeaseCommand{
		TenantContactID:    uuidPtrFromOpenAPI(body.TenantContactId),
		ClearTenantContact: body.ClearTenantContact,
		StartDate:          datePtrFromOpenAPI(body.StartDate),
		EndDate:            datePtrFromOpenAPI(body.EndDate),
		Comment:            body.Comment,
	}
	if body.RentAmountKopecks != nil {
		cmd.RentAmountKopecks = new(int64(*body.RentAmountKopecks))
	}
	if body.DepositAmountKopecks != nil {
		cmd.DepositAmountKopecks = new(int64(*body.DepositAmountKopecks))
	}
	if body.PaymentDay != nil {
		cmd.PaymentDay = body.PaymentDay
	}

	lease, err := h.leaseSvc.UpdateLease(r.Context(), actor, id, cmd)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	resp, err := h.presenter.LeaseResponse(r.Context(), actor, lease, nil, nil, nil, false)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// CompleteLease implements POST /leases/{id}/complete.
func (h *LeaseHandlers) CompleteLease(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	lease, err := h.leaseSvc.CompleteLease(r.Context(), actor, id)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	resp, err := h.presenter.LeaseResponse(r.Context(), actor, lease, nil, nil, nil, false)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, resp)
}

// CreateTenantContact implements POST /tenant-contacts.
func (h *LeaseHandlers) CreateTenantContact(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.TenantContactCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create tenant contact request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := leasesapp.CreateTenantContactCommand{
		Name:       body.Name,
		Surname:    body.Surname,
		Patronymic: body.Patronymic,
		Phone:      body.Phone,
		Email:      body.Email,
		Comment:    body.Comment,
	}

	contact, err := h.tenantContactSvc.CreateTenantContact(r.Context(), actor, cmd)
	if err != nil {
		handleTenantContactError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, tenantContactResponse(contact))
}

// ListTenantContacts implements GET /tenant-contacts.
func (h *LeaseHandlers) ListTenantContacts(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	contacts, err := h.tenantContactSvc.ListTenantContactsWithLeaseStatus(r.Context(), actor)
	if err != nil {
		handleTenantContactError(w, r, err)
		return
	}

	items := make([]openapi.TenantContactResponse, 0, len(contacts))
	for _, contact := range contacts {
		resp := tenantContactResponse(contact.TenantContact)
		resp.IsActive = contact.ActiveLease != nil

		contactsMap := map[uuid.UUID]leasesdomain.TenantContact{
			contact.ID: contact.TenantContact,
		}

		if contact.ActiveLease != nil {
			leaseResp, err := h.presenter.LeaseResponse(r.Context(), actor, *contact.ActiveLease, contactsMap, nil, nil, false)
			if err != nil {
				handleTenantContactError(w, r, err)
				return
			}
			resp.ActiveLease = &leaseResp
		}
		if contact.LastLease != nil {
			leaseResp, err := h.presenter.LeaseResponse(r.Context(), actor, *contact.LastLease, contactsMap, nil, nil, false)
			if err != nil {
				handleTenantContactError(w, r, err)
				return
			}
			resp.LastLease = &leaseResp
		}
		items = append(items, resp)
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.TenantContactsResponse{Items: items})
}

// GetTenantContact implements GET /tenant-contacts/{id}.
func (h *LeaseHandlers) GetTenantContact(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	contact, err := h.tenantContactSvc.GetTenantContact(r.Context(), actor, id)
	if err != nil {
		handleTenantContactError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, tenantContactResponse(contact))
}

// UpdateTenantContact implements PATCH /tenant-contacts/{id}.
func (h *LeaseHandlers) UpdateTenantContact(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	actor, ok := httpsupport.UserIDFromContext(r.Context())
	if !ok {
		httpsupport.WriteProblem(w, http.StatusUnauthorized, httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
		return
	}

	var body openapi.TenantContactUpdateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update tenant contact request", slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	cmd := leasesapp.UpdateTenantContactCommand{
		Name:       body.Name,
		Surname:    body.Surname,
		Patronymic: body.Patronymic,
		Phone:      body.Phone,
		Email:      body.Email,
		Comment:    body.Comment,
	}

	contact, err := h.tenantContactSvc.UpdateTenantContact(r.Context(), actor, id, cmd)
	if err != nil {
		handleTenantContactError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, tenantContactResponse(contact))
}

func handleTenantContactError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, leasesapp.ErrInvalidInput):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(w, http.StatusBadRequest, httpsupport.Problem(r.Context(), "Bad request", detail))
	case errors.Is(err, leasesapp.ErrNotFound):
		httpsupport.WriteProblem(w, http.StatusNotFound, httpsupport.Problem(r.Context(), "Not found", "Арендатор не найден"))
	case errors.Is(err, leasesapp.ErrDuplicatePhone):
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
			return
		}
		httpsupport.WriteProblem(w, http.StatusConflict, httpsupport.Problem(r.Context(), "Conflict", detail))
	default:
		httpsupport.WriteProblem(w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
	}
}

func tenantContactResponse(contact leasesdomain.TenantContact) openapi.TenantContactResponse {
	return openapi.TenantContactResponse{
		Id:         contact.ID,
		OwnerId:    contact.OwnerID,
		Name:       contact.Name,
		Surname:    contact.Surname,
		Patronymic: contact.Patronymic,
		Phone:      contact.Phone,
		Email:      contact.Email,
		Comment:    contact.Comment,
		CreatedAt:  contact.CreatedAt,
		UpdatedAt:  contact.UpdatedAt,
	}
}

func isLeaseInvalidStatusTransition(err error) bool {
	var target *leasesapp.InvalidStatusTransitionError
	return errors.As(err, &target)
}

func datePtrFromOpenAPI(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}
	return new(d.Time)
}

func uuidPtrFromOpenAPI(u *openapi_types.UUID) *uuid.UUID {
	if u == nil {
		return nil
	}
	return u
}
