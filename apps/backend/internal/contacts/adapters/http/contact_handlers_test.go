package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/photo"
)

// fakeContactManager is the func-backed ContactManager double: every use case
// is optional; an unset one fails the test loudly instead of silently
// succeeding (ADR 0035 test doubles). The func fields back the port's
// methods one to one; the field order is alphabetical, unlike the port.
type fakeContactManager struct {
	create          func(ctx context.Context, actor uuid.UUID, cmd application.CreateContactCommand) (domain.Contact, error)
	del             func(ctx context.Context, actor, id uuid.UUID) error
	get             func(ctx context.Context, actor, id uuid.UUID) (domain.Contact, error)
	list            func(ctx context.Context, actor uuid.UUID, q application.ListQuery) (application.ContactBookPage, error)
	update          func(ctx context.Context, actor, id uuid.UUID, cmd application.UpdateContactCommand) (domain.Contact, error)
	photoDescriptor func(ctx context.Context, actor, id uuid.UUID) (string, string, error)
	openPhoto       func(ctx context.Context, actor, id uuid.UUID) (io.ReadCloser, int64, string, string, error)
	setPhoto        func(ctx context.Context, actor, id uuid.UUID, processed photo.Processed) (domain.Contact, error)
	deletePhoto     func(ctx context.Context, actor, id uuid.UUID) (domain.Contact, error)
}

func (f *fakeContactManager) CreateContact(
	ctx context.Context, actor uuid.UUID, cmd application.CreateContactCommand,
) (domain.Contact, error) {
	if f.create == nil {
		return domain.Contact{}, errors.New("unexpected CreateContact call")
	}
	return f.create(ctx, actor, cmd)
}

func (f *fakeContactManager) ListContacts(
	ctx context.Context, actor uuid.UUID, q application.ListQuery,
) (application.ContactBookPage, error) {
	if f.list == nil {
		return application.ContactBookPage{}, errors.New("unexpected ListContacts call")
	}
	return f.list(ctx, actor, q)
}

func (f *fakeContactManager) GetContact(ctx context.Context, actor, id uuid.UUID) (domain.Contact, error) {
	if f.get == nil {
		return domain.Contact{}, errors.New("unexpected GetContact call")
	}
	return f.get(ctx, actor, id)
}

func (f *fakeContactManager) UpdateContact(
	ctx context.Context, actor, id uuid.UUID, cmd application.UpdateContactCommand,
) (domain.Contact, error) {
	if f.update == nil {
		return domain.Contact{}, errors.New("unexpected UpdateContact call")
	}
	return f.update(ctx, actor, id, cmd)
}

func (f *fakeContactManager) DeleteContact(ctx context.Context, actor, id uuid.UUID) error {
	if f.del == nil {
		return errors.New("unexpected DeleteContact call")
	}
	return f.del(ctx, actor, id)
}

func (f *fakeContactManager) PhotoDescriptor(
	ctx context.Context, actor, id uuid.UUID,
) (key, contentType string, err error) {
	if f.photoDescriptor == nil {
		return "", "", errors.New("unexpected PhotoDescriptor call")
	}
	return f.photoDescriptor(ctx, actor, id)
}

func (f *fakeContactManager) OpenContactPhoto(
	ctx context.Context, actor, id uuid.UUID,
) (body io.ReadCloser, size int64, contentType, key string, err error) {
	if f.openPhoto == nil {
		return nil, 0, "", "", errors.New("unexpected OpenContactPhoto call")
	}
	return f.openPhoto(ctx, actor, id)
}

func (f *fakeContactManager) SetContactPhoto(
	ctx context.Context, actor, id uuid.UUID, processed photo.Processed,
) (domain.Contact, error) {
	if f.setPhoto == nil {
		return domain.Contact{}, errors.New("unexpected SetContactPhoto call")
	}
	return f.setPhoto(ctx, actor, id, processed)
}

func (f *fakeContactManager) DeleteContactPhoto(
	ctx context.Context, actor, id uuid.UUID,
) (domain.Contact, error) {
	if f.deletePhoto == nil {
		return domain.Contact{}, errors.New("unexpected DeleteContactPhoto call")
	}
	return f.deletePhoto(ctx, actor, id)
}

// testFirstName is the given name the wire fixtures assert on.
const testFirstName = "Пётр"

// storedContact is a fully populated stored card the fakes hand back.
func storedContact() domain.Contact {
	id := uuid.Must(uuid.NewV7())
	propertyID := uuid.Must(uuid.NewV7())
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	return domain.Contact{
		ID:                id,
		OwnerID:           uuid.Must(uuid.NewV7()),
		PropertyID:        &propertyID,
		FirstName:         testFirstName,
		LastName:          "Иванов",
		Patronymic:        "Петрович",
		Role:              "сантехник",
		Phone:             "+79160000001",
		Email:             "petr@example.ru",
		MessengerName:     "Telegram",
		MessengerUsername: "@petr",
		Note:              "Код домофона 1234",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// contactRequest builds an authenticated request against the contacts
// endpoints with an optional JSON body.
func contactRequest(t *testing.T, method string, userID uuid.UUID, body string) *http.Request {
	t.Helper()
	reader := strings.NewReader(body)
	req := httptest.NewRequestWithContext(
		httpsupport.WithUserID(t.Context(), userID),
		method, "/contacts", reader,
	)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func TestContactHandlers_RequireAuth(t *testing.T) {
	t.Parallel()
	h := NewContactHandlers(&fakeContactManager{}, nil)
	contactID := uuid.Must(uuid.NewV7())

	cases := []struct {
		name string
		call func(w http.ResponseWriter, r *http.Request)
	}{
		{"create", func(w http.ResponseWriter, r *http.Request) {
			h.CreateContact(w, r)
		}},
		{"list", func(w http.ResponseWriter, r *http.Request) {
			h.ListContacts(w, r, openapi.ListContactsParams{})
		}},
		{"get", func(w http.ResponseWriter, r *http.Request) {
			h.GetContact(w, r, contactID)
		}},
		{"update", func(w http.ResponseWriter, r *http.Request) {
			h.UpdateContact(w, r, contactID)
		}},
		{"delete", func(w http.ResponseWriter, r *http.Request) {
			h.DeleteContact(w, r, contactID)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			tc.call(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", w.Code)
			}
		})
	}
}

func TestCreateContact(t *testing.T) {
	t.Parallel()

	body := `{
		"propertyId": "0198c0de-0000-7000-8000-000000000001",
		"firstName": "` + testFirstName + `",
		"lastName": "Иванов",
		"role": "сантехник",
		"phone": "8 (916) 000-00-01"
	}`

	var gotCmd application.CreateContactCommand
	var gotActor uuid.UUID
	returned := storedContact()
	svc := &fakeContactManager{
		create: func(_ context.Context, actor uuid.UUID, cmd application.CreateContactCommand) (domain.Contact, error) {
			gotActor = actor
			gotCmd = cmd
			return returned, nil
		},
	}
	h := NewContactHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	w := httptest.NewRecorder()
	h.CreateContact(w, contactRequest(t, http.MethodPost, actor, body))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body.String())
	}
	if gotActor != actor {
		t.Fatalf("actor = %s, want %s", gotActor, actor)
	}
	wantProperty := uuid.MustParse("0198c0de-0000-7000-8000-000000000001")
	if gotCmd.PropertyID == nil || *gotCmd.PropertyID != wantProperty {
		t.Fatalf("command propertyID = %v, want %s", gotCmd.PropertyID, wantProperty)
	}
	if gotCmd.FirstName != testFirstName || gotCmd.LastName != "Иванов" || gotCmd.Role != "сантехник" {
		t.Fatalf("command names = %+v", gotCmd)
	}
	// The phone travels as-is: normalization is the service's contract.
	if gotCmd.Phone != "8 (916) 000-00-01" {
		t.Fatalf("command phone = %q", gotCmd.Phone)
	}

	var resp openapi.ContactResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.FirstName != testFirstName || resp.Phone != "+79160000001" {
		t.Fatalf("response names/phone = %+v", resp)
	}
	// The response maps the stored card the service returned, property
	// binding included.
	if resp.PropertyId == nil || *resp.PropertyId != *returned.PropertyID {
		t.Fatalf("response propertyId = %v, want %s", resp.PropertyId, *returned.PropertyID)
	}
}

func TestCreateContact_WithoutProperty(t *testing.T) {
	t.Parallel()

	var gotCmd application.CreateContactCommand
	svc := &fakeContactManager{
		create: func(_ context.Context, _ uuid.UUID, cmd application.CreateContactCommand) (domain.Contact, error) {
			gotCmd = cmd
			created := storedContact()
			created.PropertyID = nil
			return created, nil
		},
	}
	h := NewContactHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	w := httptest.NewRecorder()
	h.CreateContact(w, contactRequest(t, http.MethodPost, actor, `{"firstName": "Мария"}`))

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body.String())
	}
	if gotCmd.PropertyID != nil {
		t.Fatalf("command propertyID = %v, want nil", gotCmd.PropertyID)
	}
	// Optional fields the body omits arrive as empty strings.
	if gotCmd.LastName != "" || gotCmd.Note != "" {
		t.Fatalf("command optionals = %+v", gotCmd)
	}
	var resp openapi.ContactResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PropertyId != nil {
		t.Fatalf("response propertyId = %v, want null", resp.PropertyId)
	}
}

func TestCreateContact_RejectsMalformedBody(t *testing.T) {
	t.Parallel()
	h := NewContactHandlers(&fakeContactManager{}, nil)
	actor := uuid.Must(uuid.NewV7())

	for _, body := range []string{`{`, `{"firstName": 5}`, `{"unknown": true}`} {
		w := httptest.NewRecorder()
		h.CreateContact(w, contactRequest(t, http.MethodPost, actor, body))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400", body, w.Code)
		}
	}
}

func TestCreateContact_InvalidInput(t *testing.T) {
	t.Parallel()
	// The service rejects an empty first name and a malformed phone with the
	// invalid-input sentinel; the transport maps it onto the shared 400
	// detail.
	svc := &fakeContactManager{
		create: func(_ context.Context, _ uuid.UUID, _ application.CreateContactCommand) (domain.Contact, error) {
			return domain.Contact{}, application.ErrInvalidInput
		},
	}
	h := NewContactHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	w := httptest.NewRecorder()
	h.CreateContact(w, contactRequest(t, http.MethodPost, actor, `{"firstName": "  ", "phone": "123"}`))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
	var problem openapi.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Detail == nil || *problem.Detail != "Некорректные данные контакта" {
		t.Fatalf("detail = %v, want the shared contact detail", problem.Detail)
	}
}

func TestListContacts(t *testing.T) {
	t.Parallel()

	propertyID := uuid.Must(uuid.NewV7())
	var gotQueries []application.ListQuery
	svc := &fakeContactManager{
		list: func(_ context.Context, _ uuid.UUID, q application.ListQuery) (application.ContactBookPage, error) {
			gotQueries = append(gotQueries, q)
			return application.ContactBookPage{}, nil
		},
	}
	h := NewContactHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	// The whole book: no property filter, search on, sort defaults.
	search := "петр"
	w := httptest.NewRecorder()
	h.ListContacts(w, contactRequest(t, http.MethodGet, actor, ""),
		openapi.ListContactsParams{Search: &search})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	// A property slice: the filter resolves to the property scope.
	w = httptest.NewRecorder()
	h.ListContacts(w, contactRequest(t, http.MethodGet, actor, ""),
		openapi.ListContactsParams{PropertyId: &propertyID})

	if len(gotQueries) != 2 {
		t.Fatalf("queries = %d, want 2", len(gotQueries))
	}
	if gotQueries[0].Scope != application.ListScopeAll || gotQueries[0].Search != "петр" ||
		gotQueries[0].Sort != application.ListSortName || gotQueries[0].Order != application.ListOrderAsc {
		t.Fatalf("book query = %+v", gotQueries[0])
	}
	if gotQueries[1].Scope != application.ListScopeProperty || gotQueries[1].PropertyID != propertyID {
		t.Fatalf("property query = %+v", gotQueries[1])
	}
}

// TestListContactsSortAndProjection pins the sort params on the wire and the
// list projection: sort/order travel into the query, the bound property's
// display name rides along in the response items.
func TestListContactsSortAndProjection(t *testing.T) {
	t.Parallel()

	var gotQuery application.ListQuery
	stored := storedContact()
	propName := "Моя квартира"
	svc := &fakeContactManager{
		list: func(_ context.Context, _ uuid.UUID, q application.ListQuery) (application.ContactBookPage, error) {
			gotQuery = q
			return application.ContactBookPage{
				Items: []application.ListedContact{{Contact: stored, PropertyName: propName}},
			}, nil
		},
	}
	h := NewContactHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	sortProp := openapi.ListContactsParamsSort("property")
	orderDesc := openapi.ListContactsParamsOrder("desc")
	w := httptest.NewRecorder()
	h.ListContacts(w, contactRequest(t, http.MethodGet, actor, ""),
		openapi.ListContactsParams{Sort: &sortProp, Order: &orderDesc})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if gotQuery.Sort != application.ListSortProperty || gotQuery.Order != application.ListOrderDesc {
		t.Fatalf("query = %+v", gotQuery)
	}

	var resp openapi.ContactsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].FirstName != testFirstName {
		t.Fatalf("response items = %+v", resp.Items)
	}
	if resp.Items[0].PropertyName == nil || *resp.Items[0].PropertyName != propName {
		t.Fatalf("response propertyName = %v, want %q", resp.Items[0].PropertyName, propName)
	}
}

// TestListContactsPagingForwarded pins the page vocabulary on the wire
// (ticket #600): limit and cursor travel into the query, the service's
// continuation rides back as the response's nextCursor, and a malformed
// cursor is the contract's 400.
func TestListContactsPagingForwarded(t *testing.T) {
	t.Parallel()

	nextCursor := application.EncodeContactCursor(application.ContactCursorKey{
		Name: "Пётр Иванов",
		ID:   uuid.Must(uuid.NewV7()),
	}, application.ListSortName, application.ListOrderAsc)
	badCursor := "не курсор"
	var gotQuery application.ListQuery
	svc := &fakeContactManager{
		list: func(_ context.Context, _ uuid.UUID, q application.ListQuery) (application.ContactBookPage, error) {
			gotQuery = q
			if q.Cursor == badCursor {
				return application.ContactBookPage{}, application.ErrInvalidInput
			}
			return application.ContactBookPage{NextCursor: nextCursor}, nil
		},
	}
	h := NewContactHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	limit := 25
	cursor := "cursor-blob"
	w := httptest.NewRecorder()
	h.ListContacts(w, contactRequest(t, http.MethodGet, actor, ""),
		openapi.ListContactsParams{Limit: &limit, Cursor: &cursor})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if gotQuery.Limit != 25 || gotQuery.Cursor != "cursor-blob" {
		t.Fatalf("query = %+v", gotQuery)
	}

	var resp openapi.ContactsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.NextCursor == nil || *resp.NextCursor != nextCursor {
		t.Fatalf("response nextCursor = %v, want %q", resp.NextCursor, nextCursor)
	}

	// The malformed cursor never reaches the store: the application's
	// invalid input folds into the contract's 400.
	w = httptest.NewRecorder()
	h.ListContacts(w, contactRequest(t, http.MethodGet, actor, ""),
		openapi.ListContactsParams{Limit: &limit, Cursor: &badCursor})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
}

func TestGetContact_NotFound(t *testing.T) {
	t.Parallel()
	svc := &fakeContactManager{
		get: func(_ context.Context, _, _ uuid.UUID) (domain.Contact, error) {
			return domain.Contact{}, application.ErrNotFound
		},
	}
	h := NewContactHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	w := httptest.NewRecorder()
	h.GetContact(w, contactRequest(t, http.MethodGet, actor, ""), uuid.Must(uuid.NewV7()))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", w.Code, w.Body.String())
	}
}

// GetContactPhoto answers 404 (privacy-preserving, like properties and
// identity) when the card has no photo — not the opaque 500 (#1229 found
// it: the edit screen's parallel invalidations briefly render the img with
// a fresh buster over the deleted photo).
func TestGetContactPhoto_NoPhoto(t *testing.T) {
	t.Parallel()
	svc := &fakeContactManager{
		photoDescriptor: func(_ context.Context, _, _ uuid.UUID) (string, string, error) {
			return "", "", application.ErrPhotoNotFound
		},
	}
	h := NewContactHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	w := httptest.NewRecorder()
	h.GetContactPhoto(w, contactRequest(t, http.MethodGet, actor, ""), uuid.Must(uuid.NewV7()))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", w.Code, w.Body.String())
	}
}

func TestUpdateContact_PropertyTriState(t *testing.T) {
	t.Parallel()

	target := uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())

	cases := []struct {
		name string
		body string
		want *application.PropertyIDUpdate // nil: the command field must stay nil
	}{
		{
			name: "omitted keeps the binding",
			body: `{"firstName": "Пётр"}`,
			want: nil,
		},
		{
			name: "null clears the binding",
			body: `{"propertyId": null}`,
			want: &application.PropertyIDUpdate{Value: nil},
		},
		{
			name: "uuid moves the card",
			body: `{"propertyId": "` + other.String() + `"}`,
			want: &application.PropertyIDUpdate{Value: &other},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var gotCmd application.UpdateContactCommand
			svc := &fakeContactManager{
				update: func(_ context.Context, _, _ uuid.UUID, cmd application.UpdateContactCommand) (domain.Contact, error) {
					gotCmd = cmd
					return storedContact(), nil
				},
			}
			h := NewContactHandlers(svc, nil)
			actor := uuid.Must(uuid.NewV7())

			w := httptest.NewRecorder()
			h.UpdateContact(w, contactRequest(t, http.MethodPatch, actor, tc.body), target)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
			}
			if tc.want == nil {
				if gotCmd.PropertyID != nil {
					t.Fatalf("command propertyID = %+v, want nil", gotCmd.PropertyID)
				}
				return
			}
			if gotCmd.PropertyID == nil {
				t.Fatalf("command propertyID = nil, want %+v", tc.want)
			}
			if tc.want.Value == nil {
				if gotCmd.PropertyID.Value != nil {
					t.Fatalf("propertyID.Value = %v, want nil", *gotCmd.PropertyID.Value)
				}
				return
			}
			if gotCmd.PropertyID.Value == nil || *gotCmd.PropertyID.Value != *tc.want.Value {
				t.Fatalf("propertyID.Value = %v, want %s", gotCmd.PropertyID.Value, tc.want.Value)
			}
		})
	}
}

func TestUpdateContact_FoldsOptionalFields(t *testing.T) {
	t.Parallel()

	var gotCmd application.UpdateContactCommand
	svc := &fakeContactManager{
		update: func(_ context.Context, _, _ uuid.UUID, cmd application.UpdateContactCommand) (domain.Contact, error) {
			gotCmd = cmd
			return storedContact(), nil
		},
	}
	h := NewContactHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())

	body := `{"note": "", "phone": "+79160000002"}`
	w := httptest.NewRecorder()
	h.UpdateContact(w, contactRequest(t, http.MethodPatch, actor, body), uuid.Must(uuid.NewV7()))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if gotCmd.Note == nil || *gotCmd.Note != "" {
		t.Fatalf("note = %v, want the clearing empty string", gotCmd.Note)
	}
	if gotCmd.Phone == nil || *gotCmd.Phone != "+79160000002" {
		t.Fatalf("phone = %v", gotCmd.Phone)
	}
	if gotCmd.FirstName != nil {
		t.Fatalf("firstName = %v, want nil", gotCmd.FirstName)
	}
}

func TestUpdateContact_RejectsMalformedBody(t *testing.T) {
	t.Parallel()
	h := NewContactHandlers(&fakeContactManager{}, nil)
	actor := uuid.Must(uuid.NewV7())

	// A malformed propertyId uuid fails the strict contract decode; a broken
	// JSON fails the decode itself; an unknown field is rejected.
	for _, body := range []string{
		`{"propertyId": "not-a-uuid"}`,
		`{`,
		`{"nope": 1}`,
	} {
		w := httptest.NewRecorder()
		h.UpdateContact(w, contactRequest(t, http.MethodPatch, actor, body), uuid.Must(uuid.NewV7()))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400", body, w.Code)
		}
	}
}

func TestDeleteContact(t *testing.T) {
	t.Parallel()

	var gotID uuid.UUID
	svc := &fakeContactManager{
		del: func(_ context.Context, _, id uuid.UUID) error {
			gotID = id
			return nil
		},
	}
	h := NewContactHandlers(svc, nil)
	actor := uuid.Must(uuid.NewV7())
	contactID := uuid.Must(uuid.NewV7())

	w := httptest.NewRecorder()
	h.DeleteContact(w, contactRequest(t, http.MethodDelete, actor, ""), contactID)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", w.Code, w.Body.String())
	}
	if gotID != contactID {
		t.Fatalf("deleted id = %s, want %s", gotID, contactID)
	}
}

func TestContactHandlers_ErrorMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"forbidden", application.ErrForbidden, http.StatusForbidden},
		{"not found", application.ErrNotFound, http.StatusNotFound},
		{"unknown", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := &fakeContactManager{
				get: func(_ context.Context, _, _ uuid.UUID) (domain.Contact, error) {
					return domain.Contact{}, tc.err
				},
			}
			h := NewContactHandlers(svc, nil)
			actor := uuid.Must(uuid.NewV7())

			w := httptest.NewRecorder()
			h.GetContact(w, contactRequest(t, http.MethodGet, actor, ""), uuid.Must(uuid.NewV7()))

			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
