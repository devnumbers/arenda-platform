package application

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// TestTenantContactService_ListTenantContacts_DerivedAccess verifies that the
// owner-wide derived access (issue #157) is honoured: a member with full access
// to one of the owner's properties sees the owner's contacts, a viewer sees
// them too, and a stranger with RoleNone sees nothing.
func TestTenantContactService_ListTenantContacts_DerivedAccess(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	owner := uuid.Must(uuid.NewV7())
	memberFull := uuid.Must(uuid.NewV7())
	memberViewer := uuid.Must(uuid.NewV7())
	stranger := uuid.Must(uuid.NewV7())

	ownerContact := domain.TenantContact{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Name: "Owner Contact"}
	repo := &fakeTenantContactRepo{contacts: []domain.TenantContact{ownerContact}}

	policy := fakePolicy{
		roles: map[[2]uuid.UUID]sharedpolicy.Role{
			{memberFull, owner}:   sharedpolicy.RoleFullAccess,
			{memberViewer, owner}: sharedpolicy.RoleViewer,
			// Stranger has no entry -> RoleNone.
		},
	}
	scopes := fakeAccessibleScopes{
		memberFull:   []uuid.UUID{owner},
		memberViewer: []uuid.UUID{owner},
		// Stranger -> nil (no accessible owners).
	}

	tests := []struct {
		name        string
		actor       uuid.UUID
		wantCount   int
		wantContact bool // Expects ownerContact in result.
	}{
		{"owner sees own", owner, 1, true},
		{"member with full access sees owner contacts", memberFull, 1, true},
		{"member with viewer sees owner contacts", memberViewer, 1, true},
		{"stranger sees nothing (none)", stranger, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := NewTenantContactService(repo, nil, nil)
			svc.SetPolicy(policy)
			svc.SetAccessibleScopes(scopes)

			got, err := svc.ListTenantContacts(ctx, tt.actor)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("got %d contacts, want %d: %+v", len(got), tt.wantCount, got)
			}
			if tt.wantContact {
				if idx := slices.IndexFunc(got, func(c domain.TenantContact) bool { return c.ID == ownerContact.ID }); idx < 0 {
					t.Fatalf("expected ownerContact in result, got %+v", got)
				}
			}
		})
	}
}

// TestTenantContactService_ListTenantContacts_DerivedAccess_Dedup verifies that
// when the actor is also the owner (own contact plus listed as accessible
// owner), the contact is not duplicated.
func TestTenantContactService_ListTenantContacts_DerivedAccess_Dedup(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	owner := uuid.Must(uuid.NewV7())
	ownerContact := domain.TenantContact{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Name: "Owner"}
	repo := &fakeTenantContactRepo{contacts: []domain.TenantContact{ownerContact}}

	policy := fakePolicy{}
	scopes := fakeAccessibleScopes{
		// Owner is (incorrectly) listed as accessible to themselves; the
		// service must dedup so the own contact is not returned twice.
		owner: []uuid.UUID{owner},
	}

	svc := NewTenantContactService(repo, nil, nil)
	svc.SetPolicy(policy)
	svc.SetAccessibleScopes(scopes)

	got, err := svc.ListTenantContacts(ctx, owner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d contacts, want 1 (deduped): %+v", len(got), got)
	}
	if got[0].ID != ownerContact.ID {
		t.Fatalf("got contact %v, want %v", got[0].ID, ownerContact.ID)
	}
}

// TestTenantContactService_ListTenantContactsWithLeaseStatus_DerivedAccess
// verifies that derived access also applies to the lease-status-enriched
// listing.
func TestTenantContactService_ListTenantContactsWithLeaseStatus_DerivedAccess(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	stranger := uuid.Must(uuid.NewV7())

	ownerContact := domain.TenantContact{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Name: "Owner"}
	repo := &fakeTenantContactRepo{contacts: []domain.TenantContact{ownerContact}}

	policy := fakePolicy{roles: map[[2]uuid.UUID]sharedpolicy.Role{
		{member, owner}: sharedpolicy.RoleFullAccess,
	}}
	scopes := fakeAccessibleScopes{
		member:   []uuid.UUID{owner},
		stranger: nil,
	}

	t.Run("member with full access sees owner contacts", func(t *testing.T) {
		t.Parallel()
		svc := NewTenantContactService(repo, nil, nil)
		svc.SetPolicy(policy)
		svc.SetAccessibleScopes(scopes)

		got, err := svc.ListTenantContactsWithLeaseStatus(ctx, member)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d contacts, want 1: %+v", len(got), got)
		}
		if got[0].ID != ownerContact.ID {
			t.Fatalf("got contact %v, want %v", got[0].ID, ownerContact.ID)
		}
	})

	t.Run("stranger sees nothing (none)", func(t *testing.T) {
		t.Parallel()
		svc := NewTenantContactService(repo, nil, nil)
		svc.SetPolicy(policy)
		svc.SetAccessibleScopes(scopes)

		got, err := svc.ListTenantContactsWithLeaseStatus(ctx, stranger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got %d contacts, want 0: %+v", len(got), got)
		}
	})
}

// TestTenantContactService_ListTenantContacts_NilSafe_OwnOnly verifies that
// without policy/scopes injected, only the actor's own contacts are returned.
func TestTenantContactService_ListTenantContacts_NilSafe_OwnOnly(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	owner := uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	repo := &fakeTenantContactRepo{contacts: []domain.TenantContact{
		{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Name: testOwnedRowName},
		{ID: uuid.Must(uuid.NewV7()), OwnerID: other, Name: testOtherRowName},
	}}

	svc := NewTenantContactService(repo, nil, nil) // No SetPolicy/SetAccessibleScopes.

	got, err := svc.ListTenantContacts(ctx, owner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d contacts, want 1 (owner's own only): %+v", len(got), got)
	}
	if got[0].OwnerID != owner {
		t.Fatalf("got owner %v, want %v", got[0].OwnerID, owner)
	}
}

// TestTenantContactService_ListTenantContacts_NilSafe_OnlyScopes verifies that
// when only the scopes adapter is injected (policy is nil), the service still
// falls back to own-only behaviour.
func TestTenantContactService_ListTenantContacts_NilSafe_OnlyScopes(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	owner := uuid.Must(uuid.NewV7())
	repo := &fakeTenantContactRepo{contacts: []domain.TenantContact{
		{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Name: testOwnedRowName},
	}}

	svc := NewTenantContactService(repo, nil, nil)
	svc.SetAccessibleScopes(fakeAccessibleScopes{}) // Policy intentionally left nil.

	got, err := svc.ListTenantContacts(ctx, owner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d contacts, want 1 (nil policy -> own only): %+v", len(got), got)
	}
}

// TestTenantContactService_GetTenantContact_DerivedAccess verifies the
// owner-wide read gate on get-by-id (Property Sharing follow-up): members with
// the view capability read the owner's contact, a stranger gets ErrNotFound,
// and without the policy wired the historical owner-only behaviour is kept.
func TestTenantContactService_GetTenantContact_DerivedAccess(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	owner := uuid.Must(uuid.NewV7())
	memberFull := uuid.Must(uuid.NewV7())
	memberViewer := uuid.Must(uuid.NewV7())
	stranger := uuid.Must(uuid.NewV7())

	ownerContact := domain.TenantContact{ID: uuid.Must(uuid.NewV7()), OwnerID: owner, Name: "Owner Contact"}
	repo := &fakeTenantContactRepo{contacts: []domain.TenantContact{ownerContact}}

	policy := fakePolicy{
		roles: map[[2]uuid.UUID]sharedpolicy.Role{
			{memberFull, owner}:   sharedpolicy.RoleFullAccess,
			{memberViewer, owner}: sharedpolicy.RoleViewer,
			// Stranger has no entry -> RoleNone.
		},
	}

	tests := []struct {
		name    string
		actor   uuid.UUID
		wantErr bool
	}{
		{"owner reads own", owner, false},
		{"member with full access reads owner contact", memberFull, false},
		{"member with viewer reads owner contact", memberViewer, false},
		{"stranger gets not found (none)", stranger, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := NewTenantContactService(repo, nil, nil)
			svc.SetPolicy(policy)

			got, err := svc.GetTenantContact(ctx, tt.actor, ownerContact.ID)
			if tt.wantErr {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("expected ErrNotFound, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != ownerContact.ID {
				t.Fatalf("got contact %v, want %v", got.ID, ownerContact.ID)
			}
		})
	}

	t.Run("nil policy keeps owner-only behaviour", func(t *testing.T) {
		t.Parallel()
		svc := NewTenantContactService(repo, nil, nil) // No SetPolicy.

		if _, err := svc.GetTenantContact(ctx, stranger, ownerContact.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
		got, err := svc.GetTenantContact(ctx, owner, ownerContact.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != ownerContact.ID {
			t.Fatalf("got contact %v, want %v", got.ID, ownerContact.ID)
		}
	})
}
