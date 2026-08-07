package postgres

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// These integration tests run against a real Postgres via TEST_DATABASE_URL
// and are skipped when it is unset (same convention as the leases policy
// integration tests). They exercise the T3 policy enforcement (issue #166) in
// the free-reminder use cases end-to-end: real MembershipPolicy over the real
// repository, with the service called as owner, full-access member, viewer,
// outsider and suspended member.

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// policyNoCommitTx wraps the test's pgx.Tx as a transaction.Tx whose Commit
// and Rollback are no-ops: the test owns the transaction lifecycle.
type policyNoCommitTx struct{ pgx.Tx }

func (policyNoCommitTx) Commit(context.Context) error   { return nil }
func (policyNoCommitTx) Rollback(context.Context) error { return nil }

// policyBeginner always returns the test's already-open transaction.
type policyBeginner struct{ tx pgx.Tx }

func (b policyBeginner) Begin(context.Context) (transaction.Tx, error) {
	return policyNoCommitTx{Tx: b.tx}, nil
}

// policyTestClock is a fixed clock for the service under test.
type policyTestClock struct{ now time.Time }

func (c policyTestClock) Now() time.Time { return c.now }

// policyTestTzResolver resolves every owner's timezone as UTC.
type policyTestTzResolver struct{}

func (policyTestTzResolver) Resolve(context.Context, uuid.UUID) (*time.Location, error) {
	return time.UTC, nil
}

func setupPolicyDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func beginPolicyTx(t *testing.T, pool *pgxpool.Pool) (context.Context, pgx.Tx, func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	return ctx, tx, func() { _ = tx.Rollback(ctx) }
}

func createPolicyTestUser(t *testing.T, ctx context.Context, q *genpostgres.Queries) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	// The phone must be unique per call: derive it from the uuid's random tail.
	_, err = q.CreateUser(ctx, genpostgres.CreateUserParams{
		ID:    pgUUID(id),
		Phone: fmt.Sprintf("+7999%07d", binary.BigEndian.Uint32(id[12:])%10000000),
		Role:  "owner",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func createPolicyTestProperty(t *testing.T, ctx context.Context, q *genpostgres.Queries, owner uuid.UUID) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = q.CreateProperty(ctx, genpostgres.CreatePropertyParams{
		ID:          pgUUID(id),
		OwnerID:     pgUUID(owner),
		Name:        "Policy Test Property",
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

func addPolicyMembership(t *testing.T, ctx context.Context, repo *accesspg.MembershipRepository, property, user, grantedBy uuid.UUID, role accessdomain.Role) {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := repo.Create(ctx, accessdomain.Membership{
		ID: id, PropertyID: property, UserID: user, Role: role, GrantedBy: grantedBy,
	}); err != nil {
		t.Fatalf("create membership: %v", err)
	}
}

func addSuspendedPolicyMembership(t *testing.T, ctx context.Context, repo *accesspg.MembershipRepository, property, user, grantedBy uuid.UUID, role accessdomain.Role) {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := repo.CreateWithStatus(ctx, accessdomain.Membership{
		ID: id, PropertyID: property, UserID: user, Role: role, GrantedBy: grantedBy,
		Status: accessdomain.MemberStatusSuspended,
	}); err != nil {
		t.Fatalf("create suspended membership: %v", err)
	}
}

// freeReminderPolicyFixture wires the free-reminder service with the real
// repository bound to the test transaction and the real membership policy.
type freeReminderPolicyFixture struct {
	q        *genpostgres.Queries
	policy   *accessapp.MembershipPolicy
	members  *accesspg.MembershipRepository
	repo     *FreeReminderRepository
	beginner policyBeginner
	clock    policyTestClock
}

func newFreeReminderPolicyFixture(tx pgx.Tx) *freeReminderPolicyFixture {
	return &freeReminderPolicyFixture{
		q:        genpostgres.New(tx),
		policy:   accessapp.NewMembershipPolicy(accesspg.NewOwnerResolver(tx), accesspg.NewMembershipRepository(tx)),
		members:  accesspg.NewMembershipRepository(tx),
		repo:     NewFreeReminderRepository(tx),
		beginner: policyBeginner{tx: tx},
		clock:    policyTestClock{now: time.Now()},
	}
}

func (f *freeReminderPolicyFixture) service() *application.FreeReminderService {
	return application.NewFreeReminderService(f.repo, f.beginner, f.clock, policyTestTzResolver{}, f.policy)
}

func (f *freeReminderPolicyFixture) createInput(propertyID uuid.UUID) application.CreateFreeReminderInput {
	return application.CreateFreeReminderInput{
		PropertyID:  propertyID,
		Title:       "policy test",
		TriggerAt:   f.clock.now.Add(48 * time.Hour),
		Periodicity: domain.PeriodicityOnce,
	}
}

// assertFreeReminderOwner checks the DB row behind a free reminder: it must
// belong to the data owner regardless of who acted.
func (f *freeReminderPolicyFixture) assertFreeReminderOwner(t *testing.T, ctx context.Context, id, owner uuid.UUID) {
	t.Helper()
	row, err := f.q.GetFreeReminderByIDUnscoped(ctx, pgUUID(id))
	if err != nil {
		t.Fatalf("get free reminder row: %v", err)
	}
	if row.OwnerID.Bytes != owner {
		t.Errorf("free reminder owner_id: want %s, got %s", owner, row.OwnerID.Bytes)
	}
}

func TestFreeReminderPolicyIntegration_MemberOperatesOnOwnerScope(t *testing.T) {
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newFreeReminderPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	member := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addPolicyMembership(t, ctx, f.members, property, member, owner, accessdomain.RoleFullAccess)

	// The full-access member creates a reminder: the row lands on the owner's
	// scope, not on the actor's.
	created, err := f.service().Create(ctx, member, owner, f.createInput(property))
	if err != nil {
		t.Fatalf("Create as member: %v", err)
	}
	if created.OwnerID != owner {
		t.Errorf("created OwnerID: want owner %s, got %s", owner, created.OwnerID)
	}
	f.assertFreeReminderOwner(t, ctx, created.ID, owner)

	// Reads and writes by the member use the owner's scope.
	if _, err := f.service().Get(ctx, member, created.ID); err != nil {
		t.Fatalf("Get as member: %v", err)
	}
	if _, err := f.service().ListByProperty(ctx, owner, property, 3); err != nil {
		t.Fatalf("ListByProperty: %v", err)
	}
	title := "member edit"
	if _, err := f.service().Update(ctx, member, created.ID, application.UpdateFreeReminderInput{Title: &title}); err != nil {
		t.Fatalf("Update as member: %v", err)
	}
	if err := f.service().Delete(ctx, member, created.ID); err != nil {
		t.Fatalf("Delete as member: %v", err)
	}
}

func TestFreeReminderPolicyIntegration_ViewerReadsButCannotWrite(t *testing.T) {
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newFreeReminderPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	viewer := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addPolicyMembership(t, ctx, f.members, property, viewer, owner, accessdomain.RoleViewer)

	seeded, err := f.service().Create(ctx, owner, owner, f.createInput(property))
	if err != nil {
		t.Fatalf("seed free reminder: %v", err)
	}

	// Reads are allowed.
	if _, err := f.service().Get(ctx, viewer, seeded.ID); err != nil {
		t.Errorf("Get as viewer: %v", err)
	}

	// Writes are forbidden.
	if _, err := f.service().Create(ctx, viewer, owner, f.createInput(property)); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("Create as viewer: want ErrForbidden, got %v", err)
	}
	title := "viewer edit"
	if _, err := f.service().Update(ctx, viewer, seeded.ID, application.UpdateFreeReminderInput{Title: &title}); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("Update as viewer: want ErrForbidden, got %v", err)
	}
	if err := f.service().Delete(ctx, viewer, seeded.ID); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("Delete as viewer: want ErrForbidden, got %v", err)
	}
}

func TestFreeReminderPolicyIntegration_NoneAndSuspendedGetNotFound(t *testing.T) {
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newFreeReminderPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	outsider := createPolicyTestUser(t, ctx, f.q)
	suspended := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addSuspendedPolicyMembership(t, ctx, f.members, property, suspended, owner, accessdomain.RoleFullAccess)

	seeded, err := f.service().Create(ctx, owner, owner, f.createInput(property))
	if err != nil {
		t.Fatalf("seed free reminder: %v", err)
	}

	for name, actor := range map[string]uuid.UUID{"outsider": outsider, "suspended": suspended} {
		if _, err := f.service().Get(ctx, actor, seeded.ID); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("Get as %s: want ErrNotFound, got %v", name, err)
		}
		if _, err := f.service().Create(ctx, actor, owner, f.createInput(property)); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("Create as %s: want ErrNotFound, got %v", name, err)
		}
		title := "intruder edit"
		if _, err := f.service().Update(ctx, actor, seeded.ID, application.UpdateFreeReminderInput{Title: &title}); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("Update as %s: want ErrNotFound, got %v", name, err)
		}
		if err := f.service().Delete(ctx, actor, seeded.ID); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("Delete as %s: want ErrNotFound, got %v", name, err)
		}
	}
}
