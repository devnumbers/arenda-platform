package postgres

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// These integration tests run against a real Postgres via TEST_DATABASE_URL
// and are skipped when it is unset. They exercise the shared-access
// enforcement on property contacts
// (Property Sharing follow-up) end-to-end: real MembershipPolicy over real
// repositories, with the service called as owner, full-access member, viewer,
// outsider and suspended member.

// contactPolicyNoCommitTx wraps the test's pgx.Tx as a transaction.Tx whose
// Commit and Rollback are no-ops: the test owns the transaction lifecycle.
type contactPolicyNoCommitTx struct{ pgx.Tx }

func (contactPolicyNoCommitTx) Commit(context.Context) error   { return nil }
func (contactPolicyNoCommitTx) Rollback(context.Context) error { return nil }

// contactPolicyUoW adapts the test's already-open transaction to the
// transaction.UoW port so the service under test opens its transactions
// through runInTx (ADR 0033); the no-commit tx keeps the outer test
// transaction in charge of cleanup.
type contactPolicyUoW struct{ tx pgx.Tx }

func (u contactPolicyUoW) Do(_ context.Context, work func(tx transaction.Tx) error) error {
	return work(contactPolicyNoCommitTx{Tx: u.tx})
}

func contactPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func createContactPolicyTestUser(t *testing.T, ctx context.Context, q *genpostgres.Queries) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	// The phone must be unique per call: derive it from the uuid's random tail.
	_, err = q.CreateUser(ctx, genpostgres.CreateUserParams{
		ID:    contactPgUUID(id),
		Phone: fmt.Sprintf("+7999%07d", binary.BigEndian.Uint32(id[12:])%10000000),
		Role:  "owner",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func createContactPolicyTestProperty(t *testing.T, ctx context.Context, q *genpostgres.Queries, owner uuid.UUID) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = q.CreateProperty(ctx, genpostgres.CreatePropertyParams{
		ID:          contactPgUUID(id),
		OwnerID:     contactPgUUID(owner),
		Name:        "Contact Policy Test Property",
		Type:        "apartment",
		Address:     "",
		Description: pgtype.Text{},
		Attributes:  []byte("{}"),
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("create property: %v", err)
	}
	return id
}

// contactPolicyScenario is the seeded shared-access fixture of the property
// contact policy test: five users, one property, and the contact service wired
// with the real MembershipPolicy.
type contactPolicyScenario struct {
	owner, member, viewer, outsider, suspended uuid.UUID
	property                                   uuid.UUID
	svc                                        *application.PropertyContactService
}

// seedContactPolicyScenario seeds the shared-access scenario: the owner, a
// full-access member, a viewer, an outsider and a suspended member, their
// membership rows, and the contact service wired with the real policy over the
// test transaction.
func seedContactPolicyScenario(t *testing.T, ctx context.Context, tx pgx.Tx) contactPolicyScenario {
	t.Helper()
	q := genpostgres.New(tx)
	owner := createContactPolicyTestUser(t, ctx, q)
	member := createContactPolicyTestUser(t, ctx, q)
	viewer := createContactPolicyTestUser(t, ctx, q)
	outsider := createContactPolicyTestUser(t, ctx, q)
	suspended := createContactPolicyTestUser(t, ctx, q)
	property := createContactPolicyTestProperty(t, ctx, q, owner)

	members := accesspg.NewMembershipRepository(tx)
	policy := accessapp.NewMembershipPolicy(accesspg.NewOwnerResolver(tx), members)
	for user, role := range map[uuid.UUID]accessdomain.Role{
		member: accessdomain.RoleFullAccess,
		viewer: accessdomain.RoleViewer,
	} {
		id, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("new uuid: %v", err)
		}
		if _, err := members.Create(ctx, accessdomain.Membership{
			ID: id, PropertyID: property, UserID: user, Role: role, GrantedBy: owner,
		}); err != nil {
			t.Fatalf("create membership: %v", err)
		}
	}
	suspendedID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := members.CreateWithStatus(ctx, accessdomain.Membership{
		ID: suspendedID, PropertyID: property, UserID: suspended, Role: accessdomain.RoleFullAccess,
		Status: accessdomain.MemberStatusSuspended, GrantedBy: owner,
	}); err != nil {
		t.Fatalf("create suspended membership: %v", err)
	}

	contactRepo := NewPropertyContactRepository(tx)
	propertyRepo := NewPropertyRepository(tx)
	// The contact service runs its mutations through the properties γ-factory
	// (ADR 0033); its optional property-service stores stay unwired here.
	factory := application.NewTxStoreFactory(propertyRepo, nil, contactRepo, nil, nil, contactPolicyUoW{tx: tx})
	svc := application.NewPropertyContactService(contactRepo, propertyRepo, factory, nil)
	svc.SetPolicy(policy)

	return contactPolicyScenario{
		owner: owner, member: member, viewer: viewer, outsider: outsider, suspended: suspended,
		property: property,
		svc:      svc,
	}
}

// assertContactReadAccess checks the read outcomes: the owner, the member and
// the viewer see the contact, while the outsider and the suspended member get
// ErrNotFound (object privacy) even on write.
func assertContactReadAccess(t *testing.T, ctx context.Context, s contactPolicyScenario, contactID uuid.UUID) {
	t.Helper()
	for name, actor := range map[string]uuid.UUID{"owner": s.owner, "member": s.member, "viewer": s.viewer} {
		if _, err := s.svc.GetPropertyContact(ctx, actor, s.property, contactID); err != nil {
			t.Errorf("GetPropertyContact as %s: %v", name, err)
		}
		if _, err := s.svc.ListPropertyContacts(ctx, actor, s.property); err != nil {
			t.Errorf("ListPropertyContacts as %s: %v", name, err)
		}
	}
	for name, actor := range map[string]uuid.UUID{"outsider": s.outsider, "suspended": s.suspended} {
		if _, err := s.svc.GetPropertyContact(ctx, actor, s.property, contactID); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("GetPropertyContact as %s: want ErrNotFound, got %v", name, err)
		}
		if _, err := s.svc.ListPropertyContacts(ctx, actor, s.property); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("ListPropertyContacts as %s: want ErrNotFound, got %v", name, err)
		}
		if _, err := s.svc.CreatePropertyContact(ctx, actor, s.property, application.CreatePropertyContactCommand{
			Name: "Intruder", Phone: "+79160000022",
		}); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("CreatePropertyContact as %s: want ErrNotFound, got %v", name, err)
		}
	}
}

// assertContactWriteAccess checks the write outcomes: the member updates and
// deletes the owner's contact, while the viewer gets ErrForbidden.
func assertContactWriteAccess(t *testing.T, ctx context.Context, s contactPolicyScenario, contactID uuid.UUID) {
	t.Helper()
	newName := "Senior Plumber"
	updated, err := s.svc.UpdatePropertyContact(ctx, s.member, s.property, contactID,
		application.UpdatePropertyContactCommand{Name: &newName})
	if err != nil {
		t.Fatalf("UpdatePropertyContact as member: %v", err)
	}
	if updated.OwnerID != s.owner {
		t.Errorf("updated OwnerID: want owner %s, got %s", s.owner, updated.OwnerID)
	}
	viewerName := "Viewer Rename"
	if _, err := s.svc.UpdatePropertyContact(ctx, s.viewer, s.property, contactID,
		application.UpdatePropertyContactCommand{Name: &viewerName}); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("UpdatePropertyContact as viewer: want ErrForbidden, got %v", err)
	}
	if err := s.svc.DeletePropertyContact(ctx, s.viewer, s.property, contactID); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("DeletePropertyContact as viewer: want ErrForbidden, got %v", err)
	}
	if err := s.svc.DeletePropertyContact(ctx, s.member, s.property, contactID); err != nil {
		t.Fatalf("DeletePropertyContact as member: %v", err)
	}
}

// TestPolicyIntegration_PropertyContacts exercises the shared-access
// enforcement on property contacts (Property Sharing follow-up) end-to-end: a
// full-access member manages the owner's contacts, a viewer reads but cannot
// write, and an outsider or a suspended member gets ErrNotFound.
func TestPolicyIntegration_PropertyContacts(t *testing.T) {
	t.Parallel()

	pool := setupPropertiesIntegrationDB(t)
	ctx := t.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() {
		// Rollback failure means test isolation broke: rows written in the
		// aborted test transaction would persist in the shared database.
		if err := tx.Rollback(ctx); err != nil {
			t.Errorf("rollback properties test tx: %v", err)
		}
	}()

	s := seedContactPolicyScenario(t, ctx, tx)

	// The member creates a contact: it lands on the owner's scope.
	created, err := s.svc.CreatePropertyContact(ctx, s.member, s.property, application.CreatePropertyContactCommand{
		Name: "Plumber", Phone: "+79160000021",
	})
	if err != nil {
		t.Fatalf("CreatePropertyContact as member: %v", err)
	}
	if created.OwnerID != s.owner {
		t.Errorf("created OwnerID: want owner %s, got %s", s.owner, created.OwnerID)
	}

	assertContactReadAccess(t, ctx, s, created.ID)
	assertContactWriteAccess(t, ctx, s, created.ID)
}
