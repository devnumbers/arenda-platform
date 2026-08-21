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
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// These integration tests run against a real Postgres via TEST_DATABASE_URL
// and are skipped when it is unset (same convention as the access integration
// tests). They exercise the T3 policy enforcement (issue #166) end-to-end:
// real MembershipPolicy over real repositories, with the services called as
// owner, full-access member, viewer, outsider and suspended member.

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// policyNoCommitTx wraps the test's pgx.Tx as a transaction.Tx whose Commit
// and Rollback are no-ops: the test owns the transaction lifecycle.
type policyNoCommitTx struct{ pgx.Tx }

func (policyNoCommitTx) Commit(context.Context) error   { return nil }
func (policyNoCommitTx) Rollback(context.Context) error { return nil }

// policyUoW adapts the test's already-open transaction to the
// transaction.UoW port so the services under test open their transactions
// through runInTx (ADR 0033); the no-commit tx keeps the outer test
// transaction in charge of cleanup.
type policyUoW struct{ tx pgx.Tx }

func (u policyUoW) Do(_ context.Context, work func(tx transaction.Tx) error) error {
	return work(policyNoCommitTx{Tx: u.tx})
}

// policyTestClock is a fixed clock for the services under test.
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
	// Tests run in parallel and each holds its own pool for a single rollback
	// transaction. The production DefaultPoolConfig keeps MinConns=16 warm per
	// pool; across the ~15 parallel fixtures that is hundreds of connections
	// against one PostgreSQL, so the test pools stay minimal.
	cfg := database.DefaultPoolConfig()
	cfg.MinConns = 0
	cfg.MaxConns = 2
	ctx := context.Background()
	pool, err := database.NewPoolWithConfig(ctx, databaseURL, cfg)
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
	return ctx, tx, func() {
		// Rollback failure means test isolation broke: rows written in the
		// aborted test transaction would persist in the shared database.
		if err := tx.Rollback(ctx); err != nil {
			t.Errorf("rollback leases test tx: %v", err)
		}
	}
}

func createPolicyTestUser(t *testing.T, ctx context.Context, q *genpostgres.Queries) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	// The phone must be unique per call: derive it from the uuid's random tail
	// (see createAccessTestUser for the collision rationale).
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

func addPolicyMembership(
	t *testing.T,
	ctx context.Context,
	repo *accesspg.MembershipRepository,
	property, user, grantedBy uuid.UUID,
	role accessdomain.Role,
) {
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

func addSuspendedPolicyMembership(
	t *testing.T,
	ctx context.Context,
	repo *accesspg.MembershipRepository,
	property, user, grantedBy uuid.UUID,
	role accessdomain.Role,
) {
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

// policyFixture wires the leases services with real repositories bound to the
// test transaction and the real membership policy.
type policyFixture struct {
	q       *genpostgres.Queries
	tx      pgx.Tx
	uow     policyUoW
	policy  *accessapp.MembershipPolicy
	members *accesspg.MembershipRepository
	ops     *OperationRepository
	leases  *LeaseRepository
	recs    *RecurringOperationRepository
	props   *PropertyRepository
	cats    *OperationCategoryRepository
	clock   policyTestClock
}

func newPolicyFixture(tx pgx.Tx) *policyFixture {
	return &policyFixture{
		q:       genpostgres.New(tx),
		tx:      tx,
		uow:     policyUoW{tx: tx},
		policy:  accessapp.NewMembershipPolicy(accesspg.NewOwnerResolver(tx), accesspg.NewMembershipRepository(tx)),
		members: accesspg.NewMembershipRepository(tx),
		ops:     NewOperationRepository(tx),
		leases:  NewLeaseRepository(tx),
		recs:    NewRecurringOperationRepository(tx),
		props:   NewPropertyRepository(tx),
		cats:    NewOperationCategoryRepository(tx),
		clock:   policyTestClock{now: time.Now()},
	}
}

func (f *policyFixture) operationService() *application.OperationService {
	factory := application.NewTxStoreFactory(f.leases, f.props, nil, f.recs, f.ops, f.cats, nil, nil, f.uow)
	return application.NewOperationService(f.ops, f.props, f.leases, f.cats, factory, f.clock, policyTestTzResolver{}, f.policy, nil)
}

// operationServiceWithShared mirrors the production wiring (main.go) where the
// SharedProperties adapter is injected into OperationService, so aggregate
// reads (ListOperations) fold in the actor's shared properties (issue #157).
func (f *policyFixture) operationServiceWithShared() *application.OperationService {
	factory := application.NewTxStoreFactory(f.leases, f.props, nil, f.recs, f.ops, f.cats, nil, nil, f.uow)
	svc := application.NewOperationService(f.ops, f.props, f.leases, f.cats, factory, f.clock, policyTestTzResolver{}, f.policy, nil)
	svc.SetSharedPropertyIDs(accesspg.NewSharedProperties(f.tx))
	return svc
}

func (f *policyFixture) leaseService() *application.LeaseService {
	factory := application.NewTxStoreFactory(f.leases, f.props, nil, f.recs, f.ops, f.cats, nil, nil, f.uow)
	return application.NewLeaseService(f.leases, f.props, nil, f.cats, factory, f.clock, policyTestTzResolver{}, f.policy, nil)
}

// leaseServiceWithShared mirrors the production wiring where the
// SharedProperties adapter is injected into LeaseService, so the lease payment-
// schedule reads fold in the actor's shared properties (issue #157).
func (f *policyFixture) leaseServiceWithShared() *application.LeaseService {
	factory := application.NewTxStoreFactory(f.leases, f.props, nil, f.recs, f.ops, f.cats, nil, nil, f.uow)
	svc := application.NewLeaseService(f.leases, f.props, nil, f.cats, factory, f.clock, policyTestTzResolver{}, f.policy, nil)
	svc.SetSharedPropertyIDs(accesspg.NewSharedProperties(f.tx))
	return svc
}

func (f *policyFixture) recurringService() *application.RecurringOperationService {
	factory := application.NewTxStoreFactory(f.leases, f.props, nil, f.recs, f.ops, f.cats, nil, nil, f.uow)
	return application.NewRecurringOperationService(
		f.recs, f.ops, f.props, f.cats, nil, factory, f.clock, policyTestTzResolver{}, f.policy, nil)
}

// expenseCategoryID returns a seeded default expense category of the owner.
func (f *policyFixture) expenseCategoryID(t *testing.T, ctx context.Context, owner uuid.UUID) uuid.UUID {
	t.Helper()
	expenseType := domain.OperationTypeExpense
	cats, err := f.cats.ListByOwner(ctx, owner, &expenseType)
	if err != nil || len(cats) == 0 {
		t.Fatalf("list expense categories: %v (n=%d)", err, len(cats))
	}
	return cats[0].ID
}

func (f *policyFixture) createOperationCmd(propertyID, categoryID uuid.UUID) application.CreateOperationCommand {
	return application.CreateOperationCommand{
		PropertyID:    propertyID,
		Type:          "expense",
		CategoryID:    categoryID,
		Name:          "policy test",
		AmountKopecks: 1000,
		OperationDate: time.Now(),
	}
}

func (f *policyFixture) createLeaseCmd(propertyID uuid.UUID) application.CreateLeaseCommand {
	return application.CreateLeaseCommand{
		PropertyID:        propertyID,
		StartDate:         time.Now(),
		RentAmountKopecks: 10000,
		PaymentDay:        1,
	}
}

func (f *policyFixture) createRecurringCmd(propertyID, categoryID uuid.UUID) application.CreateRecurringOperationCommand {
	return application.CreateRecurringOperationCommand{
		PropertyID:    propertyID,
		Type:          "expense",
		CategoryID:    categoryID,
		Name:          "policy test",
		AmountKopecks: 1000,
		StartDate:     time.Now(),
		PaymentDay:    1,
	}
}

// assertOperationOwner checks the DB row behind an operation: it must belong
// to the data owner regardless of who acted.
func (f *policyFixture) assertOperationOwner(t *testing.T, ctx context.Context, opID, owner uuid.UUID) {
	t.Helper()
	row, err := f.q.GetOperationByID(ctx, pgUUID(opID))
	if err != nil {
		t.Fatalf("get operation row: %v", err)
	}
	if row.OwnerID.Bytes != owner {
		t.Errorf("operation owner_id: want %s, got %s", owner, row.OwnerID.Bytes)
	}
}

func TestPolicyIntegration_FullAccessMemberOperatesOnOwnerScope(t *testing.T) {
	t.Parallel()
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	member := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	if err := f.cats.CreateDefaultCategories(ctx, owner); err != nil {
		t.Fatalf("seed categories: %v", err)
	}
	categoryID := f.expenseCategoryID(t, ctx, owner)
	addPolicyMembership(t, ctx, f.members, property, member, owner, accessdomain.RoleFullAccess)

	// Create / read / update / delete an operation as the full-access member:
	// every row stays on the owner's scope.
	created, err := f.operationService().CreateOperation(ctx, member, f.createOperationCmd(property, categoryID))
	if err != nil {
		t.Fatalf("CreateOperation as member: %v", err)
	}
	if created.OwnerID != owner {
		t.Errorf("created OwnerID: want owner %s, got %s", owner, created.OwnerID)
	}
	f.assertOperationOwner(t, ctx, created.ID, owner)

	if _, err := f.operationService().GetOperation(ctx, member, created.ID); err != nil {
		t.Fatalf("GetOperation as member: %v", err)
	}
	if _, err := f.operationService().ListOperationsByProperty(ctx, member, property, application.OperationFilter{}); err != nil {
		t.Fatalf("ListOperationsByProperty as member: %v", err)
	}
	name := "member edit"
	if _, err := f.operationService().UpdateOperation(ctx, member, created.ID, application.UpdateOperationCommand{Name: &name}); err != nil {
		t.Fatalf("UpdateOperation as member: %v", err)
	}
	if err := f.operationService().DeleteOperation(ctx, member, created.ID); err != nil {
		t.Fatalf("DeleteOperation as member: %v", err)
	}

	// Lease and recurring operation creation land on the owner's scope too.
	lease, err := f.leaseService().CreateLease(ctx, member, f.createLeaseCmd(property))
	if err != nil {
		t.Fatalf("CreateLease as member: %v", err)
	}
	if lease.OwnerID != owner {
		t.Errorf("lease OwnerID: want owner %s, got %s", owner, lease.OwnerID)
	}
	if got, err := f.leaseService().GetLease(ctx, member, lease.ID); err != nil || got.ID != lease.ID {
		t.Fatalf("GetLease as member: %v", err)
	}

	rec, err := f.recurringService().CreateRecurringOperation(ctx, member, f.createRecurringCmd(property, categoryID))
	if err != nil {
		t.Fatalf("CreateRecurringOperation as member: %v", err)
	}
	if rec.OwnerID != owner {
		t.Errorf("recurring OwnerID: want owner %s, got %s", owner, rec.OwnerID)
	}
	if _, err := f.recurringService().GetRecurringOperation(ctx, member, rec.ID); err != nil {
		t.Fatalf("GetRecurringOperation as member: %v", err)
	}
}

func TestPolicyIntegration_ViewerReadsButCannotWrite(t *testing.T) {
	t.Parallel()
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	viewer := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	if err := f.cats.CreateDefaultCategories(ctx, owner); err != nil {
		t.Fatalf("seed categories: %v", err)
	}
	categoryID := f.expenseCategoryID(t, ctx, owner)
	addPolicyMembership(t, ctx, f.members, property, viewer, owner, accessdomain.RoleViewer)

	// Seed data as the owner.
	op, err := f.operationService().CreateOperation(ctx, owner, f.createOperationCmd(property, categoryID))
	if err != nil {
		t.Fatalf("seed operation: %v", err)
	}
	lease, err := f.leaseService().CreateLease(ctx, owner, f.createLeaseCmd(property))
	if err != nil {
		t.Fatalf("seed lease: %v", err)
	}
	rec, err := f.recurringService().CreateRecurringOperation(ctx, owner, f.createRecurringCmd(property, categoryID))
	if err != nil {
		t.Fatalf("seed recurring: %v", err)
	}

	// Reads are allowed.
	if _, err := f.operationService().GetOperation(ctx, viewer, op.ID); err != nil {
		t.Errorf("GetOperation as viewer: %v", err)
	}
	if _, err := f.operationService().ListOperationsByProperty(ctx, viewer, property, application.OperationFilter{}); err != nil {
		t.Errorf("ListOperationsByProperty as viewer: %v", err)
	}
	if _, err := f.leaseService().GetLease(ctx, viewer, lease.ID); err != nil {
		t.Errorf("GetLease as viewer: %v", err)
	}
	if _, err := f.recurringService().GetRecurringOperation(ctx, viewer, rec.ID); err != nil {
		t.Errorf("GetRecurringOperation as viewer: %v", err)
	}

	// Writes are forbidden.
	if _, err := f.operationService().CreateOperation(
		ctx, viewer, f.createOperationCmd(property, categoryID),
	); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("CreateOperation as viewer: want ErrForbidden, got %v", err)
	}
	name := "viewer edit"
	if _, err := f.operationService().UpdateOperation(
		ctx, viewer, op.ID, application.UpdateOperationCommand{Name: &name},
	); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("UpdateOperation as viewer: want ErrForbidden, got %v", err)
	}
	if err := f.operationService().DeleteOperation(ctx, viewer, op.ID); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("DeleteOperation as viewer: want ErrForbidden, got %v", err)
	}
	if _, err := f.leaseService().CreateLease(ctx, viewer, f.createLeaseCmd(property)); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("CreateLease as viewer: want ErrForbidden, got %v", err)
	}
	if err := f.recurringService().DeleteRecurringOperation(ctx, viewer, rec.ID); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("DeleteRecurringOperation as viewer: want ErrForbidden, got %v", err)
	}
}

func TestPolicyIntegration_NoneAndSuspendedGetNotFound(t *testing.T) {
	t.Parallel()
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	outsider := createPolicyTestUser(t, ctx, f.q)
	suspended := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	if err := f.cats.CreateDefaultCategories(ctx, owner); err != nil {
		t.Fatalf("seed categories: %v", err)
	}
	categoryID := f.expenseCategoryID(t, ctx, owner)
	addSuspendedPolicyMembership(t, ctx, f.members, property, suspended, owner, accessdomain.RoleFullAccess)

	op, err := f.operationService().CreateOperation(ctx, owner, f.createOperationCmd(property, categoryID))
	if err != nil {
		t.Fatalf("seed operation: %v", err)
	}

	for name, actor := range map[string]uuid.UUID{"outsider": outsider, "suspended": suspended} {
		if _, err := f.operationService().GetOperation(
			ctx, actor, op.ID,
		); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("GetOperation as %s: want ErrNotFound, got %v", name, err)
		}
		if _, err := f.operationService().ListOperationsByProperty(
			ctx, actor, property, application.OperationFilter{},
		); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("ListOperationsByProperty as %s: want ErrNotFound, got %v", name, err)
		}
		newName := "intruder edit"
		if _, err := f.operationService().UpdateOperation(
			ctx, actor, op.ID, application.UpdateOperationCommand{Name: &newName},
		); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("UpdateOperation as %s: want ErrNotFound, got %v", name, err)
		}
		if err := f.operationService().DeleteOperation(ctx, actor, op.ID); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("DeleteOperation as %s: want ErrNotFound, got %v", name, err)
		}
		if _, err := f.operationService().CreateOperation(
			ctx, actor, f.createOperationCmd(property, categoryID),
		); !errors.Is(err, application.ErrNotFound) {
			t.Errorf("CreateOperation as %s: want ErrNotFound, got %v", name, err)
		}
	}
}

func TestPolicyIntegration_StandaloneReads(t *testing.T) {
	t.Parallel()
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	member := createPolicyTestUser(t, ctx, f.q)
	outsider := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addPolicyMembership(t, ctx, f.members, property, member, owner, accessdomain.RoleFullAccess)
	if err := f.cats.CreateDefaultCategories(ctx, owner); err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	lease, err := f.leaseService().CreateLease(ctx, owner, f.createLeaseCmd(property))
	if err != nil {
		t.Fatalf("seed lease: %v", err)
	}

	// The member reads the standalone lease and the property operations
	// summary (AC6, issue #166).
	if _, err := f.leaseService().GetLease(ctx, member, lease.ID); err != nil {
		t.Errorf("GetLease as member: %v", err)
	}
	if _, err := f.operationService().GetPropertyOperationsSummary(ctx, member, property); err != nil {
		t.Errorf("GetPropertyOperationsSummary as member: %v", err)
	}

	// The outsider gets ErrNotFound for both, so the object's existence is
	// never revealed.
	if _, err := f.leaseService().GetLease(ctx, outsider, lease.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("GetLease as outsider: want ErrNotFound, got %v", err)
	}
	if _, err := f.operationService().GetPropertyOperationsSummary(
		ctx, outsider, property,
	); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("GetPropertyOperationsSummary as outsider: want ErrNotFound, got %v", err)
	}
}

func TestPolicyIntegration_MoveOperation(t *testing.T) {
	t.Parallel()
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	member := createPolicyTestUser(t, ctx, f.q)
	other := createPolicyTestUser(t, ctx, f.q)
	source := createPolicyTestProperty(t, ctx, f.q, owner)
	target := createPolicyTestProperty(t, ctx, f.q, owner)
	foreign := createPolicyTestProperty(t, ctx, f.q, other)
	if err := f.cats.CreateDefaultCategories(ctx, owner); err != nil {
		t.Fatalf("seed categories: %v", err)
	}
	categoryID := f.expenseCategoryID(t, ctx, owner)
	// The member has full access everywhere — including the foreign property —
	// so the cross-owner rejection below is caused by the owner mismatch, not
	// by a missing role.
	addPolicyMembership(t, ctx, f.members, source, member, owner, accessdomain.RoleFullAccess)
	addPolicyMembership(t, ctx, f.members, target, member, owner, accessdomain.RoleFullAccess)
	addPolicyMembership(t, ctx, f.members, foreign, member, other, accessdomain.RoleFullAccess)

	// Happy path: the member moves an operation between the owner's
	// properties; the row keeps the owner's scope.
	op, err := f.operationService().CreateOperation(ctx, member, f.createOperationCmd(source, categoryID))
	if err != nil {
		t.Fatalf("seed operation: %v", err)
	}
	moved, err := f.operationService().MoveOperation(ctx, member, op.ID, application.MoveOperationCommand{PropertyID: target})
	if err != nil {
		t.Fatalf("MoveOperation as member: %v", err)
	}
	if moved.PropertyID != target {
		t.Errorf("moved PropertyID: want %s, got %s", target, moved.PropertyID)
	}
	f.assertOperationOwner(t, ctx, op.ID, owner)

	// Cross-owner move is rejected with a validation error even though the
	// member can edit the foreign property.
	op2, err := f.operationService().CreateOperation(ctx, member, f.createOperationCmd(source, categoryID))
	if err != nil {
		t.Fatalf("seed second operation: %v", err)
	}
	if _, err := f.operationService().MoveOperation(
		ctx, member, op2.ID, application.MoveOperationCommand{PropertyID: foreign},
	); !errors.Is(err, application.ErrInvalidInput) {
		t.Errorf("cross-owner move: want ErrInvalidInput, got %v", err)
	}
	if _, err := f.operationService().MoveOperation(
		ctx, member, op2.ID, application.MoveOperationCommand{PropertyID: source},
	); !errors.Is(err, application.ErrInvalidInput) {
		t.Errorf("same-property move: want ErrInvalidInput, got %v", err)
	}

	// An operation generated by a recurring operation cannot be moved.
	rec, err := f.recurringService().CreateRecurringOperation(ctx, owner, f.createRecurringCmd(source, categoryID))
	if err != nil {
		t.Fatalf("seed recurring: %v", err)
	}
	childID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	now := time.Now()
	child, err := f.ops.Create(ctx, domain.Operation{
		ID:                   childID,
		OwnerID:              owner,
		PropertyID:           source,
		RecurringOperationID: rec.ID,
		Type:                 domain.OperationTypeExpense,
		CategoryID:           categoryID,
		Status:               domain.OperationStatusPending,
		Name:                 "generated child",
		AmountKopecks:        1000,
		// A date far beyond the recurring generation horizon: the
		// (recurring_operation_id, operation_date) unique index must not
		// collide with an already generated operation.
		OperationDate: now.AddDate(10, 0, 0),
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("seed recurring child: %v", err)
	}
	if _, err := f.operationService().MoveOperation(
		ctx, owner, child.ID, application.MoveOperationCommand{PropertyID: target},
	); !errors.Is(err, application.ErrInvalidInput) {
		t.Errorf("recurring-child move: want ErrInvalidInput, got %v", err)
	}
}

// TestPolicyIntegration_TenantContactsOwnerWideAccess exercises the owner-wide
// enforcement on tenant contacts (Property Sharing follow-up) end-to-end: a
// full-access member creates and updates contacts in the owner's scope, a
// viewer reads but cannot write, and an outsider gets ErrNotFound.
func TestPolicyIntegration_TenantContactsOwnerWideAccess(t *testing.T) {
	t.Parallel()
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	member := createPolicyTestUser(t, ctx, f.q)
	viewer := createPolicyTestUser(t, ctx, f.q)
	outsider := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addPolicyMembership(t, ctx, f.members, property, member, owner, accessdomain.RoleFullAccess)
	addPolicyMembership(t, ctx, f.members, property, viewer, owner, accessdomain.RoleViewer)

	contactRepo := NewTenantContactRepository(tx)
	contactService := func() *application.TenantContactService {
		svc := application.NewTenantContactService(contactRepo, nil, nil)
		svc.SetPolicy(f.policy)
		svc.SetProperties(f.props)
		return svc
	}

	// The member creates a contact in the property context: it lands on the
	// owner's scope.
	phone := "+79160000011"
	memberCreated, err := contactService().CreateTenantContact(ctx, member, application.CreateTenantContactCommand{
		Name: "Member Contact", Phone: &phone, PropertyID: &property,
	})
	if err != nil {
		t.Fatalf("CreateTenantContact as member: %v", err)
	}
	if memberCreated.OwnerID != owner {
		t.Errorf("created OwnerID: want owner %s, got %s", owner, memberCreated.OwnerID)
	}

	// Reads: member and viewer resolve the owner's contact by id; the outsider
	// gets ErrNotFound.
	for name, actor := range map[string]uuid.UUID{"owner": owner, "member": member, "viewer": viewer} {
		if _, err := contactService().GetTenantContact(ctx, actor, memberCreated.ID); err != nil {
			t.Errorf("GetTenantContact as %s: %v", name, err)
		}
	}
	if _, err := contactService().GetTenantContact(
		ctx, outsider, memberCreated.ID,
	); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("GetTenantContact as outsider: want ErrNotFound, got %v", err)
	}

	// Writes: the member updates the owner's contact; the viewer gets
	// ErrForbidden, the outsider ErrNotFound.
	newName := "Member Contact Renamed"
	updated, err := contactService().UpdateTenantContact(ctx, member, memberCreated.ID, application.UpdateTenantContactCommand{Name: &newName})
	if err != nil {
		t.Fatalf("UpdateTenantContact as member: %v", err)
	}
	if updated.OwnerID != owner {
		t.Errorf("updated OwnerID: want owner %s, got %s", owner, updated.OwnerID)
	}
	viewerName := "Viewer Rename"
	if _, err := contactService().UpdateTenantContact(
		ctx, viewer, memberCreated.ID, application.UpdateTenantContactCommand{Name: &viewerName},
	); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("UpdateTenantContact as viewer: want ErrForbidden, got %v", err)
	}
	outsiderName := "Outsider Rename"
	if _, err := contactService().UpdateTenantContact(
		ctx, outsider, memberCreated.ID, application.UpdateTenantContactCommand{Name: &outsiderName},
	); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("UpdateTenantContact as outsider: want ErrNotFound, got %v", err)
	}
}

// TestPolicyIntegration_ListOperationsByLease_IncludesShared reproduces the bug
// where a member opening a shared property's lease card sees no rent
// operations: the lease detail page loads operations via the aggregate
// ListOperations (GET /operations?lease_id=...), which — before the shared-
// property fix — filtered strictly by owner_id = actor, hiding the owner's
// rent operations from the member.
func TestPolicyIntegration_ListOperationsByLease_IncludesShared(t *testing.T) {
	t.Parallel()
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	member := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addPolicyMembership(t, ctx, f.members, property, member, owner, accessdomain.RoleFullAccess)
	if err := f.cats.CreateDefaultCategories(ctx, owner); err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	// CreateLease generates rent operations on the owner's scope.
	lease, err := f.leaseService().CreateLease(ctx, owner, f.createLeaseCmd(property))
	if err != nil {
		t.Fatalf("seed lease: %v", err)
	}

	// The owner sees the lease's rent operations via the aggregate read.
	ownerOps, err := f.operationServiceWithShared().ListOperations(ctx, owner, application.OperationFilter{LeaseID: lease.ID})
	if err != nil {
		t.Fatalf("ListOperations as owner: %v", err)
	}
	if len(ownerOps) == 0 {
		t.Fatalf("owner sees no operations for lease — test seed is broken")
	}

	// The member — who has full access to the shared property — must see the
	// same rent operations when opening the lease card (GET /operations?lease_id).
	memberOps, err := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{LeaseID: lease.ID})
	if err != nil {
		t.Fatalf("ListOperations as member: %v", err)
	}
	if len(memberOps) != len(ownerOps) {
		t.Errorf("member sees %d operations for shared lease, owner sees %d — member should see the same rent operations",
			len(memberOps), len(ownerOps))
	}
}

// TestPolicyIntegration_LeasePaymentSchedule_IncludesShared reproduces the bug
// where the lease card (GET /leases/{id}) shows a shared lease without its
// payment schedule: LeasePaymentScheduleIndex reads overdue/next rent
// operations scoped by owner_id = actor, so a member viewing the owner's lease
// gets an empty schedule (no overdue, no next payment) and the lease is shown
// incorrectly.
func TestPolicyIntegration_LeasePaymentSchedule_IncludesShared(t *testing.T) {
	t.Parallel()
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	member := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addPolicyMembership(t, ctx, f.members, property, member, owner, accessdomain.RoleFullAccess)
	if err := f.cats.CreateDefaultCategories(ctx, owner); err != nil {
		t.Fatalf("seed categories: %v", err)
	}

	lease, err := f.leaseService().CreateLease(ctx, owner, f.createLeaseCmd(property))
	if err != nil {
		t.Fatalf("seed lease: %v", err)
	}

	// The owner's schedule is the baseline — CreateLease seeds pending rent
	// operations, so NextPaymentDate must be present.
	ownerSchedule, err := f.leaseServiceWithShared().LeasePaymentScheduleIndex(ctx, owner, []domain.Lease{lease}, f.clock.Now())
	if err != nil {
		t.Fatalf("schedule as owner: %v", err)
	}
	ownerSched, ok := ownerSchedule[lease.ID]
	if !ok || ownerSched.NextPaymentDate == nil {
		t.Fatalf("owner schedule missing next payment — test seed is broken (ok=%v, sched=%+v)", ok, ownerSched)
	}

	// The member must see the same schedule for the shared lease.
	memberSchedule, err := f.leaseServiceWithShared().LeasePaymentScheduleIndex(ctx, member, []domain.Lease{lease}, f.clock.Now())
	if err != nil {
		t.Fatalf("schedule as member: %v", err)
	}
	memberSched, ok := memberSchedule[lease.ID]
	if !ok || memberSched.NextPaymentDate == nil {
		t.Errorf("member schedule for shared lease is empty (ok=%v, sched=%+v) — should match owner's (next=%v)",
			ok, memberSched, ownerSched.NextPaymentDate)
	}
}
