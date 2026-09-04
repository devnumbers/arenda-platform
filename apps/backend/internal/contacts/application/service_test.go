package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The unit seam of the contacts use cases: fake stores over an in-memory map,
// a fake UoW and a capturing audit recorder. The role matrix runs on stub
// policies; the real membership policy and PostgreSQL arrive in the
// integration tests.

// contactFirstName is the shared name literal of the fixtures (goconst: one
// home).
const contactFirstName = "Пётр"

// plumberRole is the shared role literal of the fixtures.
const plumberRole = "сантехник"

// updatedName is the shared rename literal of the update fixtures.
const updatedName = "Борис"

// FakeTx is the UoW's transaction token; the fakes' WithTx just returns
// themselves, so one map backs every binding.
type FakeTx struct{}

func (FakeTx) Commit(context.Context) error   { return nil }
func (FakeTx) Rollback(context.Context) error { return nil }

type fakeUoW struct{}

func (fakeUoW) Do(_ context.Context, work func(tx transaction.Tx) error) error {
	return work(FakeTx{})
}

// fakeContactStore is the in-memory ContactStore double: GetByID/List read the
// map, Create/Update/Delete mutate it exactly like the SQL contract — Update
// and Delete key on (id, owner_id) and miss as ErrNotFound.
type fakeContactStore struct {
	byID map[uuid.UUID]domain.Contact

	created []domain.Contact
	updates []domain.Contact
	deleted []uuid.UUID
	// ListFn overrides List when a test needs to observe the actor scope and
	// the query the service forwarded; unset, List filters the map by the
	// card's owner.
	listFn func(actorID uuid.UUID, q ListQuery) ([]ListedContact, error)
}

func newFakeContactStore(contacts ...domain.Contact) *fakeContactStore {
	s := &fakeContactStore{byID: make(map[uuid.UUID]domain.Contact, len(contacts))}
	for _, c := range contacts {
		s.byID[c.ID] = c
	}
	return s
}

func (s *fakeContactStore) GetByID(_ context.Context, id uuid.UUID) (domain.Contact, error) {
	c, ok := s.byID[id]
	if !ok {
		return domain.Contact{}, ErrNotFound
	}
	return c, nil
}

func (s *fakeContactStore) List(ctx context.Context, actorID uuid.UUID, q ListQuery) ([]ListedContact, error) {
	if s.listFn != nil {
		return s.listFn(actorID, q)
	}
	out := []ListedContact{}
	for _, c := range s.byID {
		if c.OwnerID == actorID {
			out = append(out, ListedContact{Contact: c})
		}
	}
	return out, nil
}

func (s *fakeContactStore) Create(_ context.Context, c domain.Contact) (domain.Contact, error) {
	s.byID[c.ID] = c
	s.created = append(s.created, c)
	return c, nil
}

func (s *fakeContactStore) Update(_ context.Context, c domain.Contact) (domain.Contact, error) {
	stored, ok := s.byID[c.ID]
	if !ok || stored.OwnerID != c.OwnerID {
		return domain.Contact{}, ErrNotFound
	}
	s.byID[c.ID] = c
	s.updates = append(s.updates, c)
	return c, nil
}

func (s *fakeContactStore) Delete(_ context.Context, id, ownerID uuid.UUID) error {
	c, ok := s.byID[id]
	if !ok || c.OwnerID != ownerID {
		return ErrNotFound
	}
	delete(s.byID, id)
	s.deleted = append(s.deleted, id)
	return nil
}

func (s *fakeContactStore) WithTx(transaction.Tx) (ContactStore, error) { return s, nil }

// fakePropertyStore resolves the canned owner of each property.
type fakePropertyStore struct{ owners map[uuid.UUID]uuid.UUID }

func (s fakePropertyStore) Get(_ context.Context, propertyID uuid.UUID) (PropertyRef, error) {
	owner, ok := s.owners[propertyID]
	if !ok {
		return PropertyRef{}, ErrNotFound
	}
	return PropertyRef{OwnerID: owner}, nil
}

// fakeRecorder captures audit entries written inside the transaction.
type fakeRecorder struct{ entries []auditdomain.Entry }

func (r *fakeRecorder) Record(_ context.Context, e auditdomain.Entry) error {
	r.entries = append(r.entries, e)
	return nil
}

func (r *fakeRecorder) WithTx(transaction.Tx) auditapp.Recorder { return r }

// stubPolicy resolves every (actor, property) pair to one configured role.
type stubPolicy struct{ role sharedpolicy.Role }

func (p stubPolicy) Role(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleNone, nil
}

func (p stubPolicy) RoleForProperty(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return p.role, nil
}

// matrixPolicy resolves roles per (actor, property) with RoleNone everywhere
// else — the shared-access matrix double for the cross-property cases.
type matrixPolicy struct {
	roles map[uuid.UUID]map[uuid.UUID]sharedpolicy.Role
}

func (p matrixPolicy) Role(context.Context, uuid.UUID, uuid.UUID) (sharedpolicy.Role, error) {
	return sharedpolicy.RoleNone, nil
}

func (p matrixPolicy) RoleForProperty(_ context.Context, actor, propertyID uuid.UUID) (sharedpolicy.Role, error) {
	if role, ok := p.roles[actor][propertyID]; ok {
		return role, nil
	}
	return sharedpolicy.RoleNone, nil
}

// serviceHarness bundles a service with its fakes for assertions.
type serviceHarness struct {
	svc      *ContactService
	contacts *fakeContactStore
	audit    *fakeRecorder

	owner     uuid.UUID
	member    uuid.UUID
	viewer    uuid.UUID
	stranger  uuid.UUID
	property  uuid.UUID
	otherProp uuid.UUID
}

// newServiceHarness wires the service over the fakes with one stub role for
// every property gate (the single-role matrix cases).
func newServiceHarness(t *testing.T, role sharedpolicy.Role) *serviceHarness {
	t.Helper()
	return newServiceHarnessWithPolicy(t, func(*serviceHarness) sharedpolicy.Policy {
		return stubPolicy{role: role}
	})
}

// newServiceHarnessWithPolicy lets the test choose the policy from the
// harness's fresh ids — the per-(actor, property) matrix cases.
func newServiceHarnessWithPolicy(
	t *testing.T, build func(h *serviceHarness) sharedpolicy.Policy,
) *serviceHarness {
	t.Helper()
	owner := uuid.Must(uuid.NewV7())
	audit := &fakeRecorder{}
	h := &serviceHarness{
		contacts:  newFakeContactStore(),
		audit:     audit,
		owner:     owner,
		member:    uuid.Must(uuid.NewV7()),
		viewer:    uuid.Must(uuid.NewV7()),
		stranger:  uuid.Must(uuid.NewV7()),
		property:  uuid.Must(uuid.NewV7()),
		otherProp: uuid.Must(uuid.NewV7()),
	}
	factory := NewTxStoreFactory(
		h.contacts,
		fakePropertyStore{owners: map[uuid.UUID]uuid.UUID{h.property: owner, h.otherProp: owner}},
		audit,
		fakeUoW{},
	)
	h.svc = NewContactService(factory, build(h))
	return h
}

// createCmd is the canonical valid create fixture: a plumber with every
// optional field set.
func createCmd(propertyID *uuid.UUID) CreateContactCommand {
	return CreateContactCommand{
		PropertyID:        propertyID,
		FirstName:         "  " + contactFirstName + "  ",
		LastName:          "Иванов",
		Role:              plumberRole,
		Phone:             "89161234567",
		Email:             "plumber@example.ru",
		MessengerName:     "Telegram",
		MessengerUsername: "@plumber",
		Note:              "Код домофона 1234",
	}
}

// seedBoundContact inserts a property-bound card of the owner.
func (h *serviceHarness) seedBoundContact(propertyID uuid.UUID) uuid.UUID {
	id := uuid.Must(uuid.NewV7())
	prop := propertyID
	h.contacts.byID[id] = domain.Contact{
		ID: id, OwnerID: h.owner, PropertyID: &prop, FirstName: contactFirstName,
	}
	return id
}

// seedUnboundContact inserts an unbound card of the owner.
func (h *serviceHarness) seedUnboundContact() uuid.UUID {
	id := uuid.Must(uuid.NewV7())
	h.contacts.byID[id] = domain.Contact{ID: id, OwnerID: h.owner, FirstName: contactFirstName}
	return id
}

func TestCreateContactOwnBook(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	created, err := h.svc.CreateContact(t.Context(), h.owner, createCmd(nil))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.OwnerID != h.owner {
		t.Fatalf("owner: want %s, got %s", h.owner, created.OwnerID)
	}
	if created.PropertyID != nil {
		t.Fatalf("property: want nil, got %s", *created.PropertyID)
	}
	if created.FirstName != contactFirstName {
		t.Fatalf("first name: want trimmed, got %q", created.FirstName)
	}
	if created.Phone != "+79161234567" {
		t.Fatalf("phone: want normalized, got %q", created.Phone)
	}
	if created.ID == uuid.Nil {
		t.Fatal("id: want minted")
	}
}

func TestCreateContactOnProperty(t *testing.T) {
	t.Parallel()

	t.Run("member lands the card on the property owner's book", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleFullAccess)
		prop := h.property
		created, err := h.svc.CreateContact(t.Context(), h.member, createCmd(&prop))
		if err != nil {
			t.Fatalf("create as member: %v", err)
		}
		if created.OwnerID != h.owner {
			t.Fatalf("owner: want %s, got %s", h.owner, created.OwnerID)
		}
		if created.PropertyID == nil || *created.PropertyID != h.property {
			t.Fatalf("property: want %s, got %v", h.property, created.PropertyID)
		}
	})

	t.Run("viewer is forbidden on the property", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleViewer)
		prop := h.property
		if _, err := h.svc.CreateContact(t.Context(), h.viewer, createCmd(&prop)); !errors.Is(err, ErrForbidden) {
			t.Fatalf("want ErrForbidden, got %v", err)
		}
	})

	t.Run("stranger gets the privacy 404", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleNone)
		prop := h.property
		if _, err := h.svc.CreateContact(t.Context(), h.stranger, createCmd(&prop)); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("unknown property is the privacy 404", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleOwner)
		unknown := uuid.Must(uuid.NewV7())
		if _, err := h.svc.CreateContact(t.Context(), h.owner, createCmd(&unknown)); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

func TestCreateContactInvalidDrafts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		mut  func(*CreateContactCommand)
	}{
		{"empty first name", func(c *CreateContactCommand) { c.FirstName = "   " }},
		{"bad phone", func(c *CreateContactCommand) { c.Phone = "12345" }},
		{"bad email", func(c *CreateContactCommand) { c.Email = "no-at-sign" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newServiceHarness(t, sharedpolicy.RoleOwner)
			cmd := createCmd(nil)
			tc.mut(&cmd)
			if _, err := h.svc.CreateContact(t.Context(), h.owner, cmd); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("want ErrInvalidInput, got %v", err)
			}
			if len(h.contacts.created) != 0 {
				t.Fatalf("store saw %d creates, want 0", len(h.contacts.created))
			}
		})
	}
}

func TestCreateContactAudits(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t, sharedpolicy.RoleFullAccess)
	prop := h.property
	created, err := h.svc.CreateContact(t.Context(), h.member, createCmd(&prop))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(h.audit.entries) != 1 {
		t.Fatalf("audit entries: want 1, got %d", len(h.audit.entries))
	}
	e := h.audit.entries[0]
	if e.Action != auditdomain.ActionContactCreated || e.EntityType != auditdomain.EntityContact {
		t.Fatalf("audit action/entity: want contact.created/contact, got %s/%s", e.Action, e.EntityType)
	}
	if e.EntityID == nil || *e.EntityID != created.ID {
		t.Fatalf("audit entity id: want %s, got %v", created.ID, e.EntityID)
	}
	if e.Context["property_id"] != h.property {
		t.Fatalf("audit ctx property_id: want %s, got %v", h.property, e.Context["property_id"])
	}
	for key, v := range e.Context {
		if s, ok := v.(string); ok && (s == created.FirstName || s == created.Phone || s == created.Email) {
			t.Fatalf("audit ctx %s carries PII: %v", key, v)
		}
	}
}

func TestGetContact(t *testing.T) {
	t.Parallel()

	t.Run("viewer reads a property-bound contact", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleViewer)
		id := h.seedBoundContact(h.property)
		if _, err := h.svc.GetContact(t.Context(), h.viewer, id); err != nil {
			t.Fatalf("get as viewer: %v", err)
		}
	})

	t.Run("stranger on a bound contact is the privacy 404", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleNone)
		id := h.seedBoundContact(h.property)
		if _, err := h.svc.GetContact(t.Context(), h.stranger, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("unbound contact is the owner's alone", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleOwner)
		id := h.seedUnboundContact()
		if _, err := h.svc.GetContact(t.Context(), h.owner, id); err != nil {
			t.Fatalf("get as owner: %v", err)
		}
		other := newServiceHarness(t, sharedpolicy.RoleOwner)
		if _, err := other.svc.GetContact(t.Context(), other.owner, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("another owner's book: want ErrNotFound, got %v", err)
		}
	})

	t.Run("missing contact is not found", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleOwner)
		if _, err := h.svc.GetContact(t.Context(), h.owner, uuid.Must(uuid.NewV7())); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

func TestListContacts(t *testing.T) {
	t.Parallel()

	t.Run("property scope forwards the actor and gates shared members in", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleViewer)
		var listedActor uuid.UUID
		h.contacts.listFn = func(actor uuid.UUID, _ ListQuery) ([]ListedContact, error) {
			listedActor = actor
			return nil, nil
		}
		q := ListQuery{Scope: ListScopeProperty, PropertyID: h.property}
		if _, err := h.svc.ListContacts(t.Context(), h.member, q); err != nil {
			t.Fatalf("list as member: %v", err)
		}
		if listedActor != h.member {
			t.Fatalf("store actor scope: want %s, got %s", h.member, listedActor)
		}
	})

	t.Run("book scopes forward the actor untouched", func(t *testing.T) {
		t.Parallel()
		for name, scope := range map[string]ListScope{
			"all":              ListScopeAll,
			"without property": ListScopeWithoutProperty,
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				h := newServiceHarness(t, sharedpolicy.RoleOwner)
				var listedActor uuid.UUID
				h.contacts.listFn = func(actor uuid.UUID, _ ListQuery) ([]ListedContact, error) {
					listedActor = actor
					return nil, nil
				}
				if _, err := h.svc.ListContacts(t.Context(), h.owner, ListQuery{Scope: scope}); err != nil {
					t.Fatalf("list: %v", err)
				}
				if listedActor != h.owner {
					t.Fatalf("store actor scope: want %s, got %s", h.owner, listedActor)
				}
			})
		}
	})

	t.Run("property scope gates the stranger out", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleNone)
		q := ListQuery{Scope: ListScopeProperty, PropertyID: h.property}
		if _, err := h.svc.ListContacts(t.Context(), h.stranger, q); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("unknown scope is invalid input", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleOwner)
		if _, err := h.svc.ListContacts(t.Context(), h.owner, ListQuery{Scope: "bogus"}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
	})
}

// TestListContactsSortOrderForwarded pins the sort validation: known keys
// travel to the store as-is, the empty values mean the defaults, unknown
// keys are the invalid input.
func TestListContactsSortOrderForwarded(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		sort    ListSort
		order   ListOrder
		wantErr bool
	}{
		{"defaults", "", "", false},
		{"name ascending", ListSortName, ListOrderAsc, false},
		{"property descending", ListSortProperty, ListOrderDesc, false},
		{"unknown sort", "sideways", ListOrderAsc, true},
		{"unknown order", ListSortName, "upside", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newServiceHarness(t, sharedpolicy.RoleOwner)
			var gotQuery ListQuery
			h.contacts.listFn = func(_ uuid.UUID, q ListQuery) ([]ListedContact, error) {
				gotQuery = q
				return nil, nil
			}
			_, err := h.svc.ListContacts(t.Context(), h.owner, ListQuery{
				Scope: ListScopeAll, Sort: tc.sort, Order: tc.order,
			})
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("want ErrInvalidInput, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if gotQuery.Sort != tc.sort || gotQuery.Order != tc.order {
				t.Fatalf("forwarded sort/order: want %q/%q, got %q/%q", tc.sort, tc.order, gotQuery.Sort, gotQuery.Order)
			}
		})
	}
}

func TestUpdateContactFoldAndNormalize(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	id := h.seedBoundContact(h.property)
	stored := h.contacts.byID[id]
	stored.Role = plumberRole
	stored.Phone = "+79161234567"
	h.contacts.byID[id] = stored
	newPhone := "89160000000"
	newName := updatedName
	updated, err := h.svc.UpdateContact(t.Context(), h.owner, id, UpdateContactCommand{
		FirstName: &newName,
		Phone:     &newPhone,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.FirstName != newName {
		t.Fatalf("first name: want %q, got %q", newName, updated.FirstName)
	}
	if updated.Phone != "+79160000000" {
		t.Fatalf("phone: want normalized, got %q", updated.Phone)
	}
	if updated.Role != plumberRole {
		t.Fatalf("role: want kept, got %q", updated.Role)
	}
	if len(h.audit.entries) != 1 || h.audit.entries[0].Action != auditdomain.ActionContactUpdated {
		t.Fatalf("audit: want one contact.updated, got %+v", h.audit.entries)
	}
}

func TestUpdateContactForbiddenForViewer(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t, sharedpolicy.RoleViewer)
	id := h.seedBoundContact(h.property)
	newName := updatedName
	if _, err := h.svc.UpdateContact(t.Context(), h.viewer, id, UpdateContactCommand{
		FirstName: &newName,
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestUpdateContactUnboundIsOwnerAlone(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	id := h.seedUnboundContact()
	newName := updatedName
	if _, err := h.svc.UpdateContact(t.Context(), h.owner, id, UpdateContactCommand{
		FirstName: &newName,
	}); err != nil {
		t.Fatalf("owner update: %v", err)
	}
	other := newServiceHarness(t, sharedpolicy.RoleOwner)
	if _, err := other.svc.UpdateContact(t.Context(), other.owner, id, UpdateContactCommand{
		FirstName: &newName,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another owner's book: want ErrNotFound, got %v", err)
	}
}

func TestUpdateContactClearProperty(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t, sharedpolicy.RoleFullAccess)
	id := h.seedBoundContact(h.property)
	updated, err := h.svc.UpdateContact(t.Context(), h.member, id, UpdateContactCommand{
		PropertyID: &PropertyIDUpdate{Value: nil},
	})
	if err != nil {
		t.Fatalf("clear property: %v", err)
	}
	if updated.PropertyID != nil {
		t.Fatalf("property: want nil, got %s", *updated.PropertyID)
	}
}

func TestUpdateContactMoveForbiddenForViewerOnTarget(t *testing.T) {
	t.Parallel()

	// The member holds Full Access on the source property but only a viewer's
	// role on the target: the move must fail forbidden.
	h := newServiceHarnessWithPolicy(t, func(h *serviceHarness) sharedpolicy.Policy {
		return matrixPolicy{roles: map[uuid.UUID]map[uuid.UUID]sharedpolicy.Role{
			h.member: {h.property: sharedpolicy.RoleFullAccess, h.otherProp: sharedpolicy.RoleViewer},
		}}
	})
	id := h.seedBoundContact(h.property)
	if _, err := h.svc.UpdateContact(t.Context(), h.member, id, UpdateContactCommand{
		PropertyID: &PropertyIDUpdate{Value: &h.otherProp},
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestUpdateContactMoveLands(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t, sharedpolicy.RoleFullAccess)
	id := h.seedBoundContact(h.property)
	updated, err := h.svc.UpdateContact(t.Context(), h.member, id, UpdateContactCommand{
		PropertyID: &PropertyIDUpdate{Value: &h.otherProp},
	})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if updated.PropertyID == nil || *updated.PropertyID != h.otherProp {
		t.Fatalf("property: want %s, got %v", h.otherProp, updated.PropertyID)
	}
	if updated.OwnerID != h.owner {
		t.Fatalf("owner: want unchanged %s, got %s", h.owner, updated.OwnerID)
	}
}

func TestUpdateContactInvalidDraft(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t, sharedpolicy.RoleOwner)
	id := h.seedUnboundContact()
	blank := "   "
	if _, err := h.svc.UpdateContact(t.Context(), h.owner, id, UpdateContactCommand{
		FirstName: &blank,
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestDeleteContact(t *testing.T) {
	t.Parallel()

	t.Run("member deletes the owner's bound contact", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleFullAccess)
		id := h.seedBoundContact(h.property)
		if err := h.svc.DeleteContact(t.Context(), h.member, id); err != nil {
			t.Fatalf("delete as member: %v", err)
		}
		if len(h.audit.entries) != 1 || h.audit.entries[0].Action != auditdomain.ActionContactDeleted {
			t.Fatalf("audit: want one contact.deleted, got %+v", h.audit.entries)
		}
	})

	t.Run("viewer is forbidden", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleViewer)
		id := h.seedBoundContact(h.property)
		if err := h.svc.DeleteContact(t.Context(), h.viewer, id); !errors.Is(err, ErrForbidden) {
			t.Fatalf("want ErrForbidden, got %v", err)
		}
	})

	t.Run("unbound contact is the owner's alone", func(t *testing.T) {
		t.Parallel()
		h := newServiceHarness(t, sharedpolicy.RoleOwner)
		id := h.seedUnboundContact()
		other := newServiceHarness(t, sharedpolicy.RoleOwner)
		if err := other.svc.DeleteContact(t.Context(), other.owner, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("another owner's book: want ErrNotFound, got %v", err)
		}
	})
}
