// Package http holds the contacts HTTP adapters (ADR 0054, ticket #507): the
// flat contact book endpoints — create, list with the property filter and
// search, get, partial update with the tri-state property binding, delete.
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

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ContactManager is the consumer-side port of these handlers (ADR 0035): the
// contact book use cases. The concrete application service satisfies it; the
// handler tests run against func-backed fakes.
type ContactManager interface {
	CreateContact(ctx context.Context, actor uuid.UUID, cmd application.CreateContactCommand) (domain.Contact, error)
	ListContacts(ctx context.Context, actor uuid.UUID, q application.ListQuery) (application.ContactBookPage, error)
	GetContact(ctx context.Context, actor, id uuid.UUID) (domain.Contact, error)
	UpdateContact(ctx context.Context, actor, id uuid.UUID, cmd application.UpdateContactCommand) (domain.Contact, error)
	DeleteContact(ctx context.Context, actor, id uuid.UUID) error
	PhotoDescriptor(ctx context.Context, actor, id uuid.UUID) (key, contentType string, err error)
	OpenContactPhoto(ctx context.Context, actor, id uuid.UUID) (body io.ReadCloser, size int64, contentType, key string, err error)
	SetContactPhoto(ctx context.Context, actor, id uuid.UUID, processed photo.Processed) (domain.Contact, error)
	DeleteContactPhoto(ctx context.Context, actor, id uuid.UUID) (domain.Contact, error)
}

// ContactHandlers implements the generated contact endpoints.
type ContactHandlers struct {
	svc    ContactManager
	logger *slog.Logger
}

// NewContactHandlers creates HTTP handlers for the contacts API.
func NewContactHandlers(svc ContactManager, logger *slog.Logger) *ContactHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &ContactHandlers{svc: svc, logger: logger}
}

// staticContactProblems maps the contacts application errors with a fixed
// outcome onto their wire status, title and detail (the shared ErrorProblem
// table shape). Invalid input is handled dynamically through the shared
// user-facing table like in every context.
var staticContactProblems = []httpsupport.ErrorProblem{
	{
		Err: application.ErrNotFound, Status: http.StatusNotFound,
		Title: httpsupport.ProblemTitleNotFound, Detail: "Не найдено",
	},
	{
		// A photo request for a card without a photo (tickets #1227/#1229):
		// the privacy-preserving 404 like properties and identity — not the
		// opaque 500 the missing row used to answer with.
		Err: application.ErrPhotoNotFound, Status: http.StatusNotFound,
		Title: httpsupport.ProblemTitleNotFound, Detail: "Не найдено",
	},
	{
		Err: application.ErrForbidden, Status: http.StatusForbidden,
		Title: httpsupport.ProblemTitleForbidden, Detail: "Недостаточно прав для этого действия",
	},
}

// handleContactError maps an application error onto the wire contract: the
// fixed table first, invalid input through its user-facing detail, anything
// else an opaque 500.
func (h *ContactHandlers) handleContactError(w http.ResponseWriter, r *http.Request, err error) {
	if httpsupport.WriteErrorProblem(r.Context(), w, err, staticContactProblems) {
		return
	}
	if errors.Is(err, application.ErrInvalidInput) {
		detail, ok := httpsupport.UserFacingDetail(err)
		if !ok {
			h.writeInternal(w, r, err)
			return
		}
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", detail))
		return
	}
	h.writeInternal(w, r, err)
}

func (h *ContactHandlers) writeInternal(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.ErrorContext(r.Context(), "contacts request failed",
		slog.String("error", httpsupport.SanitizeError(err)))
	httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError, httpsupport.InternalError(r.Context(), err))
}

// CreateContact implements POST /contacts.
func (h *ContactHandlers) CreateContact(w http.ResponseWriter, r *http.Request) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	var body openapi.ContactCreateRequest
	if err := httpsupport.DecodeJSONBody(w, r, &body); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode create contact request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	contact, err := h.svc.CreateContact(r.Context(), actor, createContactCommand(body))
	if err != nil {
		h.handleContactError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusCreated, contactResponse(contact))
}

// ListContacts implements GET /contacts. The page vocabulary is the keyset
// walk (ticket #600): limit is the window's size (the contract default when
// omitted) and cursor echoes the previous page's nextCursor.
func (h *ContactHandlers) ListContacts(w http.ResponseWriter, r *http.Request, params openapi.ListContactsParams) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	q := application.ListQuery{
		Scope: application.ListScopeAll,
		Sort:  application.ListSortName,
		Order: application.ListOrderAsc,
	}
	if params.PropertyId != nil {
		q.Scope = application.ListScopeProperty
		q.PropertyID = *params.PropertyId
	}
	if params.Search != nil {
		q.Search = *params.Search
	}
	if params.Sort != nil && *params.Sort != "" {
		q.Sort = application.ListSort(*params.Sort)
	}
	if params.Order != nil && *params.Order != "" {
		q.Order = application.ListOrder(*params.Order)
	}
	if params.Limit != nil {
		if *params.Limit < 1 || *params.Limit > application.MaxContactsPageSize {
			h.handleContactError(w, r, fmt.Errorf("limit %d: %w", *params.Limit, application.ErrInvalidInput))
			return
		}
		q.Limit = int32(*params.Limit)
	}
	if params.Cursor != nil {
		q.Cursor = *params.Cursor
	}

	page, err := h.svc.ListContacts(r.Context(), actor, q)
	if err != nil {
		h.handleContactError(w, r, err)
		return
	}

	items := make([]openapi.ContactResponse, 0, len(page.Items))
	for _, listed := range page.Items {
		items = append(items, listContactResponse(listed))
	}
	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, openapi.ContactsResponse{
		Items:      items,
		NextCursor: httpsupport.StringPtr(page.NextCursor),
	})
}

// GetContact implements GET /contacts/{contactId}.
func (h *ContactHandlers) GetContact(w http.ResponseWriter, r *http.Request, contactID openapi_types.UUID) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	contact, err := h.svc.GetContact(r.Context(), actor, contactID)
	if err != nil {
		h.handleContactError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, contactResponse(contact))
}

// UpdateContact implements PATCH /contacts/{contactId}.
func (h *ContactHandlers) UpdateContact(w http.ResponseWriter, r *http.Request, contactID openapi_types.UUID) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	var body openapi.ContactUpdateRequest
	propertyRaw, err := h.decodeUpdateBody(w, r, &body)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to decode update contact request",
			slog.String("error", httpsupport.SanitizeError(err)))
		httpsupport.WriteProblem(r.Context(), w, http.StatusBadRequest,
			httpsupport.Problem(r.Context(), "Bad request", "Некорректное тело запроса"))
		return
	}

	contact, err := h.svc.UpdateContact(r.Context(), actor, contactID, updateContactCommand(body, propertyRaw))
	if err != nil {
		h.handleContactError(w, r, err)
		return
	}

	httpsupport.WriteJSON(r.Context(), w, http.StatusOK, contactResponse(contact))
}

// DeleteContact implements DELETE /contacts/{contactId}.
func (h *ContactHandlers) DeleteContact(w http.ResponseWriter, r *http.Request, contactID openapi_types.UUID) {
	actor, ok := httpsupport.RequireUser(w, r)
	if !ok {
		return
	}

	if err := h.svc.DeleteContact(r.Context(), actor, contactID); err != nil {
		h.handleContactError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// decodeUpdateBody reads the update body once and decodes it two ways: the
// strict contract decode (bounded, unknown fields rejected) and a shadow pass
// that preserves the tri-state propertyId. The shadow exists because
// encoding/json collapses absent and null onto the same nil pointer for every
// optional field, while a non-pointer json.RawMessage receives the raw "null"
// bytes — that distinction is exactly the PATCH semantics of the property
// binding (omit keeps, null clears, a uuid moves).
func (h *ContactHandlers) decodeUpdateBody(
	w http.ResponseWriter, r *http.Request, body *openapi.ContactUpdateRequest,
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
		Property json.RawMessage `json:"propertyId"`
	}
	if err := json.Unmarshal(raw, &shadow); err != nil {
		return nil, err
	}
	return shadow.Property, nil
}

// createContactCommand folds the decoded body into the application command.
// The contract rules — the required first name, the name length cap, the
// phone and email shapes — are the contact module's single responsibility
// (domain.Validate); the transport only builds, nil optionals fold to "".
func createContactCommand(body openapi.ContactCreateRequest) application.CreateContactCommand {
	return application.CreateContactCommand{
		PropertyID:        body.PropertyId,
		FirstName:         body.FirstName,
		LastName:          stringPtr(body.LastName),
		Patronymic:        stringPtr(body.Patronymic),
		Role:              stringPtr(body.Role),
		Phone:             stringPtr(body.Phone),
		Email:             stringPtr(body.Email),
		MessengerName:     stringPtr(body.MessengerName),
		MessengerUsername: stringPtr(body.MessengerUsername),
		Note:              stringPtr(body.Note),
	}
}

// updateContactCommand folds the PATCH body into the application command and
// resolves the tri-state propertyId from its raw wire bytes: omitted keeps
// (nil raw), null clears, a uuid sets. The uuid itself is already validated
// and parsed by the strict decode pass, so the raw bytes only distinguish
// omitted from null. The contract rules live in the contact module
// (domain.Validate); the transport only builds.
func updateContactCommand(
	body openapi.ContactUpdateRequest, propertyRaw json.RawMessage,
) application.UpdateContactCommand {
	cmd := application.UpdateContactCommand{
		FirstName:         body.FirstName,
		LastName:          body.LastName,
		Patronymic:        body.Patronymic,
		Role:              body.Role,
		Phone:             body.Phone,
		Email:             body.Email,
		MessengerName:     body.MessengerName,
		MessengerUsername: body.MessengerUsername,
		Note:              body.Note,
	}
	trimmed := bytes.TrimSpace(propertyRaw)
	switch {
	case len(trimmed) == 0: // Omitted — keep the current binding.
	case bytes.Equal(trimmed, []byte("null")):
		cmd.PropertyID = &application.PropertyIDUpdate{}
	case body.PropertyId != nil:
		cmd.PropertyID = &application.PropertyIDUpdate{Value: body.PropertyId}
	}
	return cmd
}

// stringPtr folds an optional request string onto the command's plain string:
// an absent field arrives as nil and means the same as empty for the create
// draft — the service trims and validates both alike.
func stringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// contactResponse maps the domain card onto the wire response; «без объекта»
// travels as a null propertyId.
func contactResponse(c domain.Contact) openapi.ContactResponse {
	return openapi.ContactResponse{
		Id:                c.ID,
		PropertyId:        c.PropertyID,
		FirstName:         c.FirstName,
		LastName:          c.LastName,
		Patronymic:        c.Patronymic,
		Role:              c.Role,
		Phone:             c.Phone,
		Email:             c.Email,
		MessengerName:     c.MessengerName,
		MessengerUsername: c.MessengerUsername,
		Note:              c.Note,
		PhotoUrl:          contactPhotoURL(c),
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
	}
}

// contactPhotoURL is the card photo's same-origin streaming path (ADR 0065):
// nil without a photo. The path is contract-fixed; the photo bytes are
// served by GET /api/contacts/{id}/photo.
func contactPhotoURL(c domain.Contact) *string {
	if c.PhotoKey == nil {
		return nil
	}
	path := fmt.Sprintf("/api/contacts/%s/photo", c.ID)
	return &path
}

// listContactResponse maps the list projection onto the wire response: the
// bound property's display name travels alongside the id (null when the card
// is unbound), so the client labels and groups rows without re-reading
// properties.
func listContactResponse(l application.ListedContact) openapi.ContactResponse {
	response := contactResponse(l.Contact)
	if l.PropertyName != "" {
		response.PropertyName = &l.PropertyName
	}
	return response
}
