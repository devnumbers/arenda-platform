package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// LeaseHandlers implements the generated lease and tenant contact endpoints.
type LeaseHandlers struct {
	leaseSvc         *leasesapp.LeaseService
	tenantContactSvc *leasesapp.TenantContactService
	logger           *slog.Logger
}

// NewLeaseHandlers creates HTTP handlers for the leases API.
func NewLeaseHandlers(leaseSvc *leasesapp.LeaseService, tenantContactSvc *leasesapp.TenantContactService, logger *slog.Logger) *LeaseHandlers {
	return &LeaseHandlers{
		leaseSvc:         leaseSvc,
		tenantContactSvc: tenantContactSvc,
		logger:           logger,
	}
}

func (h *LeaseHandlers) handleLeaseError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, leasesapp.ErrInvalidInput):
		detail, _ := UserFacingDetail(err)
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	case errors.Is(err, leasesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "lease not found"))
	case errors.Is(err, leasesapp.ErrPropertyNotAvailable):
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "property is not available for lease"))
	case errors.Is(err, leasesapp.ErrOpenLeaseExists):
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", "property already has an open lease"))
	case errors.Is(err, leasesapp.ErrAlreadyCompleted),
		errors.Is(err, leasesapp.ErrArchivedLease),
		errors.Is(err, leasesapp.ErrInvalidTransition),
		isLeaseInvalidStatusTransition(err):
		detail, _ := UserFacingDetail(err)
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", detail))
	case errors.Is(err, leasesapp.ErrTenantContactNotFound):
		detail, _ := UserFacingDetail(err)
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	default:
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
	}
}

// CreateLease implements POST /leases.
func (h *LeaseHandlers) CreateLease(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.LeaseCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create lease request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
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

	lease, err := h.leaseSvc.CreateLease(r.Context(), ownerID, cmd)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	resp, err := h.leaseResponse(r.Context(), ownerID, lease)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, resp)
}

// ListLeases implements GET /leases.
func (h *LeaseHandlers) ListLeases(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	leases, err := h.leaseSvc.ListLeases(r.Context(), ownerID)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	items := make([]openapi.LeaseResponse, 0, len(leases))
	for _, lease := range leases {
		resp, err := h.leaseResponse(r.Context(), ownerID, lease)
		if err != nil {
			h.handleLeaseError(w, r, err)
			return
		}
		items = append(items, resp)
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.LeasesResponse{Items: items})
}

// GetLease implements GET /leases/{id}.
func (h *LeaseHandlers) GetLease(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	lease, err := h.leaseSvc.GetLease(r.Context(), ownerID, id)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	resp, err := h.leaseResponse(r.Context(), ownerID, lease)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
}

// UpdateLease implements PATCH /leases/{id}.
func (h *LeaseHandlers) UpdateLease(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.LeaseUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update lease request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
		return
	}

	cmd := leasesapp.UpdateLeaseCommand{
		TenantContactID: uuidPtrFromOpenAPI(body.TenantContactId),
		StartDate:       datePtrFromOpenAPI(body.StartDate),
		EndDate:         datePtrFromOpenAPI(body.EndDate),
		Comment:         body.Comment,
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

	lease, err := h.leaseSvc.UpdateLease(r.Context(), ownerID, id, cmd)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	resp, err := h.leaseResponse(r.Context(), ownerID, lease)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
}

// CompleteLease implements POST /leases/{id}/complete.
func (h *LeaseHandlers) CompleteLease(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	lease, err := h.leaseSvc.CompleteLease(r.Context(), ownerID, id)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	resp, err := h.leaseResponse(r.Context(), ownerID, lease)
	if err != nil {
		h.handleLeaseError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, resp)
}

// CreateTenantContact implements POST /tenant-contacts.
func (h *LeaseHandlers) CreateTenantContact(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.TenantContactCreateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create tenant contact request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
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

	contact, err := h.tenantContactSvc.CreateTenantContact(r.Context(), ownerID, cmd)
	if err != nil {
		handleTenantContactError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, tenantContactResponse(contact))
}

// ListTenantContacts implements GET /tenant-contacts.
func (h *LeaseHandlers) ListTenantContacts(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	contacts, err := h.tenantContactSvc.ListTenantContacts(r.Context(), ownerID)
	if err != nil {
		handleTenantContactError(w, r, err)
		return
	}

	items := make([]openapi.TenantContactResponse, 0, len(contacts))
	for _, contact := range contacts {
		items = append(items, tenantContactResponse(contact))
	}

	writeJSON(r.Context(), w, http.StatusOK, openapi.TenantContactsResponse{Items: items})
}

// GetTenantContact implements GET /tenant-contacts/{id}.
func (h *LeaseHandlers) GetTenantContact(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	contact, err := h.tenantContactSvc.GetTenantContact(r.Context(), ownerID, id)
	if err != nil {
		handleTenantContactError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, tenantContactResponse(contact))
}

// UpdateTenantContact implements PATCH /tenant-contacts/{id}.
func (h *LeaseHandlers) UpdateTenantContact(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	ownerID, ok := ownerIDFromContext(r)
	if !ok {
		writeProblem(w, http.StatusUnauthorized, problem(r.Context(), "Unauthorized", "session required"))
		return
	}

	var body openapi.TenantContactUpdateRequest
	if err := decodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update tenant contact request", slog.String("error", sanitizeError(err)))
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", "invalid request body"))
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

	contact, err := h.tenantContactSvc.UpdateTenantContact(r.Context(), ownerID, id, cmd)
	if err != nil {
		handleTenantContactError(w, r, err)
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, tenantContactResponse(contact))
}

func handleTenantContactError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, leasesapp.ErrInvalidInput):
		detail, _ := UserFacingDetail(err)
		writeProblem(w, http.StatusBadRequest, problem(r.Context(), "Bad request", detail))
	case errors.Is(err, leasesapp.ErrNotFound):
		writeProblem(w, http.StatusNotFound, problem(r.Context(), "Not found", "tenant contact not found"))
	case errors.Is(err, leasesapp.ErrDuplicatePhone):
		detail, _ := UserFacingDetail(err)
		writeProblem(w, http.StatusConflict, problem(r.Context(), "Conflict", detail))
	default:
		writeProblem(w, http.StatusInternalServerError, internalError(r.Context(), err))
	}
}

func (h *LeaseHandlers) leaseResponse(ctx context.Context, ownerID uuid.UUID, lease domain.Lease) (openapi.LeaseResponse, error) {
	resp := openapi.LeaseResponse{
		Id:                   lease.ID,
		OwnerId:              lease.OwnerID,
		PropertyId:           lease.PropertyID,
		Status:               openapi.LeaseResponseStatus(lease.Status),
		StartDate:            openapi_types.Date{Time: lease.StartDate},
		EndDate:              datePtrToOpenAPI(lease.EndDate),
		RentAmountKopecks:    int(lease.RentAmountKopecks),
		DepositAmountKopecks: int(lease.DepositAmountKopecks),
		PaymentDay:           lease.PaymentDay,
		CreatedAt:            lease.CreatedAt,
		UpdatedAt:            lease.UpdatedAt,
	}
	if lease.Comment != "" {
		resp.Comment = &lease.Comment
	}
	if lease.TenantContactID != nil && *lease.TenantContactID != uuid.Nil {
		contact, err := h.tenantContactSvc.GetTenantContact(ctx, ownerID, *lease.TenantContactID)
		if err != nil {
			return openapi.LeaseResponse{}, err
		}
		resp.TenantContact = new(tenantContactResponse(contact))
	}
	return resp, nil
}

func tenantContactResponse(contact domain.TenantContact) openapi.TenantContactResponse {
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

func datePtrToOpenAPI(t *time.Time) *openapi_types.Date {
	if t == nil {
		return nil
	}
	return &openapi_types.Date{Time: *t}
}

func uuidPtrFromOpenAPI(u *openapi_types.UUID) *uuid.UUID {
	if u == nil {
		return nil
	}
	return u
}
