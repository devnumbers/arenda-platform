package application

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// This file holds the SlotCoordinator end-to-end scenarios on in-memory ports
// (issue #158, T4). The pure selection logic is covered by selection_test.go;
// here we exercise the coordinator wiring the selection to the repository, the
// owner resolver, the limiter, and the occupancy port.
//
// Reused fixtures from sibling test files (same package):
//   - memRepo (service_integration_test.go): in-memory MembershipRepository;
//     memRepo.SetOwner records the owner of a property for the
//     ListActiveByPropertyOwner owner-wide query.
//   - staticResolver (service_integration_test.go): fixed property→owner map.
//   - noopTx / noopBeginner (service_integration_test.go): no-op transaction.
//   - mustUUID (selection_test.go): fixed-UUID helper for readable setup.

// ---------------------------------------------------------------------------
// In-memory port stubs for the coordinator's remaining dependencies.
// ---------------------------------------------------------------------------

// fakeRecipientLimiter is an in-memory RecipientLimiter. Limits are per
// recipient; an absent recipient reads as 0. WithTx returns itself because the
// fake holds no transactional state.
type fakeRecipientLimiter struct {
	limits map[uuid.UUID]int
}

func newFakeRecipientLimiter() *fakeRecipientLimiter {
	return &fakeRecipientLimiter{limits: map[uuid.UUID]int{}}
}

func (f *fakeRecipientLimiter) set(recipientID uuid.UUID, limit int) {
	f.limits[recipientID] = limit
}

func (f *fakeRecipientLimiter) ActivePropertyLimit(_ context.Context, recipientID uuid.UUID) (int, error) {
	return f.limits[recipientID], nil // default 0
}

func (f *fakeRecipientLimiter) WithTx(_ transaction.Tx) (RecipientLimiter, error) {
	return f, nil
}

// fakeOccupancy is an in-memory OccupancyPort keyed by data owner: each owner
// maps to the set of property ids that have an open lease.
type fakeOccupancy struct {
	byOwner map[uuid.UUID]map[uuid.UUID]bool
}

func newFakeOccupancy() *fakeOccupancy {
	return &fakeOccupancy{byOwner: map[uuid.UUID]map[uuid.UUID]bool{}}
}

// setOpen marks propertyID as having an open lease under ownerID.
func (f *fakeOccupancy) setOpen(ownerID, propertyID uuid.UUID) {
	set, ok := f.byOwner[ownerID]
	if !ok {
		set = map[uuid.UUID]bool{}
		f.byOwner[ownerID] = set
	}
	set[propertyID] = true
}

func (f *fakeOccupancy) OccupiedPropertyIDs(_ context.Context, ownerID uuid.UUID) (map[uuid.UUID]bool, error) {
	out := make(map[uuid.UUID]bool, len(f.byOwner[ownerID]))
	maps.Copy(out, f.byOwner[ownerID])
	return out, nil
}

// fakeOwnedProps is an in-memory OwnedActivePropertiesPort: each "owner" maps to
// the slice of their own active properties (with UpdatedAt for the eviction
// comparator).
type fakeOwnedProps struct {
	byOwner map[uuid.UUID][]OwnedPropertyMeta
}

func newFakeOwnedProps() *fakeOwnedProps {
	return &fakeOwnedProps{byOwner: map[uuid.UUID][]OwnedPropertyMeta{}}
}

func (f *fakeOwnedProps) add(ownerID, propertyID uuid.UUID, updatedAt time.Time) {
	f.byOwner[ownerID] = append(f.byOwner[ownerID], OwnedPropertyMeta{ID: propertyID, UpdatedAt: updatedAt})
}

func (f *fakeOwnedProps) ListActiveWithMeta(_ context.Context, ownerID uuid.UUID) ([]OwnedPropertyMeta, error) {
	return slices.Clone(f.byOwner[ownerID]), nil
}

// Compile-time interface checks for the in-memory stubs.
var (
	_ RecipientLimiter          = (*fakeRecipientLimiter)(nil)
	_ OccupancyPort             = (*fakeOccupancy)(nil)
	_ OwnedActivePropertiesPort = (*fakeOwnedProps)(nil)
)

// ---------------------------------------------------------------------------
// Test scaffolding helpers.
// ---------------------------------------------------------------------------

// coordinatorFixture bundles the coordinator with its in-memory dependencies so
// each scenario can set up state and assert against the same handles.
type coordinatorFixture struct {
	repo        *memRepo
	owners      staticResolver
	limiter     *fakeRecipientLimiter
	occupancy   *fakeOccupancy
	ownedProps  *fakeOwnedProps
	coordinator *SlotCoordinator
}

func newCoordinatorFixture() *coordinatorFixture {
	repo := newMemRepo()
	owners := staticResolver{}
	limiter := newFakeRecipientLimiter()
	occupancy := newFakeOccupancy()
	ownedProps := newFakeOwnedProps()
	coordinator := NewSlotCoordinator(
		repo,
		owners,
		limiter,
		occupancy,
		ownedProps,
		nil,
		auditapp.Noop{},
		noopBeginner{},
	)
	return &coordinatorFixture{
		repo:        repo,
		owners:      owners,
		limiter:     limiter,
		occupancy:   occupancy,
		ownedProps:  ownedProps,
		coordinator: coordinator,
	}
}

// linkOwner records in every in-memory port that propertyID belongs to ownerID:
// the repo owner join and the resolver. (occupancy/owned are keyed by owner at
// query time, so nothing to record there.)
func (f *coordinatorFixture) linkOwner(ownerID, propertyID uuid.UUID) {
	f.repo.SetOwner(propertyID, ownerID)
	f.owners[propertyID] = ownerID
}

// addActiveMember inserts an active viewer shared membership of recipientID on
// propertyID granted by ownerID. It sets deterministic UpdatedAt via at so the
// eviction/recency comparator is stable.
func (f *coordinatorFixture) addActiveMember(t *testing.T, memberID, propertyID, ownerID, recipientID uuid.UUID, at time.Time) {
	t.Helper()
	f.linkOwner(ownerID, propertyID)
	if _, err := f.repo.Create(context.Background(), domain.Membership{
		ID:         memberID,
		PropertyID: propertyID,
		UserID:     recipientID,
		Role:       domain.RoleViewer,
		GrantedBy:  ownerID,
	}); err != nil {
		t.Fatalf("Create active member %s: %v", memberID, err)
	}
	// Create stamps CreatedAt/UpdatedAt with time.Now(); overwrite for stable
	// recency comparisons in the eviction comparator.
	f.repo.setUpdatedAt(memberID, propertyID, at)
}

// addSuspendedMember inserts a suspended viewer shared membership with a fixed
// SuspendedAt/UpdatedAt so FIFO ordering and the "already suspended" invariant
// are stable across runs.
func (f *coordinatorFixture) addSuspendedMember(t *testing.T, memberID, propertyID, ownerID, recipientID uuid.UUID, suspendedAt, updatedAt time.Time) {
	t.Helper()
	f.linkOwner(ownerID, propertyID)
	if _, err := f.repo.CreateWithStatus(context.Background(), domain.Membership{
		ID:          memberID,
		PropertyID:  propertyID,
		UserID:      recipientID,
		Role:        domain.RoleViewer,
		GrantedBy:   ownerID,
		Status:      domain.MemberStatusSuspended,
		SuspendedAt: &suspendedAt,
	}); err != nil {
		t.Fatalf("CreateWithStatus suspended member %s: %v", memberID, err)
	}
	f.repo.setUpdatedAt(memberID, propertyID, updatedAt)
	f.repo.setSuspendedAt(memberID, propertyID, suspendedAt)
}

// statusOf returns the current status of a membership, failing the test if the
// row is missing.
func (f *coordinatorFixture) statusOf(t *testing.T, memberID, propertyID uuid.UUID) domain.MemberStatus {
	t.Helper()
	return f.repo.mustGet(t, memberID, propertyID).Status
}

// assertStatus fails the test unless the membership currently has want.
func (f *coordinatorFixture) assertStatus(t *testing.T, memberID, propertyID uuid.UUID, want domain.MemberStatus, msg string) {
	t.Helper()
	if got := f.statusOf(t, memberID, propertyID); got != want {
		t.Errorf("%s: member %s status = %s, want %s", msg, memberID, got, want)
	}
}

// ---------------------------------------------------------------------------
// memRepo test helpers: deterministic timestamps.
// ---------------------------------------------------------------------------

// setUpdatedAt overwrites the UpdatedAt of a row. memRepo.Create stamps
// time.Now(), which is fine for service tests but breaks recency comparison in
// coordinator scenarios. This is a test-only helper, not production code.
func (r *memRepo) setUpdatedAt(id, propertyID uuid.UUID, at time.Time) {
	for i := range r.rows {
		if r.rows[i].ID == id && r.rows[i].PropertyID == propertyID {
			r.rows[i].UpdatedAt = at
			return
		}
	}
}

// setSuspendedAt overwrites SuspendedAt of a row. memRepo.Suspend always
// re-stamps time.Now(); for scenario D ("already-suspended not re-stamped by a
// repeat downgrade") we need a fixed original timestamp we can compare against.
func (r *memRepo) setSuspendedAt(id, propertyID uuid.UUID, at time.Time) {
	for i := range r.rows {
		if r.rows[i].ID == id && r.rows[i].PropertyID == propertyID {
			r.rows[i].SuspendedAt = &at
			return
		}
	}
}

// mustGet returns the row, failing the test if it is missing.
func (r *memRepo) mustGet(t *testing.T, id, propertyID uuid.UUID) domain.Membership {
	t.Helper()
	m, err := r.GetByID(context.Background(), id, propertyID)
	if err != nil {
		t.Fatalf("GetByID %s: %v", id, err)
	}
	return m
}

// ---------------------------------------------------------------------------
// Scenarios.
// ---------------------------------------------------------------------------

// Fixed instants; t1Old is oldest. Using fixed times keeps eviction recency
// stable across runs.
var (
	scenarioBase = time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	t1Old        = scenarioBase
	t2New        = scenarioBase.Add(1 * time.Hour)
	t3Newer      = scenarioBase.Add(2 * time.Hour)
)

// Scenario A: EnforceRecipientLimit — downgrade suspends the excess shared
// membership (the one with the earlier UpdatedAt, neither having an open
// lease). One is suspended, the other stays active.
func TestSlotCoordinator_EnforceRecipientLimit_DowngradeSuspendsExcess(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	owner := uuid.New()
	p1 := uuid.New()
	p2 := uuid.New()
	recipient := uuid.New()
	m1 := uuid.New()
	m2 := uuid.New()

	// Recipient holds active shared memberships on both of owner's properties.
	// m1 is the earlier-updated (recency comparator ranks it "worse", so it is
	// the eviction candidate when neither has an open lease).
	f.addActiveMember(t, m1, p1, owner, recipient, t1Old)
	f.addActiveMember(t, m2, p2, owner, recipient, t2New)
	f.limiter.set(recipient, 1) // limit 1, but two shared → 1 excess

	if err := f.coordinator.EnforceRecipientLimit(context.Background(), noopTx{}, owner, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}

	// Exactly one membership suspended (the earlier one), the other active.
	f.assertStatus(t, m1, p1, domain.MemberStatusSuspended, "excess member should be suspended")
	f.assertStatus(t, m2, p2, domain.MemberStatusActive, "kept member should stay active")

	if got, _ := f.repo.ListActiveByUser(context.Background(), recipient); len(got) != 1 {
		t.Errorf("ListActiveByUser = %d, want 1", len(got))
	}
	if got, _ := f.repo.ListSuspendedByUser(context.Background(), recipient); len(got) != 1 {
		t.Errorf("ListSuspendedByUser = %d, want 1", len(got))
	}
}

// Scenario B: EnforceRecipientLimit — the recipient's own objects appear in the
// pool but are NOT suspended by the coordinator (own-object eviction is the
// PropertyArchiver's job). Only the excess shared membership is suspended.
func TestSlotCoordinator_EnforceRecipientLimit_OwnObjectsNotTouched(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	owner := uuid.New()
	ownProp := uuid.New()
	sharedProp := uuid.New()
	recipient := uuid.New()
	sharedMember := uuid.New()

	// Recipient has one own property (registered with ownedProps only — it is
	// NOT a membership row, the coordinator never sees it in memRepo) and one
	// shared membership. The own property is the more-recently-updated of the
	// two (no open lease on either), so it is the "best stays" entry and the
	// shared membership is the eviction candidate — which the coordinator
	// suspends. The own object is never suspended (PropertyArchiver's job).
	f.linkOwner(owner, sharedProp)
	f.ownedProps.add(recipient, ownProp, t2New) // own = newer = "stays"
	f.addActiveMember(t, sharedMember, sharedProp, owner, recipient, t1Old)
	f.limiter.set(recipient, 1) // pool size = own(1) + shared(1) = 2 > 1 → 1 excess

	if err := f.coordinator.EnforceRecipientLimit(context.Background(), noopTx{}, owner, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}

	// The shared membership must be suspended; the own object is the
	// PropertyArchiver's concern and the coordinator never touched it (it has
	// no membership row to suspend).
	f.assertStatus(t, sharedMember, sharedProp, domain.MemberStatusSuspended, "excess shared member suspended")
}

// Scenario C: EnforceRecipientLimit — an open lease protects the membership
// even when it is the less-recently-updated one. The membership without an open
// lease is suspended.
func TestSlotCoordinator_EnforceRecipientLimit_OpenLeaseProtects(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	owner := uuid.New()
	pNoLease := uuid.New()   // shared membership, no open lease
	pOpenLease := uuid.New() // shared membership, open lease present
	recipient := uuid.New()
	mNo := uuid.New()
	mOpen := uuid.New()

	// mNo is the more-recently-updated but has no lease; mOpen is older but has
	// an open lease. The open lease must win and protect mOpen.
	f.addActiveMember(t, mNo, pNoLease, owner, recipient, t3Newer)
	f.addActiveMember(t, mOpen, pOpenLease, owner, recipient, t1Old)
	f.occupancy.setOpen(owner, pOpenLease) // owner's property pOpenLease has an open lease
	f.limiter.set(recipient, 1)

	if err := f.coordinator.EnforceRecipientLimit(context.Background(), noopTx{}, owner, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}

	f.assertStatus(t, mNo, pNoLease, domain.MemberStatusSuspended, "no-lease member suspended")
	f.assertStatus(t, mOpen, pOpenLease, domain.MemberStatusActive, "open-lease member protected")
}

// Scenario D: EnforceRecipientLimit — a repeat downgrade does not re-evaluate
// an already-suspended membership (it is not in the active pool), and does not
// re-stamp its SuspendedAt. A second active membership is suspended if the
// limit is still exceeded.
func TestSlotCoordinator_EnforceRecipientLimit_RepeatDowngradeDoesNotRetouchSuspended(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	owner := uuid.New()
	p1 := uuid.New()
	p2 := uuid.New()
	recipient := uuid.New()
	mAlready := uuid.New()
	mActive := uuid.New()

	// mAlready is pre-suspended with a fixed timestamp; mActive is still active.
	originalSuspended := t1Old
	f.addSuspendedMember(t, mAlready, p1, owner, recipient, originalSuspended, t1Old)
	f.addActiveMember(t, mActive, p2, owner, recipient, t2New)
	// Limit 0: the active pool (1 shared) already exceeds it, so mActive is
	// suspended on the repeat call. mAlready is not in the active pool, so its
	// SuspendedAt must be untouched.
	f.limiter.set(recipient, 0)

	if err := f.coordinator.EnforceRecipientLimit(context.Background(), noopTx{}, owner, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}

	// mActive was suspended by this call.
	f.assertStatus(t, mActive, p2, domain.MemberStatusSuspended, "active member suspended on repeat downgrade")

	// mAlready's SuspendedAt is unchanged: the coordinator skipped it (it was
	// not in ListActiveByUser) and never called Suspend on it again.
	got := f.repo.mustGet(t, mAlready, p1)
	if got.SuspendedAt == nil || !got.SuspendedAt.Equal(originalSuspended) {
		t.Errorf("already-suspended member re-stamped: SuspendedAt = %v, want %v", got.SuspendedAt, originalSuspended)
	}
}

// Scenario E: EnforceOnActivation — a free slot exists (pool empty, limit 1) →
// the new membership need not be suspended.
func TestSlotCoordinator_EnforceOnActivation_FreeSlotReturnsFalse(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()
	recipient := uuid.New()
	f.limiter.set(recipient, 1) // used=0 < limit=1 → free slot

	got, err := f.coordinator.EnforceOnActivation(context.Background(), noopTx{}, recipient)
	if err != nil {
		t.Fatalf("EnforceOnActivation: %v", err)
	}
	if got {
		t.Errorf("EnforceOnActivation = true, want false (free slot available)")
	}
}

// Scenario F: EnforceOnActivation — the recipient's pool is at the limit (own
// property occupies the single slot) → the new membership must be suspended.
func TestSlotCoordinator_EnforceOnActivation_PoolFullReturnsTrue(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	recipient := uuid.New()
	ownProp := uuid.New()
	f.ownedProps.add(recipient, ownProp, t1Old)
	f.limiter.set(recipient, 1) // used=1 >= limit=1 → no free slot

	got, err := f.coordinator.EnforceOnActivation(context.Background(), noopTx{}, recipient)
	if err != nil {
		t.Fatalf("EnforceOnActivation: %v", err)
	}
	if !got {
		t.Errorf("EnforceOnActivation = false, want true (pool full)")
	}
}

// Scenario G: RecoverSuspended — FIFO: the earliest-suspended membership is
// reactivated when a slot frees up, the later one stays suspended.
func TestSlotCoordinator_RecoverSuspended_FIFORecoversEarliest(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	owner := uuid.New()
	p1 := uuid.New()
	p2 := uuid.New()
	recipient := uuid.New()
	m1 := uuid.New() // suspended earlier
	m2 := uuid.New() // suspended later

	f.addSuspendedMember(t, m1, p1, owner, recipient, t1Old, t1Old)
	f.addSuspendedMember(t, m2, p2, owner, recipient, t2New, t2New)
	f.limiter.set(recipient, 1) // used=0, freeSlots=1 → recover 1

	if err := f.coordinator.RecoverSuspended(context.Background(), noopTx{}, recipient); err != nil {
		t.Fatalf("RecoverSuspended: %v", err)
	}

	// m1 (earliest) reactivated, m2 stays suspended.
	f.assertStatus(t, m1, p1, domain.MemberStatusActive, "earliest suspended should be reactivated (FIFO)")
	f.assertStatus(t, m2, p2, domain.MemberStatusSuspended, "later suspended should stay suspended")

	if got, _ := f.repo.ListActiveByUser(context.Background(), recipient); len(got) != 1 {
		t.Errorf("ListActiveByUser = %d, want 1", len(got))
	}
	if got, _ := f.repo.ListSuspendedByUser(context.Background(), recipient); len(got) != 1 {
		t.Errorf("ListSuspendedByUser = %d, want 1", len(got))
	}
}

// Scenario H: RecoverSuspended — no free slot (an active membership already
// occupies the single slot) → nothing is recovered.
func TestSlotCoordinator_RecoverSuspended_NoFreeSlotRecoverNothing(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	owner := uuid.New()
	pActive := uuid.New()
	pSuspended := uuid.New()
	recipient := uuid.New()
	mActive := uuid.New()
	mSuspended := uuid.New()

	// One active shared membership (occupies the slot) + one suspended.
	f.addActiveMember(t, mActive, pActive, owner, recipient, t1Old)
	f.addSuspendedMember(t, mSuspended, pSuspended, owner, recipient, t2New, t2New)
	f.limiter.set(recipient, 1) // used=1, freeSlots=0

	if err := f.coordinator.RecoverSuspended(context.Background(), noopTx{}, recipient); err != nil {
		t.Fatalf("RecoverSuspended: %v", err)
	}

	f.assertStatus(t, mSuspended, pSuspended, domain.MemberStatusSuspended, "suspended should stay suspended (no free slot)")
	f.assertStatus(t, mActive, pActive, domain.MemberStatusActive, "active should stay active")

	if got, _ := f.repo.ListActiveByUser(context.Background(), recipient); len(got) != 1 {
		t.Errorf("ListActiveByUser = %d, want 1", len(got))
	}
	if got, _ := f.repo.ListSuspendedByUser(context.Background(), recipient); len(got) != 1 {
		t.Errorf("ListSuspendedByUser = %d, want 1", len(got))
	}
}

// Scenario I: RecoverSuspended — tie-break inside a downgrade batch (equal
// SuspendedAt): the membership whose property has an open lease is recovered
// first.
func TestSlotCoordinator_RecoverSuspended_BatchTieBreakOpenLeaseFirst(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	owner := uuid.New()
	pNoLease := uuid.New()
	pOpenLease := uuid.New()
	recipient := uuid.New()
	mNo := uuid.New()
	mOpen := uuid.New()

	// Same SuspendedAt (a downgrade batch). mOpen's property has an open lease,
	// so the batch tie-break recovers it first. Only one free slot → only one
	// is recovered.
	batchSuspended := t1Old
	f.addSuspendedMember(t, mNo, pNoLease, owner, recipient, batchSuspended, t1Old)
	f.addSuspendedMember(t, mOpen, pOpenLease, owner, recipient, batchSuspended, t2New)
	f.occupancy.setOpen(owner, pOpenLease)
	f.limiter.set(recipient, 1) // used=0, freeSlots=1

	if err := f.coordinator.RecoverSuspended(context.Background(), noopTx{}, recipient); err != nil {
		t.Fatalf("RecoverSuspended: %v", err)
	}

	f.assertStatus(t, mOpen, pOpenLease, domain.MemberStatusActive, "open-lease suspended should be recovered first")
	f.assertStatus(t, mNo, pNoLease, domain.MemberStatusSuspended, "no-lease suspended should stay suspended")
}

// Scenario J: EnforceRecipientLimit — multiple recipients of the same owner are
// each evaluated against their own limit.
func TestSlotCoordinator_EnforceRecipientLimit_MultipleRecipients(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	owner := uuid.New()
	p1 := uuid.New()
	p2 := uuid.New()
	r1 := uuid.New() // limit 0 → suspended
	r2 := uuid.New() // limit 1 → stays active
	m1 := uuid.New()
	m2 := uuid.New()

	f.addActiveMember(t, m1, p1, owner, r1, t1Old)
	f.addActiveMember(t, m2, p2, owner, r2, t2New)
	f.limiter.set(r1, 0)
	f.limiter.set(r2, 1)

	if err := f.coordinator.EnforceRecipientLimit(context.Background(), noopTx{}, owner, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}

	f.assertStatus(t, m1, p1, domain.MemberStatusSuspended, "r1 (limit 0) suspended")
	f.assertStatus(t, m2, p2, domain.MemberStatusActive, "r2 (limit 1) stays active")
}

// Scenario N: EnforceRecipientLimit — the argument is the downgrading user
// himself (billing passes sub.UserID). When he holds shared memberships on
// OTHER owners' properties over his own limit, his excess memberships are
// suspended even though he has no members on properties of his own.
func TestSlotCoordinator_EnforceRecipientLimit_DowngradingUserAsRecipient(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	foreignOwner := uuid.New()
	p1 := uuid.New()
	p2 := uuid.New()
	downgrading := uuid.New() // the user whose tariff dropped; billing passes his id
	m1 := uuid.New()
	m2 := uuid.New()

	// The downgrading user holds active shared memberships on a foreign owner's
	// properties; m1 is the earlier-updated eviction candidate.
	f.addActiveMember(t, m1, p1, foreignOwner, downgrading, t1Old)
	f.addActiveMember(t, m2, p2, foreignOwner, downgrading, t2New)
	f.limiter.set(downgrading, 1)

	if err := f.coordinator.EnforceRecipientLimit(context.Background(), noopTx{}, downgrading, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}

	f.assertStatus(t, m1, p1, domain.MemberStatusSuspended, "downgrading user's excess shared membership suspended")
	f.assertStatus(t, m2, p2, domain.MemberStatusActive, "kept membership stays active")
}

// Scenario O: EnforceRecipientLimit — the downgrading user is both an owner
// with a member and a recipient on a foreign object: both pools are enforced
// in one call (the member's and his own), each recipient processed once.
func TestSlotCoordinator_EnforceRecipientLimit_OwnerAndRecipientInOneCall(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	user := uuid.New()
	ownProp := uuid.New()
	foreignOwner := uuid.New()
	foreignProp := uuid.New()
	member := uuid.New()   // member of the user's own property
	mOwn := uuid.New()     // the member's membership on the user's property
	mForeign := uuid.New() // the user's membership on the foreign property

	f.addActiveMember(t, mOwn, ownProp, user, member, t1Old)
	f.addActiveMember(t, mForeign, foreignProp, foreignOwner, user, t2New)
	f.limiter.set(member, 0) // the member's pool (1 shared) exceeds → suspended
	f.limiter.set(user, 0)   // the user's own pool (1 shared) exceeds → suspended

	if err := f.coordinator.EnforceRecipientLimit(context.Background(), noopTx{}, user, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}

	f.assertStatus(t, mOwn, ownProp, domain.MemberStatusSuspended, "member of the user's property suspended")
	f.assertStatus(t, mForeign, foreignProp, domain.MemberStatusSuspended, "user's own foreign membership suspended")
}

// Scenario K: RecoverSuspendedForProperty — archiving a shared object frees a
// slot for each recipient; each recipient's own suspended queue is recovered
// FIFO per-recipient.
func TestSlotCoordinator_RecoverSuspendedForProperty_PerRecipientRecovery(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	owner := uuid.New()
	pArchived := uuid.New() // the object being archived (both recipients hold it)
	// Each recipient also has a suspended membership on another of owner's
	// properties, which should be recovered once their slot frees.
	pR1Suspended := uuid.New()
	pR2Suspended := uuid.New()
	r1 := uuid.New()
	r2 := uuid.New()
	mArchivedR1 := uuid.New()
	mArchivedR2 := uuid.New()
	mR1Suspended := uuid.New()
	mR2Suspended := uuid.New()

	// Active memberships on the archived object (these would be removed by the
	// properties service in a real archive flow; here they still occupy slots
	// because we exercise the access-side recovery in isolation).
	f.addActiveMember(t, mArchivedR1, pArchived, owner, r1, t1Old)
	f.addActiveMember(t, mArchivedR2, pArchived, owner, r2, t1Old)
	// Each recipient also has one suspended membership elsewhere.
	f.addSuspendedMember(t, mR1Suspended, pR1Suspended, owner, r1, t2New, t2New)
	f.addSuspendedMember(t, mR2Suspended, pR2Suspended, owner, r2, t2New, t2New)

	// Each recipient has limit 1. Currently used=1 (the archived object) → no
	// free slot. RecoverSuspendedForProperty must therefore not recover
	// anything yet: this verifies the per-recipient iteration runs without error
	// and respects the free-slot guard.
	f.limiter.set(r1, 1)
	f.limiter.set(r2, 1)

	if err := f.coordinator.RecoverSuspendedForProperty(context.Background(), noopTx{}, pArchived); err != nil {
		t.Fatalf("RecoverSuspendedForProperty: %v", err)
	}

	// No free slot → both suspended memberships stay suspended.
	f.assertStatus(t, mR1Suspended, pR1Suspended, domain.MemberStatusSuspended, "r1 suspended stays (no free slot)")
	f.assertStatus(t, mR2Suspended, pR2Suspended, domain.MemberStatusSuspended, "r2 suspended stays (no free slot)")

	// Now simulate the slot actually freeing: the properties service removed
	// the archived-object membership rows. Remove them from the repo and re-run
	// recovery; both recipients should now recover their suspended membership.
	if err := f.repo.Delete(context.Background(), mArchivedR1, pArchived); err != nil {
		t.Fatalf("Delete mArchivedR1: %v", err)
	}
	if err := f.repo.Delete(context.Background(), mArchivedR2, pArchived); err != nil {
		t.Fatalf("Delete mArchivedR2: %v", err)
	}
	if err := f.coordinator.RecoverSuspendedForProperty(context.Background(), noopTx{}, pArchived); err != nil {
		t.Fatalf("RecoverSuspendedForProperty (after free): %v", err)
	}

	// pArchived still has no remaining memberships (both were deleted), so the
	// per-property loop finds no recipients and does nothing — which is the
	// correct behavior, but means recovery here is driven by each recipient's
	// own RecoverSuspended, not the per-property entry point. Exercise that
	// directly to assert the slot-freed outcome.
	for _, r := range []uuid.UUID{r1, r2} {
		if err := f.coordinator.RecoverSuspended(context.Background(), noopTx{}, r); err != nil {
			t.Fatalf("RecoverSuspended(%s): %v", r, err)
		}
	}
	f.assertStatus(t, mR1Suspended, pR1Suspended, domain.MemberStatusActive, "r1 recovered after slot freed")
	f.assertStatus(t, mR2Suspended, pR2Suspended, domain.MemberStatusActive, "r2 recovered after slot freed")
}

// Scenario L (bonus): RecoverAfterPropertyDelete drops every membership on the
// deleted property and then recovers each recipient's oldest suspended
// membership FIFO.
func TestSlotCoordinator_RecoverAfterPropertyDelete_DropsAndRecovers(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	owner := uuid.New()
	pDeleted := uuid.New()
	pSuspended := uuid.New()
	recipient := uuid.New()
	mDeleted := uuid.New()
	mSuspended := uuid.New()

	f.addActiveMember(t, mDeleted, pDeleted, owner, recipient, t1Old)
	f.addSuspendedMember(t, mSuspended, pSuspended, owner, recipient, t2New, t2New)
	f.limiter.set(recipient, 1)

	if err := f.coordinator.RecoverAfterPropertyDelete(context.Background(), noopTx{}, pDeleted); err != nil {
		t.Fatalf("RecoverAfterPropertyDelete: %v", err)
	}

	// The deleted-property membership is gone.
	if _, err := f.repo.GetByID(context.Background(), mDeleted, pDeleted); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("deleted membership still present: err = %v, want ErrMemberNotFound", err)
	}
	// The slot freed by the delete recovered the suspended membership.
	f.assertStatus(t, mSuspended, pSuspended, domain.MemberStatusActive, "suspended recovered after property delete")
}

// Scenario M (bonus): EnforceOnUnarchiveForProperty suspends a recipient's
// membership on the unarchived object when the recipient has no free slot.
func TestSlotCoordinator_EnforceOnUnarchiveForProperty_SuspendsWhenNoSlot(t *testing.T) {
	t.Parallel()
	f := newCoordinatorFixture()

	owner := uuid.New()
	pUnarchived := uuid.New()
	pOwn := uuid.New()
	recipient := uuid.New()
	mUnarchived := uuid.New()

	f.addActiveMember(t, mUnarchived, pUnarchived, owner, recipient, t1Old)
	f.ownedProps.add(recipient, pOwn, t2New) // own property occupies the single slot
	f.limiter.set(recipient, 1)              // used=1 >= limit=1 → suspend on unarchive

	if err := f.coordinator.EnforceOnUnarchiveForProperty(context.Background(), noopTx{}, pUnarchived); err != nil {
		t.Fatalf("EnforceOnUnarchiveForProperty: %v", err)
	}

	f.assertStatus(t, mUnarchived, pUnarchived, domain.MemberStatusSuspended, "unarchived member suspended when no free slot")
}
