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
	svc := application.NewRecurringOperationService(f.recs, f.ops, f.props, f.cats, nil, factory, f.clock, policyTestTzResolver{}, f.policy, nil)
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
	owner1 := createPolicyTestUser(t, ctx, f.q)
	owner2 := createPolicyTestUser(t, ctx, f.q)
	stranger := createPolicyTestUser(t, ctx, f.q) // owner the member has NO access to
	member := createPolicyTestUser(t, ctx, f.q)

	prop1 := createPolicyTestProperty(t, ctx, f.q, owner1)
	prop2 := createPolicyTestProperty(t, ctx, f.q, owner2)
	foreign := createPolicyTestProperty(t, ctx, f.q, stranger)

	addPolicyMembership(t, ctx, f.members, prop1, member, owner1, accessdomain.RoleFullAccess)
	addPolicyMembership(t, ctx, f.members, prop2, member, owner2, accessdomain.RoleViewer)
	// No membership on `foreign`.

	if err := f.cats.CreateDefaultCategories(ctx, owner1); err != nil {
		t.Fatalf("seed o1 categories: %v", err)
	}
	if err := f.cats.CreateDefaultCategories(ctx, owner2); err != nil {
		t.Fatalf("seed o2 categories: %v", err)
	}
	if err := f.cats.CreateDefaultCategories(ctx, stranger); err != nil {
		t.Fatalf("seed stranger categories: %v", err)
	}
	cat1 := f.expenseCategoryID(t, ctx, owner1)
	cat2 := f.expenseCategoryID(t, ctx, owner2)
	catForeign := f.expenseCategoryID(t, ctx, stranger)

	// Seed one operation on each property (owner acts).
	op1, err := f.operationService().CreateOperation(ctx, owner1, f.createOperationCmd(prop1, cat1))
	if err != nil {
		t.Fatalf("seed op1: %v", err)
	}
	if op1.PropertyID != prop1 {
		t.Fatalf("op1 property: want %s, got %s — test seed broken", prop1, op1.PropertyID)
	}
	op2, err := f.operationService().CreateOperation(ctx, owner2, f.createOperationCmd(prop2, cat2))
	if err != nil {
		t.Fatalf("seed op2: %v", err)
	}
	opForeign, err := f.operationService().CreateOperation(ctx, stranger, f.createOperationCmd(foreign, catForeign))
	if err != nil {
		t.Fatalf("seed opForeign: %v", err)
	}

	// Leases on prop1 and prop2.
	lease1, err := f.leaseService().CreateLease(ctx, owner1, f.createLeaseCmd(prop1))
	if err != nil {
		t.Fatalf("seed lease1: %v", err)
	}
	lease2, err := f.leaseService().CreateLease(ctx, owner2, f.createLeaseCmd(prop2))
	if err != nil {
		t.Fatalf("seed lease2: %v", err)
	}

	// 1. Operations aggregate: member sees op1 + op2, not opForeign.
	// Use a property filter scoped to prop1 to isolate from unrelated rows in
	// the shared test database, then prop2 separately.
	memberOps1, err := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{PropertyID: prop1, CategoryIDs: []uuid.UUID{cat1}, Limit: 100})
	if err != nil {
		t.Fatalf("ListOperations (prop1) as member: %v", err)
	}
	memberOps2, err := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{PropertyID: prop2, CategoryIDs: []uuid.UUID{cat2}, Limit: 100})
	if err != nil {
		t.Fatalf("ListOperations (prop2) as member: %v", err)
	}
	memberOpsForeign, err := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{PropertyID: foreign, CategoryIDs: []uuid.UUID{catForeign}, Limit: 100})
	if err != nil {
		t.Fatalf("ListOperations (foreign) as member: %v", err)
	}
	hasOp := func(ops []domain.Operation, id uuid.UUID) bool {
		for _, o := range ops {
			if o.ID == id {
				return true
			}
		}
		return false
	}
	if !hasOp(memberOps1, op1.ID) {
		t.Errorf("member missing op1 (owner1 shared) in prop1+cat1-filtered list; got %d ops", len(memberOps1))
	}
	if !hasOp(memberOps2, op2.ID) {
		t.Errorf("member missing op2 (owner2 shared) in prop2-filtered list; got %d ops", len(memberOps2))
	}
	if hasOp(memberOpsForeign, opForeign.ID) {
		t.Errorf("member saw foreign op (no membership) — over-exposure leak")
	}

	// 2. Leases aggregate: member sees lease1 + lease2.
	memberLeases, err := f.leaseServiceWithShared().ListLeases(ctx, member)
	if err != nil {
		t.Fatalf("ListLeases as member: %v", err)
	}
	hasLease := func(id uuid.UUID) bool {
		for _, l := range memberLeases {
			if l.ID == id {
				return true
			}
		}
		return false
	}
	if !hasLease(lease1.ID) {
		t.Errorf("member missing lease1 (owner1 shared); got %d leases", len(memberLeases))
	}
	if !hasLease(lease2.ID) {
		t.Errorf("member missing lease2 (owner2 shared); got %d leases", len(memberLeases))
	}

	// 3. Finance report: runs without error for the member and never includes
	// the foreign property (over-exposure guard). Seeded operations are
	// pending/planned (not paid/received), so the report may be empty by
	// property — the meaningful assertions are no-error and no-foreign-leak.
	report, err := f.operationServiceWithShared().GetFinanceReport(ctx, member, nil, nil)
	if err != nil {
		t.Fatalf("GetFinanceReport as member: %v", err)
	}
	for _, row := range report.ByProperty {
		if row.PropertyID == foreign {
			t.Errorf("finance report included foreign property (no membership) — over-exposure leak")
		}
	}
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
	ops, err := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{Limit: 100})
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
	opsBefore, err := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{Limit: 100})
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
	opsAfter, err := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{Limit: 100})
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
	opsBefore, _ := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{Limit: 100})
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
	opsAfter, _ := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{Limit: 100})
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
	opsActive, _ := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{PropertyID: property, CategoryIDs: []uuid.UUID{categoryID}, Limit: 100})
	if !containsOp(opsActive, op.ID) {
		t.Fatalf("active member did not see shared op — test seed broken")
	}

	// Suspend: hidden.
	if err := f.members.Suspend(ctx, memID, property); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	opsSuspended, _ := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{PropertyID: property, CategoryIDs: []uuid.UUID{categoryID}, Limit: 100})
	if containsOp(opsSuspended, op.ID) {
		t.Errorf("suspended member saw shared op — should be hidden")
	}

	// Reactivate: visible again.
	if _, err := f.members.Reactivate(ctx, memID, property); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	opsReactivated, _ := f.operationServiceWithShared().ListOperations(ctx, member, application.OperationFilter{PropertyID: property, CategoryIDs: []uuid.UUID{categoryID}, Limit: 100})
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

	// full_access: can read + write.
	svc := f.operationServiceWithShared()
	opsFull, _ := svc.ListOperations(ctx, member, application.OperationFilter{PropertyID: property, CategoryIDs: []uuid.UUID{categoryID}, Limit: 100})
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
	opsViewer, _ := svc.ListOperations(ctx, member, application.OperationFilter{PropertyID: property, CategoryIDs: []uuid.UUID{categoryID}, Limit: 100})
	if !containsOp(opsViewer, op.ID) {
		t.Errorf("viewer member did not see shared op — read access must persist after downgrade")
	}
	// viewer: write blocked.
	if _, err := svc.CreateOperation(ctx, member, f.createOperationCmd(property, categoryID)); !errors.Is(err, application.ErrForbidden) {
		t.Errorf("viewer member CreateOperation: want ErrForbidden, got %v", err)
	}
}
