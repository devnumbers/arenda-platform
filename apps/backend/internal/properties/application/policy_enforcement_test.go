package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// This file is the role-matrix cover for the shared-access enforcement on
// property contacts (Property Sharing follow-up): every use case is exercised
// as owner, full-access member, viewer, outsider (none) and suspended member.
// Reads map any non-view role to ErrNotFound; writes map none/suspended to
// ErrNotFound and viewer to ErrForbidden. Repository calls always use the data
// owner (scope), never the actor.

var contactRoleCases = []struct {
	name string
	role sharedpolicy.Role
}{
	{name: "owner", role: sharedpolicy.RoleOwner},
	{name: "full access", role: sharedpolicy.RoleFullAccess},
	{name: "viewer", role: sharedpolicy.RoleViewer},
	{name: "none", role: sharedpolicy.RoleNone},
	{name: "suspended", role: sharedpolicy.RoleSuspended},
}

// fakeAuditRecorder captures recorded audit entries.
type fakeAuditRecorder struct {
	entries []auditdomain.Entry
}

func (f *fakeAuditRecorder) Record(_ context.Context, entry auditdomain.Entry) error {
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeAuditRecorder) WithTx(_ transaction.Tx) auditapp.Recorder { return f }

// fakePropertyContactRepo is an in-memory PropertyContactRepository. All
// scoped lookups match on OwnerID so tests verify that repository calls use
// the data owner, not the actor.
type fakePropertyContactRepo struct {
	contacts map[uuid.UUID]domain.PropertyContact
}

func newFakePropertyContactRepo(contacts ...domain.PropertyContact) *fakePropertyContactRepo {
	r := &fakePropertyContactRepo{contacts: make(map[uuid.UUID]domain.PropertyContact, len(contacts))}
	for _, c := range contacts {
		r.contacts[c.ID] = c
	}
	return r
}

func (r *fakePropertyContactRepo) Create(_ context.Context, contact domain.PropertyContact) (domain.PropertyContact, error) {
	r.contacts[contact.ID] = contact
	return contact, nil
}

func (r *fakePropertyContactRepo) ListByProperty(_ context.Context, propertyID, scope uuid.UUID) ([]domain.PropertyContact, error) {
	out := make([]domain.PropertyContact, 0, len(r.contacts))
	for _, c := range r.contacts {
		if c.PropertyID == propertyID && c.OwnerID == scope {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *fakePropertyContactRepo) GetByIDAndOwner(_ context.Context, contactID, scope uuid.UUID) (domain.PropertyContact, error) {
	c, ok := r.contacts[contactID]
	if !ok || c.OwnerID != scope {
		return domain.PropertyContact{}, ErrNotFound
	}
	return c, nil
}

func (r *fakePropertyContactRepo) Update(
	_ context.Context, scope uuid.UUID, contact domain.PropertyContact,
) (domain.PropertyContact, error) {
	c, ok := r.contacts[contact.ID]
	if !ok || c.OwnerID != scope {
		return domain.PropertyContact{}, ErrNotFound
	}
	r.contacts[contact.ID] = contact
	return contact, nil
}

func (r *fakePropertyContactRepo) Delete(_ context.Context, contactID, scope uuid.UUID) error {
	c, ok := r.contacts[contactID]
	if !ok || c.OwnerID != scope {
		return ErrNotFound
	}
	delete(r.contacts, contactID)
	return nil
}

func (r *fakePropertyContactRepo) WithTx(_ transaction.Tx) PropertyContactRepository { return r }

// contactFixture bundles the ids and repositories shared by the matrix tests:
// a property owned by ownerID and one contact of that property.
type contactFixture struct {
	ownerID     uuid.UUID
	actor       uuid.UUID
	propertyID  uuid.UUID
	contactID   uuid.UUID
	contactRepo *fakePropertyContactRepo
}

func newContactFixture() *contactFixture {
	f := &contactFixture{
		ownerID:    uuid.Must(uuid.NewV7()),
		actor:      uuid.Must(uuid.NewV7()),
		propertyID: uuid.Must(uuid.NewV7()),
		contactID:  uuid.Must(uuid.NewV7()),
	}
	f.contactRepo = newFakePropertyContactRepo(domain.PropertyContact{
		ID:         f.contactID,
		PropertyID: f.propertyID,
		OwnerID:    f.ownerID,
		Name:       "Plumber",
		Phone:      "+79161234567",
	})
	return f
}

func (f *contactFixture) propertyRepo() *fakePropertyRepo {
	return newFakePropertyRepo(domain.Property{
		ID:      f.propertyID,
		OwnerID: f.ownerID,
		Status:  domain.PropertyStatusActive,
	})
}

func (f *contactFixture) service(role sharedpolicy.Role, audit *fakeAuditRecorder) *PropertyContactService {
	svc := NewPropertyContactService(f.contactRepo, f.propertyRepo(), newContactTestFactory(f.propertyRepo(), f.contactRepo, audit), nil)
	svc.SetPolicy(staticRolePolicy{role: role})
	return svc
}

// wantContactAuditActorRole maps a policy role to the expected audit actor
// role for write paths that reach the Record call.
func wantContactAuditActorRole(role sharedpolicy.Role) auditdomain.ActorRole {
	if role == sharedpolicy.RoleFullAccess {
		return auditdomain.ActorRoleFullAccess
	}
	return auditdomain.ActorRoleOwner
}

func assertContactAuditActorRole(t *testing.T, audit *fakeAuditRecorder, action auditdomain.Action, want auditdomain.ActorRole) {
	t.Helper()
	if len(audit.entries) != 1 {
		t.Fatalf("audit entries: want 1, got %d", len(audit.entries))
	}
	entry := audit.entries[0]
	if entry.Action != action {
		t.Errorf("audit action: want %q, got %q", action, entry.Action)
	}
	if entry.ActorRole != want {
		t.Errorf("audit actor role: want %q, got %q", want, entry.ActorRole)
	}
}

func assertNoContactAuditEntries(t *testing.T, audit *fakeAuditRecorder) {
	t.Helper()
	if len(audit.entries) != 0 {
		t.Fatalf("audit entries: want 0, got %d", len(audit.entries))
	}
}

func TestPolicyEnforcement_ListPropertyContacts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, tc := range contactRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newContactFixture()
			svc := f.service(tc.role, &fakeAuditRecorder{})

			contacts, err := svc.ListPropertyContacts(ctx, f.actor, f.propertyID)
			if sharedpolicy.CanView(tc.role) {
				if err != nil {
					t.Fatalf("ListPropertyContacts: want success, got %v", err)
				}
				if len(contacts) != 1 || contacts[0].ID != f.contactID {
					t.Fatalf("ListPropertyContacts: want the owner's contact, got %+v", contacts)
				}
				return
			}
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("ListPropertyContacts: want ErrNotFound, got %v", err)
			}
		})
	}
}

func TestPolicyEnforcement_GetPropertyContact(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, tc := range contactRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newContactFixture()
			svc := f.service(tc.role, &fakeAuditRecorder{})

			contact, err := svc.GetPropertyContact(ctx, f.actor, f.propertyID, f.contactID)
			if sharedpolicy.CanView(tc.role) {
				if err != nil {
					t.Fatalf("GetPropertyContact: want success, got %v", err)
				}
				if contact.ID != f.contactID {
					t.Errorf("GetPropertyContact: want contact %s, got %s", f.contactID, contact.ID)
				}
				return
			}
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("GetPropertyContact: want ErrNotFound, got %v", err)
			}
		})
	}
}

func TestPolicyEnforcement_CreatePropertyContact(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, tc := range contactRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newContactFixture()
			audit := &fakeAuditRecorder{}
			svc := f.service(tc.role, audit)

			created, err := svc.CreatePropertyContact(ctx, f.actor, f.propertyID, CreatePropertyContactCommand{
				Name: "Electrician", Phone: "+79169876543",
			})
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("CreatePropertyContact: want success, got %v", err)
				}
				if created.OwnerID != f.ownerID {
					t.Errorf("OwnerID: want the data owner %s, got %s", f.ownerID, created.OwnerID)
				}
				assertContactAuditActorRole(t, audit, auditdomain.ActionPropertyContactCreated, wantContactAuditActorRole(tc.role))
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("CreatePropertyContact: want ErrForbidden, got %v", err)
				}
				assertNoContactAuditEntries(t, audit)
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("CreatePropertyContact: want ErrNotFound, got %v", err)
				}
				assertNoContactAuditEntries(t, audit)
			}
		})
	}
}

func TestPolicyEnforcement_UpdatePropertyContact(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, tc := range contactRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newContactFixture()
			audit := &fakeAuditRecorder{}
			svc := f.service(tc.role, audit)

			name := "Senior Plumber"
			updated, err := svc.UpdatePropertyContact(ctx, f.actor, f.propertyID, f.contactID, UpdatePropertyContactCommand{Name: &name})
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("UpdatePropertyContact: want success, got %v", err)
				}
				if updated.OwnerID != f.ownerID {
					t.Errorf("OwnerID: want the data owner %s, got %s", f.ownerID, updated.OwnerID)
				}
				if updated.Name != name {
					t.Errorf("Name: want %q, got %q", name, updated.Name)
				}
				assertContactAuditActorRole(t, audit, auditdomain.ActionPropertyContactUpdated, wantContactAuditActorRole(tc.role))
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("UpdatePropertyContact: want ErrForbidden, got %v", err)
				}
				assertNoContactAuditEntries(t, audit)
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("UpdatePropertyContact: want ErrNotFound, got %v", err)
				}
				assertNoContactAuditEntries(t, audit)
			}
		})
	}
}

func TestPolicyEnforcement_DeletePropertyContact(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, tc := range contactRoleCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newContactFixture()
			audit := &fakeAuditRecorder{}
			svc := f.service(tc.role, audit)

			err := svc.DeletePropertyContact(ctx, f.actor, f.propertyID, f.contactID)
			switch {
			case sharedpolicy.CanEdit(tc.role):
				if err != nil {
					t.Fatalf("DeletePropertyContact: want success, got %v", err)
				}
				if _, ok := f.contactRepo.contacts[f.contactID]; ok {
					t.Error("DeletePropertyContact: contact was not deleted")
				}
				assertContactAuditActorRole(t, audit, auditdomain.ActionPropertyContactDeleted, wantContactAuditActorRole(tc.role))
			case tc.role == sharedpolicy.RoleViewer:
				if !errors.Is(err, ErrForbidden) {
					t.Fatalf("DeletePropertyContact: want ErrForbidden, got %v", err)
				}
				assertNoContactAuditEntries(t, audit)
			default:
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("DeletePropertyContact: want ErrNotFound, got %v", err)
				}
				assertNoContactAuditEntries(t, audit)
			}
		})
	}
}
