//go:build integration

package application_test

// The integration harness of the contacts context: the contact use case
// service over the real stores, the real membership policy (ADR 0028) and
// the real audit recorder, against testcontainers PostgreSQL — the role
// matrix, the book scopes, the search and the property-detach lifecycle of
// ADR 0054 (ticket #506).

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
	historypg "github.com/nambers/arenda-planform/apps/backend/internal/history/adapters/postgres"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	realtimetest "github.com/nambers/arenda-planform/apps/backend/internal/realtime/realtimetest"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	sharedclock "github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// contactFirstName is the shared name literal of the fixtures (goconst: one
// home).
const contactFirstName = "Пётр"

// contactLastName is the shared surname literal of the fixtures.
const contactLastName = "Сантехников"

// plumberRole is the shared role literal of the fixtures.
const plumberRole = "сантехник"

// flatPropertyName is the display name of the property the union and
// book-span tests seed.
const flatPropertyName = "Квартира"

// memberFirstName is the given name of the member's own card in the union
// and book-span fixtures.
const memberFirstName = "Мария"

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
	// Realtime is the recording carrier the service dispatches its frames
	// through — the realtime seam's test double (карта #714, #716).
	realtime *realtimetest.RecordingPublisher

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
		historypg.NewRecorder(pool),
		uow,
	)
	realtime := &realtimetest.RecordingPublisher{}
	svc := contactsapp.NewContactService(factory, policy)
	svc.SetRealtimePublisher(realtime)
	return &contactsHarness{
		t:        t,
		pool:     pool,
		svc:      svc,
		realtime: realtime,
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
	return h.seedPropertyNamed(owner, "Квартира")
}

// seedPropertyNamed inserts an active property with the given display name.
func (h *contactsHarness) seedPropertyNamed(owner uuid.UUID, name string) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	if _, err := h.pool.Exec(h.t.Context(),
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, $3, 'apartment', 'Москва, Тверская 1', 'active')`,
		id, owner, name,
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
		FirstName: "Иван", LastName: contactLastName, Role: plumberRole, Phone: "+79160000001",
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
			page, err := h.svc.ListContacts(t.Context(), h.owner, contactsapp.ListQuery{
				Scope: tc.scope, PropertyID: h.property, Search: tc.search,
			})
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			got := page.Items
			gotIDs := make([]uuid.UUID, 0, len(got))
			for _, c := range got {
				gotIDs = append(gotIDs, c.Contact.ID)
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

	unboundPage, err := h.svc.ListContacts(t.Context(), h.owner, contactsapp.ListQuery{
		Scope: contactsapp.ListScopeWithoutProperty,
	})
	if err != nil {
		t.Fatalf("list unbound: %v", err)
	}
	unbound := unboundPage.Items
	mustEqual(t, "unbound contacts", len(unbound), 1)
	mustEqual(t, "detached contact", unbound[0].Contact.ID, created.ID)
}

// TestContactsIntegration_RoleMatrix exercises the shared-access enforcement
// (ADR 0028, ADR 0054) end-to-end over the real membership policy: a
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

// listFlat runs the whole-book listing for the actor and returns the card
// ids with the property-name projection.
func listFlat(
	t *testing.T, svc *contactsapp.ContactService, actorID uuid.UUID, q contactsapp.ListQuery,
) (ids []uuid.UUID, propertyNames map[uuid.UUID]string) {
	t.Helper()
	page, err := svc.ListContacts(t.Context(), actorID, q)
	if err != nil {
		t.Fatalf("flat list: %v", err)
	}
	got := page.Items
	ids = make([]uuid.UUID, 0, len(got))
	propertyNames = make(map[uuid.UUID]string, len(got))
	for _, listed := range got {
		ids = append(ids, listed.Contact.ID)
		propertyNames[listed.Contact.ID] = listed.PropertyName
	}
	return ids, propertyNames
}

// walkBook lists the book in portions of limit, echoing nextCursor until a
// page comes back short, and returns the collected ids. A repeated id, a
// cursor loop or a missing nextCursor on a full page all fail loudly —
// the keyset walk must be total (ticket #600).
func walkBook(
	t *testing.T, svc *contactsapp.ContactService, actorID uuid.UUID, q contactsapp.ListQuery, limit int32,
) []uuid.UUID {
	t.Helper()
	ids := []uuid.UUID{}
	seen := map[uuid.UUID]bool{}
	for pages := 1; ; pages++ {
		if pages > 10 {
			t.Fatal("cursor loop: more than 10 pages collected")
		}
		q.Limit = limit
		page, err := svc.ListContacts(t.Context(), actorID, q)
		if err != nil {
			t.Fatalf("walk page %d: %v", pages, err)
		}
		for _, listed := range page.Items {
			if seen[listed.Contact.ID] {
				t.Fatalf("page %d repeats id %s", pages, listed.Contact.ID)
			}
			seen[listed.Contact.ID] = true
			ids = append(ids, listed.Contact.ID)
		}
		if page.NextCursor == "" {
			return ids
		}
		q.Cursor = page.NextCursor
	}
}

// TestContactsIntegration_FlatBookUnion exercises the merged visibility of
// the whole-book listing (ADR 0054, ADR 0028): a shared member sees the
// property-bound cards of the shared properties plus their own unbound
// cards, never the owner's unbound ones; a suspended membership sees only
// its own book; the owner sees the whole own book.
func TestContactsIntegration_FlatBookUnion(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	h.member = h.seedUser()
	h.viewer = h.seedUser()
	h.suspended = h.seedUser()
	h.property = h.seedPropertyNamed(h.owner, flatPropertyName)
	h.seedMembership(h.member, h.property, accessdomain.RoleFullAccess)
	h.seedMembership(h.viewer, h.property, accessdomain.RoleViewer)
	h.seedSuspendedMembership(h.suspended, h.property)
	ctx := t.Context()

	propID := h.property
	bound := h.create(contactsapp.CreateContactCommand{
		FirstName: "Борис", Role: plumberRole, PropertyID: &propID,
	})
	unbound := h.create(contactsapp.CreateContactCommand{
		FirstName: "Анна", Role: "уборщица",
	})
	memberUnbound, err := h.svc.CreateContact(ctx, h.member, contactsapp.CreateContactCommand{
		FirstName: memberFirstName,
	})
	if err != nil {
		t.Fatalf("create member unbound: %v", err)
	}
	suspendedUnbound, err := h.svc.CreateContact(ctx, h.suspended, contactsapp.CreateContactCommand{
		FirstName: "Олег",
	})
	if err != nil {
		t.Fatalf("create suspended unbound: %v", err)
	}

	t.Run("member sees the bound cards of the shared property and own unbound", func(t *testing.T) {
		t.Parallel()
		ids, names := listFlat(t, h.svc, h.member, contactsapp.ListQuery{Scope: contactsapp.ListScopeAll})
		want := []uuid.UUID{bound.ID, memberUnbound.ID}
		if len(ids) != len(want) {
			t.Fatalf("listed ids = %v, want %v", ids, want)
		}
		for i := range want {
			mustEqual(t, "listed contact", ids[i], want[i])
		}
		mustEqual(t, "bound property name", names[bound.ID], flatPropertyName)
		mustEqual(t, "unbound property name", names[memberUnbound.ID], "")
	})

	t.Run("viewer sees the bound cards only", func(t *testing.T) {
		t.Parallel()
		ids, _ := listFlat(t, h.svc, h.viewer, contactsapp.ListQuery{Scope: contactsapp.ListScopeAll})
		if len(ids) != 1 || ids[0] != bound.ID {
			t.Fatalf("listed ids = %v, want [%s]", ids, bound.ID)
		}
	})

	t.Run("suspended membership hides the shared property", func(t *testing.T) {
		t.Parallel()
		ids, _ := listFlat(t, h.svc, h.suspended, contactsapp.ListQuery{Scope: contactsapp.ListScopeAll})
		if len(ids) != 1 || ids[0] != suspendedUnbound.ID {
			t.Fatalf("listed ids = %v, want [%s]", ids, suspendedUnbound.ID)
		}
	})

	t.Run("owner sees the whole own book", func(t *testing.T) {
		t.Parallel()
		ids, _ := listFlat(t, h.svc, h.owner, contactsapp.ListQuery{Scope: contactsapp.ListScopeAll})
		if len(ids) != 2 {
			t.Fatalf("listed ids = %v, want 2 own cards", ids)
		}
	})

	t.Run("unbound scope keeps the owner's unbound alone", func(t *testing.T) {
		t.Parallel()
		ids, _ := listFlat(t, h.svc, h.owner, contactsapp.ListQuery{Scope: contactsapp.ListScopeWithoutProperty})
		if len(ids) != 1 || ids[0] != unbound.ID {
			t.Fatalf("listed ids = %v, want [%s]", ids, unbound.ID)
		}
	})
}

// TestContactsIntegration_FlatBookSort pins the server-side ordering of the
// whole-book listing: Russian collation by display name (ё next to е), and
// the property sort with the unbound cards first in both directions and
// contact-name order inside the groups.
func TestContactsIntegration_FlatBookSort(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	anna := h.create(contactsapp.CreateContactCommand{FirstName: "Анна"})
	yolkin := h.create(contactsapp.CreateContactCommand{FirstName: "Ёлкин"})
	p1 := h.seedPropertyNamed(h.owner, "Моя квартира")
	p2 := h.seedPropertyNamed(h.owner, "Студия")
	p1id, p2id := p1, p2
	boris := h.create(contactsapp.CreateContactCommand{
		FirstName: "Борис", PropertyID: &p1id,
	})
	ezhov := h.create(contactsapp.CreateContactCommand{
		FirstName: "Ежов", LastName: "Игорь", PropertyID: &p2id,
	})

	t.Run("name ascending: Russian collation, ё next to е", func(t *testing.T) {
		t.Parallel()
		ids, _ := listFlat(t, h.svc, h.owner, contactsapp.ListQuery{
			Scope: contactsapp.ListScopeAll, Sort: contactsapp.ListSortName, Order: contactsapp.ListOrderAsc,
		})
		want := []uuid.UUID{anna.ID, boris.ID, ezhov.ID, yolkin.ID}
		if len(ids) != len(want) {
			t.Fatalf("listed ids = %v, want %v", ids, want)
		}
		for i := range want {
			mustEqual(t, "sort order", ids[i], want[i])
		}
	})

	t.Run("name descending reverses exactly", func(t *testing.T) {
		t.Parallel()
		ids, _ := listFlat(t, h.svc, h.owner, contactsapp.ListQuery{
			Scope: contactsapp.ListScopeAll, Sort: contactsapp.ListSortName, Order: contactsapp.ListOrderDesc,
		})
		want := []uuid.UUID{yolkin.ID, ezhov.ID, boris.ID, anna.ID}
		if len(ids) != len(want) {
			t.Fatalf("listed ids = %v, want %v", ids, want)
		}
		for i := range want {
			mustEqual(t, "sort order", ids[i], want[i])
		}
	})

	t.Run("property ascending: unbound first, groups by property, names inside", func(t *testing.T) {
		t.Parallel()
		ids, names := listFlat(t, h.svc, h.owner, contactsapp.ListQuery{
			Scope: contactsapp.ListScopeAll, Sort: contactsapp.ListSortProperty, Order: contactsapp.ListOrderAsc,
		})
		want := []uuid.UUID{anna.ID, yolkin.ID, boris.ID, ezhov.ID}
		if len(ids) != len(want) {
			t.Fatalf("listed ids = %v, want %v", ids, want)
		}
		for i := range want {
			mustEqual(t, "sort order", ids[i], want[i])
		}
		mustEqual(t, "unbound group property name", names[anna.ID], "")
		mustEqual(t, "first property group name", names[boris.ID], "Моя квартира")
		mustEqual(t, "second property group name", names[ezhov.ID], "Студия")
	})

	t.Run("property descending: unbound still first, groups reverse, names reverse", func(t *testing.T) {
		t.Parallel()
		ids, _ := listFlat(t, h.svc, h.owner, contactsapp.ListQuery{
			Scope: contactsapp.ListScopeAll, Sort: contactsapp.ListSortProperty, Order: contactsapp.ListOrderDesc,
		})
		want := []uuid.UUID{yolkin.ID, anna.ID, ezhov.ID, boris.ID}
		if len(ids) != len(want) {
			t.Fatalf("listed ids = %v, want %v", ids, want)
		}
		for i := range want {
			mustEqual(t, "sort order", ids[i], want[i])
		}
	})
}

// TestContactsIntegration_PropertyScopeSpansBooks pins the binding-driven
// visibility: a card moved onto a property is listed in that property's
// slice and in the flat book of everyone who can view the property, whatever
// book the card lives in (the book owner never changes on a move).
func TestContactsIntegration_PropertyScopeSpansBooks(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	h.member = h.seedUser()
	h.property = h.seedPropertyNamed(h.owner, flatPropertyName)
	h.seedMembership(h.member, h.property, accessdomain.RoleFullAccess)
	ctx := t.Context()

	// The member's own card moves onto the owner's property: the book stays
	// the member's, the binding now points at the owner's property.
	memberCard, err := h.svc.CreateContact(ctx, h.member, contactsapp.CreateContactCommand{
		FirstName: memberFirstName,
	})
	if err != nil {
		t.Fatalf("create member card: %v", err)
	}
	moved, err := h.svc.UpdateContact(ctx, h.member, memberCard.ID, contactsapp.UpdateContactCommand{
		PropertyID: &contactsapp.PropertyIDUpdate{Value: &h.property},
	})
	if err != nil {
		t.Fatalf("move member card: %v", err)
	}
	mustEqual(t, "moved book owner", moved.OwnerID, h.member)

	t.Run("owner's property slice spans books", func(t *testing.T) {
		t.Parallel()
		propertyPage, err := h.svc.ListContacts(t.Context(), h.owner, contactsapp.ListQuery{
			Scope: contactsapp.ListScopeProperty, PropertyID: h.property,
		})
		if err != nil {
			t.Fatalf("property list: %v", err)
		}
		got := propertyPage.Items
		if len(got) != 1 || got[0].Contact.ID != memberCard.ID {
			t.Fatalf("listed = %v, want [%s]", got, memberCard.ID)
		}
		mustEqual(t, "property name", got[0].PropertyName, flatPropertyName)
	})

	t.Run("the flat book of the property owner sees the foreign-book card", func(t *testing.T) {
		t.Parallel()
		ids, _ := listFlat(t, h.svc, h.owner, contactsapp.ListQuery{Scope: contactsapp.ListScopeAll})
		if len(ids) != 1 || ids[0] != memberCard.ID {
			t.Fatalf("listed ids = %v, want [%s]", ids, memberCard.ID)
		}
	})

	t.Run("the card's owner still sees it in the flat book", func(t *testing.T) {
		t.Parallel()
		ids, _ := listFlat(t, h.svc, h.member, contactsapp.ListQuery{Scope: contactsapp.ListScopeAll})
		if len(ids) != 1 || ids[0] != memberCard.ID {
			t.Fatalf("listed ids = %v, want [%s]", ids, memberCard.ID)
		}
	})
}

// assertSameWalk pins the totality of a keyset walk (ticket #600): two
// windows collect the same id sequence — want of them, no duplicates, no
// drops; any skipped or repeated row desynchronizes the two walks.
func assertSameWalk(t *testing.T, by50, by100 []uuid.UUID, want int) {
	t.Helper()
	if len(by50) != want || len(by100) != want {
		t.Fatalf("walks collected %d/%d ids, want %d each", len(by50), len(by100), want)
	}
	for i := range by50 {
		if by50[i] != by100[i] {
			t.Fatalf("page size changes the walk at %d: %s vs %s", i, by50[i], by100[i])
		}
	}
}

// TestContactsIntegration_KeysetPages walks a 150-card book in portions of
// 50 (ticket #600, acceptance): every sort key and direction pages without
// duplicates or drops — the id sequence collected with 50-per-page windows
// must equal the one collected with 100-per-page windows — and the search
// filter pages the same way within its own matched scope.
func TestContactsIntegration_KeysetPages(t *testing.T) {
	t.Parallel()

	h := newContactsHarness(t)
	h.owner = h.seedUser()
	p1 := h.seedPropertyNamed(h.owner, "Моя квартира")
	p2 := h.seedPropertyNamed(h.owner, "Студия")

	// 100 unbound cards, 40 bound across two properties, and a tie group of
	// ten identical display names — the id tie-off is part of the keyset key.
	for i := 1; i <= 100; i++ {
		h.create(contactsapp.CreateContactCommand{
			FirstName: fmt.Sprintf("Контакт %03d", i),
		})
	}
	for i := 1; i <= 20; i++ {
		prop := p1
		h.create(contactsapp.CreateContactCommand{
			FirstName: fmt.Sprintf("Контакт 1%02d", i), PropertyID: &prop,
		})
	}
	for i := 1; i <= 20; i++ {
		prop := p2
		h.create(contactsapp.CreateContactCommand{
			FirstName: fmt.Sprintf("Контакт 2%02d", i), PropertyID: &prop,
		})
	}
	for range 10 {
		prop := p1
		h.create(contactsapp.CreateContactCommand{
			FirstName: "Двойник", PropertyID: &prop,
		})
	}

	for _, tc := range []struct {
		name  string
		sort  contactsapp.ListSort
		order contactsapp.ListOrder
	}{
		{"name ascending", contactsapp.ListSortName, contactsapp.ListOrderAsc},
		{"name descending", contactsapp.ListSortName, contactsapp.ListOrderDesc},
		{"property ascending", contactsapp.ListSortProperty, contactsapp.ListOrderAsc},
		{"property descending", contactsapp.ListSortProperty, contactsapp.ListOrderDesc},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base := contactsapp.ListQuery{
				Scope: contactsapp.ListScopeAll, Sort: tc.sort, Order: tc.order,
			}
			by50 := walkBook(t, h.svc, h.owner, base, 50)
			by100 := walkBook(t, h.svc, h.owner, base, 100)
			assertSameWalk(t, by50, by100, 150)
		})
	}

	t.Run("search pages within the matched scope", func(t *testing.T) {
		t.Parallel()
		// «Контакт» matches the numbered cards (140); the tie group and the
		// search term stay out of each other's way.
		base := contactsapp.ListQuery{Scope: contactsapp.ListScopeAll, Search: "Контакт"}
		by50 := walkBook(t, h.svc, h.owner, base, 50)
		by100 := walkBook(t, h.svc, h.owner, base, 100)
		assertSameWalk(t, by50, by100, 140)
	})
}
