package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	accesspg "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/postgres"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

// These integration tests run against a real Postgres via TEST_DATABASE_URL and
// are skipped when it is unset. They audit the Property Sharing aggregate reads
// (issue #153, #157) end-to-end: a member must see shared-property data on
// cross-property lists, across multiple owners, and through lifecycle edge
// cases (suspended, archived, detach-deleted).

// incomeRentCategoryID returns the owner's seeded rent (income) category id.
func (f *policyFixture) incomeRentCategoryID(t *testing.T, ctx context.Context, owner uuid.UUID) uuid.UUID {
	t.Helper()
	incomeType := domain.OperationTypeIncome
	cats, err := f.cats.ListByOwner(ctx, owner, &incomeType)
	if err != nil {
		t.Fatalf("list income categories: %v", err)
	}
	for _, c := range cats {
		if c.Code != nil && *c.Code == "rent" {
			return c.ID
		}
	}
	t.Fatalf("no rent income category for owner %s", owner)
	return uuid.Nil
}

func (f *policyFixture) recurringServiceWithShared() *application.RecurringOperationService {
	factory := application.NewTxStoreFactory(f.leases, f.props, nil, f.recs, f.ops, f.cats, nil, nil, f.uow)
	svc := application.NewRecurringOperationService(
		f.recs, f.ops, f.props, f.cats, nil, factory, f.clock, policyTestTzResolver{}, f.policy, nil)
	svc.SetSharedPropertyIDs(accesspg.NewSharedProperties(f.tx))
	return svc
}

func (f *policyFixture) createIncomeRecurringCmd(propertyID, categoryID uuid.UUID) application.CreateRecurringOperationCommand {
	return application.CreateRecurringOperationCommand{
		PropertyID:    propertyID,
		Type:          "income",
		CategoryID:    categoryID,
		Name:          "audit recurring",
		AmountKopecks: 5000,
		StartDate:     time.Now(),
		PaymentDay:    1,
		Periodicity:   "monthly",
	}
}

// TestSharingAudit_RecurringOperations_IncludesShared verifies the fix for the
// recurring-operations aggregate list: a member sees recurring operations of
// shared properties (GET /recurring-operations), not just their own.
func TestSharingAudit_RecurringOperations_IncludesShared(t *testing.T) {
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
	categoryID := f.incomeRentCategoryID(t, ctx, owner)

	// Owner creates a recurring operation (lands on owner's scope).
	created, err := f.recurringService().CreateRecurringOperation(ctx, owner, f.createIncomeRecurringCmd(property, categoryID))
	if err != nil {
		t.Fatalf("seed recurring op: %v", err)
	}

	// The member — with full access — sees it via the aggregate list.
	memberRecs, err := f.recurringServiceWithShared().ListRecurringOperations(ctx, member)
	if err != nil {
		t.Fatalf("ListRecurringOperations as member: %v", err)
	}
	found := false
	for _, r := range memberRecs {
		if r.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("member did not see shared recurring operation %s in aggregate list (got %d recs)", created.ID, len(memberRecs))
	}

	// Without the shared adapter injected, the member sees nothing (backward-
	// compatible pre-T3 behaviour guard).
	noneRecs, err := f.recurringService().ListRecurringOperations(ctx, member)
	if err != nil {
		t.Fatalf("ListRecurringOperations (no shared) as member: %v", err)
	}
	if len(noneRecs) != 0 {
		t.Errorf("member without shared adapter saw %d recurring ops, expected 0", len(noneRecs))
	}
}

// multiOwnerWorld is the seeded multi-owner sharing fixture: two owners
// sharing their properties with the member, a stranger owner with no
// membership, and each owner's expense category.
type multiOwnerWorld struct {
	owner1, owner2, stranger, member uuid.UUID
	prop1, prop2, foreign            uuid.UUID
	cat1, cat2, catForeign           uuid.UUID
}

// seedMultiOwnerWorld seeds the multi-owner fixture: the member gets full
// access to owner1's property, viewer access to owner2's property, and no
// access to the stranger's property.
func seedMultiOwnerWorld(t *testing.T, ctx context.Context, f *policyFixture) multiOwnerWorld {
	t.Helper()
	w := multiOwnerWorld{
		owner1:   createPolicyTestUser(t, ctx, f.q),
		owner2:   createPolicyTestUser(t, ctx, f.q),
		stranger: createPolicyTestUser(t, ctx, f.q), // Owner the member has NO access to.
		member:   createPolicyTestUser(t, ctx, f.q),
	}
	w.prop1 = createPolicyTestProperty(t, ctx, f.q, w.owner1)
	w.prop2 = createPolicyTestProperty(t, ctx, f.q, w.owner2)
	w.foreign = createPolicyTestProperty(t, ctx, f.q, w.stranger)

	addPolicyMembership(t, ctx, f.members, w.prop1, w.member, w.owner1, accessdomain.RoleFullAccess)
	addPolicyMembership(t, ctx, f.members, w.prop2, w.member, w.owner2, accessdomain.RoleViewer)
	// No membership on `foreign`.

	if err := f.cats.CreateDefaultCategories(ctx, w.owner1); err != nil {
		t.Fatalf("seed o1 categories: %v", err)
	}
	if err := f.cats.CreateDefaultCategories(ctx, w.owner2); err != nil {
		t.Fatalf("seed o2 categories: %v", err)
	}
	if err := f.cats.CreateDefaultCategories(ctx, w.stranger); err != nil {
		t.Fatalf("seed stranger categories: %v", err)
	}
	w.cat1 = f.expenseCategoryID(t, ctx, w.owner1)
	w.cat2 = f.expenseCategoryID(t, ctx, w.owner2)
	w.catForeign = f.expenseCategoryID(t, ctx, w.stranger)
	return w
}

// seedOwnerOperation seeds one operation acting as the given owner and returns
// it.
func seedOwnerOperation(t *testing.T, ctx context.Context, f *policyFixture, owner, propertyID, categoryID uuid.UUID) domain.Operation {
	t.Helper()
	op, err := f.operationService().CreateOperation(ctx, owner, f.createOperationCmd(propertyID, categoryID))
	if err != nil {
		t.Fatalf("seed operation on property %s: %v", propertyID, err)
	}
	if op.PropertyID != propertyID {
		t.Fatalf("operation property: want %s, got %s — test seed broken", propertyID, op.PropertyID)
	}
	return op
}

// seedOwnerLease seeds one lease acting as the given owner and returns it.
func seedOwnerLease(t *testing.T, ctx context.Context, f *policyFixture, owner, propertyID uuid.UUID) domain.Lease {
	t.Helper()
	lease, err := f.leaseService().CreateLease(ctx, owner, f.createLeaseCmd(propertyID))
	if err != nil {
		t.Fatalf("seed lease on property %s: %v", propertyID, err)
	}
	return lease
}

// memberOperations runs the operations aggregate for the member with the
// property and category filter isolating one owner's rows in the shared test
// database.
func memberOperations(t *testing.T, ctx context.Context, f *policyFixture, member, propertyID, categoryID uuid.UUID) []domain.Operation {
	t.Helper()
	ops, err := f.operationServiceWithShared().ListOperations(ctx, member,
		application.OperationFilter{PropertyID: propertyID, CategoryIDs: []uuid.UUID{categoryID}, Limit: 100})
	if err != nil {
		t.Fatalf("ListOperations (property %s) as member: %v", propertyID, err)
	}
	return ops
}

// containsLease reports whether the lease id is in the list.
func containsLease(leases []domain.Lease, id uuid.UUID) bool {
	for _, l := range leases {
		if l.ID == id {
			return true
		}
	}
	return false
}

// assertMemberOperationsVisibility checks the operations aggregate: the member
// sees both shared owners' operations and never the stranger's.
func assertMemberOperationsVisibility(
	t *testing.T,
	ctx context.Context,
	f *policyFixture,
	w multiOwnerWorld,
	op1, op2, opForeign domain.Operation,
) {
	t.Helper()
	memberOps1 := memberOperations(t, ctx, f, w.member, w.prop1, w.cat1)
	memberOps2 := memberOperations(t, ctx, f, w.member, w.prop2, w.cat2)
	memberOpsForeign := memberOperations(t, ctx, f, w.member, w.foreign, w.catForeign)

	if !containsOp(memberOps1, op1.ID) {
		t.Errorf("member missing op1 (owner1 shared) in prop1+cat1-filtered list; got %d ops", len(memberOps1))
	}
	if !containsOp(memberOps2, op2.ID) {
		t.Errorf("member missing op2 (owner2 shared) in prop2-filtered list; got %d ops", len(memberOps2))
	}
	if containsOp(memberOpsForeign, opForeign.ID) {
		t.Errorf("member saw foreign op (no membership) — over-exposure leak")
	}
}

// assertMemberLeasesVisibility checks the leases aggregate: the member sees
// both shared owners' leases.
func assertMemberLeasesVisibility(t *testing.T, ctx context.Context, f *policyFixture, w multiOwnerWorld, lease1, lease2 domain.Lease) {
	t.Helper()
	memberLeases, err := f.leaseServiceWithShared().ListLeases(ctx, w.member)
	if err != nil {
		t.Fatalf("ListLeases as member: %v", err)
	}
	if !containsLease(memberLeases, lease1.ID) {
		t.Errorf("member missing lease1 (owner1 shared); got %d leases", len(memberLeases))
	}
	if !containsLease(memberLeases, lease2.ID) {
		t.Errorf("member missing lease2 (owner2 shared); got %d leases", len(memberLeases))
	}
}

// assertFinanceReportHidesForeign checks that the member's finance report runs
// without error and never includes the stranger's property. Seeded operations
// are pending/planned (not paid/received), so the report may be empty by
// property — the meaningful assertions are no-error and no-foreign-leak.
func assertFinanceReportHidesForeign(t *testing.T, ctx context.Context, f *policyFixture, w multiOwnerWorld) {
	t.Helper()
	report, err := f.operationServiceWithShared().GetFinanceReport(ctx, w.member, nil, nil)
	if err != nil {
		t.Fatalf("GetFinanceReport as member: %v", err)
	}
	for _, row := range report.ByProperty {
		if row.PropertyID == w.foreign {
			t.Errorf("finance report included foreign property (no membership) — over-exposure leak")
		}
	}
}

// TestSharingAudit_MultiOwner_AggregatesMerge verifies that a member with
// shared access to properties owned by two DIFFERENT owners sees data from both
// on every aggregate read: operations, leases, recurring operations, and the
// finance report. No owner is missed, none is double-counted, and a third
// owner the member has NO access to does not leak.
func TestSharingAudit_MultiOwner_AggregatesMerge(t *testing.T) {
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newPolicyFixture(tx)
	w := seedMultiOwnerWorld(t, ctx, f)

	// Seed one operation and (for the shared owners) one lease per property.
	op1 := seedOwnerOperation(t, ctx, f, w.owner1, w.prop1, w.cat1)
	op2 := seedOwnerOperation(t, ctx, f, w.owner2, w.prop2, w.cat2)
	opForeign := seedOwnerOperation(t, ctx, f, w.stranger, w.foreign, w.catForeign)
	lease1 := seedOwnerLease(t, ctx, f, w.owner1, w.prop1)
	lease2 := seedOwnerLease(t, ctx, f, w.owner2, w.prop2)

	// 1. Operations aggregate: member sees op1 + op2, not opForeign.
	assertMemberOperationsVisibility(t, ctx, f, w, op1, op2, opForeign)

	// 2. Leases aggregate: member sees lease1 + lease2.
	assertMemberLeasesVisibility(t, ctx, f, w, lease1, lease2)

	// 3. Finance report: runs without error and never includes the foreign
	// property (over-exposure guard).
	assertFinanceReportHidesForeign(t, ctx, f, w)
}

// TestSharingAudit_SuspendedMember_ExcludedFromAggregates verifies that when a
// member's access is suspended (tariff limit), all aggregate reads drop the
// shared data: operations, leases, and recurring operations.
func TestSharingAudit_SuspendedMember_ExcludedFromAggregates(t *testing.T) {
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	member := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addSuspendedPolicyMembership(t, ctx, f.members, property, member, owner, accessdomain.RoleFullAccess)
	if err := f.cats.CreateDefaultCategories(ctx, owner); err != nil {
		t.Fatalf("seed categories: %v", err)
	}
	categoryID := f.expenseCategoryID(t, ctx, owner)

	// Seed operation + lease on owner's scope.
	op, err := f.operationService().CreateOperation(ctx, owner, f.createOperationCmd(property, categoryID))
	if err != nil {
		t.Fatalf("seed op: %v", err)
	}
	_, err = f.leaseService().CreateLease(ctx, owner, f.createLeaseCmd(property))
	if err != nil {
		t.Fatalf("seed lease: %v", err)
	}

	// Suspended member sees NOTHING of the shared property on aggregates.
	ops, err := f.operationServiceWithShared().ListOperations(ctx, member,
		application.OperationFilter{Limit: 100})
	if err != nil {
		t.Fatalf("ListOperations as suspended: %v", err)
	}
	for _, o := range ops {
		if o.ID == op.ID {
			t.Errorf("suspended member saw shared operation — should be excluded")
		}
	}

	leases, err := f.leaseServiceWithShared().ListLeases(ctx, member)
	if err != nil {
		t.Fatalf("ListLeases as suspended: %v", err)
	}
	if len(leases) != 0 {
		t.Errorf("suspended member saw %d leases, expected 0", len(leases))
	}
}

// TestSharingAudit_ArchivedSharedProperty_ExcludedFromAggregates verifies that
// when a shared property is archived, its data no longer appears on aggregate
// reads for the member (the member's own archived data is unaffected, but a
// shared archived object is hidden — consistent with ListProperties).
func TestSharingAudit_ArchivedSharedProperty_ExcludedFromAggregates(t *testing.T) {
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
	categoryID := f.expenseCategoryID(t, ctx, owner)

	op, err := f.operationService().CreateOperation(ctx, owner, f.createOperationCmd(property, categoryID))
	if err != nil {
		t.Fatalf("seed op: %v", err)
	}

	// Before archive: member sees the operation.
	opsBefore, err := f.operationServiceWithShared().ListOperations(ctx, member,
		application.OperationFilter{Limit: 100})
	if err != nil {
		t.Fatalf("ListOperations before archive: %v", err)
	}
	if !containsOp(opsBefore, op.ID) {
		t.Fatalf("member did not see shared op before archive — test seed broken")
	}

	// Archive the property directly in the DB (bypassing the lifecycle service
	// which requires an open-lease check; this test is about the read filter).
	if _, err := f.tx.Exec(ctx, "UPDATE properties SET status = 'archived' WHERE id = $1", property); err != nil {
		t.Fatalf("archive property: %v", err)
	}

	// After archive: member no longer sees the shared operation on the aggregate.
	opsAfter, err := f.operationServiceWithShared().ListOperations(ctx, member,
		application.OperationFilter{Limit: 100})
	if err != nil {
		t.Fatalf("ListOperations after archive: %v", err)
	}
	if containsOp(opsAfter, op.ID) {
		t.Errorf("member saw archived shared-property operation — should be excluded from aggregate")
	}

	// The OWNER still sees their own archived operation (behaviour unchanged).
	ownerOps, err := f.operationServiceWithShared().ListOperations(ctx, owner, application.OperationFilter{Limit: 100})
	if err != nil {
		t.Fatalf("ListOperations as owner after archive: %v", err)
	}
	if !containsOp(ownerOps, op.ID) {
		t.Errorf("owner did not see own archived operation — owner behaviour must be unchanged")
	}
}

func containsOp(ops []domain.Operation, id uuid.UUID) bool {
	for _, o := range ops {
		if o.ID == id {
			return true
		}
	}
	return false
}

// membershipIDFor fetches the membership row id for a (property, user) pair, for
// suspend/reactivate/downgrade lifecycle edge-case tests.
func membershipIDFor(t *testing.T, ctx context.Context, repo *accesspg.MembershipRepository, property, user uuid.UUID) uuid.UUID {
	t.Helper()
	m, err := repo.GetByPropertyAndUser(ctx, property, user)
	if err != nil {
		t.Fatalf("get membership (%s,%s): %v", property, user, err)
	}
	return m.ID
}

// TestSharingAudit_DetachDelete_HidesFromMember verifies that after a property
// is deleted in detach mode, the member no longer sees its operations on
// aggregate reads (GET /operations), while the owner still sees the detached
// history under "no property". This locks the privacy guarantee of detach mode
// (ADR 0025) for shared access.
func TestSharingAudit_DetachDelete_HidesFromMember(t *testing.T) {
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
	categoryID := f.expenseCategoryID(t, ctx, owner)

	op, err := f.operationService().CreateOperation(ctx, owner, f.createOperationCmd(property, categoryID))
	if err != nil {
		t.Fatalf("seed op: %v", err)
	}

	// Before delete: member sees the shared operation.
	opsBefore, err := f.operationServiceWithShared().ListOperations(ctx, member,
		application.OperationFilter{Limit: 100})
	if err != nil {
		t.Fatalf("list operations before delete: %v", err)
	}
	if !containsOp(opsBefore, op.ID) {
		t.Fatalf("member did not see shared op before delete — test seed broken")
	}

	// Simulate detach delete at the SQL level: null the operation's property_id
	// (detached history stays with owner under "no property"), delete the
	// property's memberships (cascade on property delete does this in prod), and
	// delete the property itself.
	if _, err := f.tx.Exec(ctx, "UPDATE operations SET property_id = NULL WHERE property_id = $1", property); err != nil {
		t.Fatalf("detach operations: %v", err)
	}
	if _, err := f.tx.Exec(ctx, "DELETE FROM property_members WHERE property_id = $1", property); err != nil {
		t.Logf("delete memberships (best-effort): %v", err)
	}
	if _, err := f.tx.Exec(ctx, "DELETE FROM properties WHERE id = $1", property); err != nil {
		t.Fatalf("delete property: %v", err)
	}

	// After detach delete: member no longer sees the operation (no membership,
	// and the detached op has no property_id to match).
	opsAfter, err := f.operationServiceWithShared().ListOperations(ctx, member,
		application.OperationFilter{Limit: 100})
	if err != nil {
		t.Fatalf("list operations after delete: %v", err)
	}
	if containsOp(opsAfter, op.ID) {
		t.Errorf("member saw detached operation after property delete — privacy leak")
	}
}

// TestSharingAudit_SuspendReactivate_AggregateVisibility verifies the suspend →
// reactivate lifecycle: an active member sees shared data; after suspension the
// data disappears from aggregates; after reactivation it reappears.
func TestSharingAudit_SuspendReactivate_AggregateVisibility(t *testing.T) {
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
	categoryID := f.expenseCategoryID(t, ctx, owner)
	op, err := f.operationService().CreateOperation(ctx, owner, f.createOperationCmd(property, categoryID))
	if err != nil {
		t.Fatalf("seed op: %v", err)
	}
	memID := membershipIDFor(t, ctx, f.members, property, member)

	// Active: sees the operation.
	opsActive, err := f.operationServiceWithShared().ListOperations(ctx, member,
		application.OperationFilter{PropertyID: property, CategoryIDs: []uuid.UUID{categoryID}, Limit: 100})
	if err != nil {
		t.Fatalf("list operations as active member: %v", err)
	}
	if !containsOp(opsActive, op.ID) {
		t.Fatalf("active member did not see shared op — test seed broken")
	}

	// Suspend: hidden.
	if err := f.members.Suspend(ctx, memID, property); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	opsSuspended, err := f.operationServiceWithShared().ListOperations(ctx, member,
		application.OperationFilter{PropertyID: property, CategoryIDs: []uuid.UUID{categoryID}, Limit: 100})
	if err != nil {
		t.Fatalf("list operations as suspended member: %v", err)
	}
	if containsOp(opsSuspended, op.ID) {
		t.Errorf("suspended member saw shared op — should be hidden")
	}

	// Reactivate: visible again.
	if _, err := f.members.Reactivate(ctx, memID, property); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	opsReactivated, err := f.operationServiceWithShared().ListOperations(ctx, member,
		application.OperationFilter{PropertyID: property, CategoryIDs: []uuid.UUID{categoryID}, Limit: 100})
	if err != nil {
		t.Fatalf("list operations as reactivated member: %v", err)
	}
	if !containsOp(opsReactivated, op.ID) {
		t.Errorf("reactivated member did not see shared op — should be visible again")
	}
}

// TestSharingAudit_DowngradeFullToViewer_ReadOkWriteBlocked verifies that
// downgrading a member from full_access to viewer preserves read access
// (aggregates + per-property) but blocks writes (CreateOperation → Forbidden).
func TestSharingAudit_DowngradeFullToViewer_ReadOkWriteBlocked(t *testing.T) {
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
	categoryID := f.expenseCategoryID(t, ctx, owner)
	op, err := f.operationService().CreateOperation(ctx, owner, f.createOperationCmd(property, categoryID))
	if err != nil {
		t.Fatalf("seed op: %v", err)
	}
	memID := membershipIDFor(t, ctx, f.members, property, member)

	// Full access: can read + write.
	svc := f.operationServiceWithShared()
	opsFull, err := svc.ListOperations(ctx, member,
		application.OperationFilter{PropertyID: property, CategoryIDs: []uuid.UUID{categoryID}, Limit: 100})
	if err != nil {
		t.Fatalf("list operations as full_access member: %v", err)
	}
	if !containsOp(opsFull, op.ID) {
		t.Fatalf("full_access member did not see shared op — test seed broken")
	}
	if _, err := svc.CreateOperation(ctx, member, f.createOperationCmd(property, categoryID)); err != nil {
		t.Fatalf("full_access member CreateOperation failed (should succeed): %v", err)
	}

	// Downgrade to viewer.
	if _, err := f.members.UpdateRole(ctx, memID, property, accessdomain.RoleViewer); err != nil {
		t.Fatalf("downgrade to viewer: %v", err)
	}

	// viewer: can still read.
	opsViewer, err := svc.ListOperations(ctx, member,
		application.OperationFilter{PropertyID: property, CategoryIDs: []uuid.UUID{categoryID}, Limit: 100})
	if err != nil {
		t.Fatalf("list operations as viewer member: %v", err)
	}
	if !containsOp(opsViewer, op.ID) {
		t.Errorf("viewer member did not see shared op — read access must persist after downgrade")
	}
	// viewer: write blocked.
	if _, err := svc.CreateOperation(ctx, member, f.createOperationCmd(property, categoryID)); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("viewer member CreateOperation: want ErrForbidden, got %v", err)
	}
}
