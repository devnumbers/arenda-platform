//go:build integration

package application_test

// The integration harness of the contacts context: the contact use case
// service over the real stores, the real membership policy (ADR 0028) and
// the real audit recorder, against testcontainers PostgreSQL — the role
// matrix, the book scopes, the search and the property-detach lifecycle of
// ADR 0051 (ticket #506).

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	contactspg "github.com/nambers/arenda-planform/apps/backend/internal/contacts/adapters/postgres"
	contactsapp "github.com/nambers/arenda-planform/apps/backend/internal/contacts/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	sharedclock "github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// contactFirstName is the shared name literal of the fixtures (goconst: one
// home).
const contactFirstName = "Пётр"

// plumberRole is the shared role literal of the fixtures.
const plumberRole = "сантехник"

// mustEqual is the assertion helper keeping the test bodies branch-free.
func mustEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: want %v, got %v", name, want, got)
	}
}

// contactsHarness wires the contacts context to real PostgreSQL: the service
// over the real stores and membership policy, with the seeded shared-access
// cast ready on demand.
type contactsHarness struct {
	t    *testing.T
	pool *pgxpool.Pool
	svc  *contactsapp.ContactService

	owner     uuid.UUID
	member    uuid.UUID
	viewer    uuid.UUID
	outsider  uuid.UUID
	suspended uuid.UUID
	property  uuid.UUID
	otherProp uuid.UUID
}

func newContactsHarness(t *testing.T) *contactsHarness {
	t.Helper()

	pool := testdb.Setup(t)
	policy := accessapp.NewMembershipPolicy(
		accesspg.NewOwnerResolver(pool),
		accesspg.NewMembershipRepository(pool),
	)
	audit := auditapp.NewService(auditpg.NewWriter(pool), sharedclock.Real{})
	uow := pgdb.NewUoW(pool, slog.New(slog.DiscardHandler))
	factory := contactsapp.NewTxStoreFactory(
		contactspg.NewContactStore(pool),
		contactspg.NewPropertyStore(pool),
		audit,
		uow,
	)
	return &contactsHarness{
		t:    t,
		pool: pool,
		svc:  contactsapp.NewContactService(factory, policy),
	}
}

// seedUser inserts a user with a unique phone and returns the id.
func (h *contactsHarness) seedUser() uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
	if _, err := h.pool.Exec(h.t.Context(),
		`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, 'Europe/Moscow')`,
		id, phone, actor.RoleOwner,
	); err != nil {
		h.t.Fatalf("seed user: %v", err)
	}
	return id
}

// seedProperty inserts an active property of the owner and returns the id.
func (h *contactsHarness) seedProperty(owner uuid.UUID) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.t.Context(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Квартира', 'apartment', 'Москва, Тверская 1', 'active')`,
		id, owner,
	); err != nil {
		h.t.Fatalf("seed property: %v", err)
	}
	return id
}

// seedMembership grants the user the role on the property.
func (h *contactsHarness) seedMembership(user, property uuid.UUID, role accessdomain.Role) {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	members := accesspg.NewMembershipRepository(h.pool)
	if _, err := members.Create(h.t.Context(), accessdomain.Membership{
		ID: id, PropertyID: property, UserID: user, Role: role, GrantedBy: h.owner,
	}); err != nil {
		h.t.Fatalf("create membership: %v", err)
	}
}

// seedSuspendedMembership inserts the user as a suspended full-access member.
func (h *contactsHarness) seedSuspendedMembership(user, property uuid.UUID) {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	members := accesspg.NewMembershipRepository(h.pool)
	if _, err := members.CreateWithStatus(h.t.Context(), accessdomain.Membership{
		ID: id, PropertyID: property, UserID: user, Role: accessdomain.RoleFullAccess,
		Status: accessdomain.MemberStatusSuspended, GrantedBy: h.owner,
	}); err != nil {
		h.t.Fatalf("create suspended membership: %v", err)
	}
}

// createCmd builds a valid create command for the actor's own book.
func createCmd() contactsapp.CreateContactCommand {
	return contactsapp.CreateContactCommand{
		FirstName:         contactFirstName,
		LastName:          "Иванов",
		Role:              plumberRole,
		Phone:             "89161234567",
		Email:             "plumber@example.ru",
		MessengerName:     "Telegram",
		MessengerUsername: "@plumber",
		Note:              "Код домофона 1234",
	}
}

// create creates a contact as the owner, failing the test on any error.
func (h *contactsHarness) create(cmd contactsapp.CreateContactCommand) domain.Contact {
	h.t.Helper()
	created, err := h.svc.CreateContact(h.t.Context(), h.owner, cmd)
	if err != nil {
		h.t.Fatalf("create contact: %v", err)
	}
	return created
}

// contactActions loads the contact's audit trail actions in recording order.
func (h *contactsHarness) contactActions(t *testing.T, contactID uuid.UUID) []string {
	t.Helper()
	rows, err := h.pool.Query(t.Context(),
		`SELECT action FROM audit_log WHERE entity_type = 'contact' AND entity_id = $1 ORDER BY created_at, id`,
		contactID)
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	defer rows.Close()
	actions := []string{}
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			t.Fatalf("scan audit: %v", err)
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate audit: %v", err)
	}
	return actions
}

func TestContactsIntegration_BookCRUDAndAudit(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	ctx := t.Context()

	created := h.create(createCmd())
	mustEqual(t, "owner", created.OwnerID, h.owner)
	mustEqual(t, "phone", created.Phone, "+79161234567")
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("timestamps: want persisted, got %v/%v", created.CreatedAt, created.UpdatedAt)
	}

	got, err := h.svc.GetContact(ctx, h.owner, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	mustEqual(t, "note", got.Note, "Код домофона 1234")
	mustEqual(t, "messenger username", got.MessengerUsername, "@plumber")

	newRole := "прораб"
	newPhone := "+79160001122"
	emptyEmail := ""
	updated, err := h.svc.UpdateContact(ctx, h.owner, created.ID, contactsapp.UpdateContactCommand{
		Role:  &newRole,
		Phone: &newPhone,
		Email: &emptyEmail,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	mustEqual(t, "updated role", updated.Role, newRole)
	mustEqual(t, "updated phone", updated.Phone, newPhone)
	mustEqual(t, "cleared email", updated.Email, "")

	if err := h.svc.DeleteContact(ctx, h.owner, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := h.svc.GetContact(ctx, h.owner, created.ID); !errors.Is(err, contactsapp.ErrNotFound) {
		t.Fatalf("get after delete: want ErrNotFound, got %v", err)
	}

	want := []string{"contact.created", "contact.updated", "contact.deleted"}
	actions := h.contactActions(t, created.ID)
	mustEqual(t, "audit actions count", len(actions), len(want))
	for i := range want {
		mustEqual(t, "audit actions", actions[i], want[i])
	}
}

func TestContactsIntegration_SearchAndScopes(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	h.property = h.seedProperty(h.owner)

	plumber := h.create(contactsapp.CreateContactCommand{
		FirstName: "Иван", LastName: "Сантехников", Role: plumberRole, Phone: "+79160000001",
	})
	cleaner := h.create(contactsapp.CreateContactCommand{
		FirstName: "Мария", Role: "уборщица", Email: "maria@example.ru",
	})
	propID := h.property
	concierge := h.create(contactsapp.CreateContactCommand{
		FirstName: contactFirstName, Role: "консьерж", PropertyID: &propID, MessengerUsername: "@ptcon",
	})

	cases := []struct {
		name   string
		scope  contactsapp.ListScope
		search string
		want   []uuid.UUID
	}{
		{"whole book", contactsapp.ListScopeAll, "", []uuid.UUID{plumber.ID, cleaner.ID, concierge.ID}},
		{"unbound only", contactsapp.ListScopeWithoutProperty, "", []uuid.UUID{plumber.ID, cleaner.ID}},
		{"property only", contactsapp.ListScopeProperty, "", []uuid.UUID{concierge.ID}},
		{"search by role lowercase", contactsapp.ListScopeAll, "сантех", []uuid.UUID{plumber.ID}},
		{"search by name uppercase", contactsapp.ListScopeAll, "МАРИЯ", []uuid.UUID{cleaner.ID}},
		{"search by phone", contactsapp.ListScopeAll, "79160000001", []uuid.UUID{plumber.ID}},
		{"search by email", contactsapp.ListScopeAll, "maria@EX", []uuid.UUID{cleaner.ID}},
		{"search by messenger username", contactsapp.ListScopeProperty, "@ptcon", []uuid.UUID{concierge.ID}},
		{"search by last name", contactsapp.ListScopeAll, "ов", []uuid.UUID{plumber.ID}},
		{"search with a LIKE wildcard matches literally", contactsapp.ListScopeAll, "%", nil},
		{"search with an underscore matches literally", contactsapp.ListScopeAll, "_", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := h.svc.ListContacts(t.Context(), h.owner, contactsapp.ListQuery{
				Scope: tc.scope, PropertyID: h.property, Search: tc.search,
			})
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			gotIDs := make([]uuid.UUID, 0, len(got))
			for _, c := range got {
				gotIDs = append(gotIDs, c.ID)
			}
			mustEqual(t, "listed contacts", len(gotIDs), len(tc.want))
			for i := range tc.want {
				mustEqual(t, "listed contact", gotIDs[i], tc.want[i])
			}
		})
	}
}

func TestContactsIntegration_PropertyDeleteDetaches(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	h.property = h.seedProperty(h.owner)
	propID := h.property
	created := h.create(contactsapp.CreateContactCommand{
		FirstName: contactFirstName, Role: plumberRole, PropertyID: &propID,
	})

	if _, err := h.pool.Exec(t.Context(), `DELETE FROM properties WHERE id = $1`, h.property); err != nil {
		t.Fatalf("delete property: %v", err)
	}

	got, err := h.svc.GetContact(t.Context(), h.owner, created.ID)
	if err != nil {
		t.Fatalf("get after property delete: %v", err)
	}
	if got.PropertyID != nil {
		t.Fatalf("property: want detached (nil), got %s", *got.PropertyID)
	}

	unbound, err := h.svc.ListContacts(t.Context(), h.owner, contactsapp.ListQuery{
		Scope: contactsapp.ListScopeWithoutProperty,
	})
	if err != nil {
		t.Fatalf("list unbound: %v", err)
	}
	mustEqual(t, "unbound contacts", len(unbound), 1)
	mustEqual(t, "detached contact", unbound[0].ID, created.ID)
}

// TestContactsIntegration_RoleMatrix exercises the shared-access enforcement
// (ADR 0028, ADR 0051) end-to-end over the real membership policy: a
// full-access member manages the owner's property-bound contacts, a viewer
// reads but cannot write, an outsider or a suspended member gets ErrNotFound
// (the privacy 404).
func TestContactsIntegration_RoleMatrix(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	h.member = h.seedUser()
	h.viewer = h.seedUser()
	h.outsider = h.seedUser()
	h.suspended = h.seedUser()
	h.property = h.seedProperty(h.owner)
	h.seedMembership(h.member, h.property, accessdomain.RoleFullAccess)
	h.seedMembership(h.viewer, h.property, accessdomain.RoleViewer)
	h.seedSuspendedMembership(h.suspended, h.property)
	ctx := t.Context()

	propID := h.property
	cmd := createCmd()
	cmd.PropertyID = &propID
	created, err := h.svc.CreateContact(ctx, h.member, cmd)
	if err != nil {
		t.Fatalf("create as member: %v", err)
	}
	mustEqual(t, "created owner", created.OwnerID, h.owner)

	for name, user := range map[string]uuid.UUID{
		"owner": h.owner, "member": h.member, "viewer": h.viewer,
	} {
		if _, err := h.svc.GetContact(ctx, user, created.ID); err != nil {
			t.Errorf("GetContact as %s: %v", name, err)
		}
		if _, err := h.svc.ListContacts(ctx, user, contactsapp.ListQuery{
			Scope: contactsapp.ListScopeProperty, PropertyID: h.property,
		}); err != nil {
			t.Errorf("ListContacts as %s: %v", name, err)
		}
	}
	for name, user := range map[string]uuid.UUID{
		"outsider": h.outsider, "suspended": h.suspended,
	} {
		if _, err := h.svc.GetContact(ctx, user, created.ID); !errors.Is(err, contactsapp.ErrNotFound) {
			t.Errorf("GetContact as %s: want ErrNotFound, got %v", name, err)
		}
		if _, err := h.svc.ListContacts(ctx, user, contactsapp.ListQuery{
			Scope: contactsapp.ListScopeProperty, PropertyID: h.property,
		}); !errors.Is(err, contactsapp.ErrNotFound) {
			t.Errorf("ListContacts as %s: want ErrNotFound, got %v", name, err)
		}
		if _, err := h.svc.CreateContact(ctx, user, cmd); !errors.Is(err, contactsapp.ErrNotFound) {
			t.Errorf("CreateContact as %s: want ErrNotFound, got %v", name, err)
		}
	}

	newRole := "старший сантехник"
	if _, err := h.svc.UpdateContact(ctx, h.viewer, created.ID, contactsapp.UpdateContactCommand{
		Role: &newRole,
	}); !errors.Is(err, contactsapp.ErrForbidden) {
		t.Errorf("UpdateContact as viewer: want ErrForbidden, got %v", err)
	}
	if err := h.svc.DeleteContact(ctx, h.viewer, created.ID); !errors.Is(err, contactsapp.ErrForbidden) {
		t.Errorf("DeleteContact as viewer: want ErrForbidden, got %v", err)
	}

	updated, err := h.svc.UpdateContact(ctx, h.member, created.ID, contactsapp.UpdateContactCommand{Role: &newRole})
	if err != nil {
		t.Fatalf("UpdateContact as member: %v", err)
	}
	mustEqual(t, "updated role", updated.Role, newRole)
	if err := h.svc.DeleteContact(ctx, h.member, created.ID); err != nil {
		t.Fatalf("DeleteContact as member: %v", err)
	}
}

func TestContactsIntegration_MoveBetweenProperties(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	h.member = h.seedUser()
	h.property = h.seedProperty(h.owner)
	h.otherProp = h.seedProperty(h.owner)
	h.seedMembership(h.member, h.property, accessdomain.RoleFullAccess)
	h.seedMembership(h.member, h.otherProp, accessdomain.RoleViewer)
	ctx := t.Context()

	propID := h.property
	created := h.create(contactsapp.CreateContactCommand{
		FirstName: contactFirstName, Role: plumberRole, PropertyID: &propID,
	})

	// The member passes the source gate but holds only a viewer's role on
	// the target: the move fails forbidden.
	if _, err := h.svc.UpdateContact(ctx, h.member, created.ID, contactsapp.UpdateContactCommand{
		PropertyID: &contactsapp.PropertyIDUpdate{Value: &h.otherProp},
	}); !errors.Is(err, contactsapp.ErrForbidden) {
		t.Fatalf("move as member to viewer-gated target: want ErrForbidden, got %v", err)
	}

	moved, err := h.svc.UpdateContact(ctx, h.owner, created.ID, contactsapp.UpdateContactCommand{
		PropertyID: &contactsapp.PropertyIDUpdate{Value: &h.otherProp},
	})
	if err != nil {
		t.Fatalf("move as owner: %v", err)
	}
	if moved.PropertyID == nil || *moved.PropertyID != h.otherProp {
		t.Fatalf("moved property: want %s, got %v", h.otherProp, moved.PropertyID)
	}
	mustEqual(t, "moved owner", moved.OwnerID, h.owner)

	// Clearing the binding detaches the card into the owner-only book.
	detached, err := h.svc.UpdateContact(ctx, h.owner, created.ID, contactsapp.UpdateContactCommand{
		PropertyID: &contactsapp.PropertyIDUpdate{Value: nil},
	})
	if err != nil {
		t.Fatalf("clear binding: %v", err)
	}
	if detached.PropertyID != nil {
		t.Fatalf("detached property: want nil, got %s", *detached.PropertyID)
	}
}
