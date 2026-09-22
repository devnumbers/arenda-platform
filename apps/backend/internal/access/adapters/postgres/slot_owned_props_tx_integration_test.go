package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	propertiespg "github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters/postgres"
)

// This integration test guards the slot coordinator's transaction visibility
// (issue #158, T4). The coordinator must read the recipient's own-property pool
// from the SAME transaction as the operation in progress; otherwise, under Read
// Committed, an own object just archived inside ArchiveProperty is still seen as
// active on a separate connection, usedSlots reports a slot still occupied,
// freeSlots is 0, and a suspended shared membership is never reactivated.
// Skipped when TEST_DATABASE_URL is unset (same convention as the other access
// integration tests).
//
// To reproduce the production condition precisely, the own-properties port is
// built over the POOL (a separate connection from the test transaction) — the
// adapter is injected once at startup over the pool and only bound to the
// caller's tx via WithTx inside the coordinator. Setup data is COMMITTED, then
// the archive + recovery run in a second (rolled-back) transaction: that way
// the pool sees the committed active property but not the uncommitted archive,
// exactly as in production.

// TestSlotCoordinator_RecoverSuspended_OwnArchiveFreesSlotTxVisible is the
// regression test for the bug where archiving one of the owner's OWN objects
// inside the ArchiveProperty transaction did not reactivate the owner's own
// suspended shared membership: the own-property pool was read from a separate
// Read-Committed connection that still saw the just-archived (uncommitted)
// object as active, so usedSlots reported 1 occupied slot, freeSlots was 0, and
// recovery was a no-op. The fix binds the own-properties port to the caller's
// transaction via WithTx.
func TestSlotCoordinator_RecoverSuspended_OwnArchiveFreesSlotTxVisible(t *testing.T) {
	t.Parallel()
	pool := setupAccessDB(t)
	ctx := context.Background()

	// --- Setup (committed): recipient owns one active property and holds one
	// suspended shared membership on another owner's object. Tariff limit is 1,
	// so the recipient's pool (1 own) is full and the shared membership stays
	// suspended until a slot frees.
	setupTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin setup tx: %v", err)
	}
	setupQ := genpostgres.New(setupTx)
	recipient := createAccessTestUser(t, ctx, setupQ)
	otherOwner := createAccessTestUser(t, ctx, setupQ)
	ownProperty := createActiveProperty(t, ctx, setupQ, recipient, "Своя квартира")
	sharedProperty := createActiveProperty(t, ctx, setupQ, otherOwner, "Москва тест")
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatalf("commit setup: %v", err)
	}

	// Suspended membership on the shared object (committed).
	suspendedAt := time.Now()
	setupTx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin setup tx 2: %v", err)
	}
	memberID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := NewMembershipRepository(setupTx2).CreateWithStatus(ctx, accessdomain.Membership{
		ID: memberID, PropertyID: sharedProperty, UserID: recipient, Role: accessdomain.RoleViewer,
		GrantedBy: otherOwner, Status: accessdomain.MemberStatusSuspended, SuspendedAt: &suspendedAt,
	}); err != nil {
		t.Fatalf("create suspended membership: %v", err)
	}
	if err := setupTx2.Commit(ctx); err != nil {
		t.Fatalf("commit setup tx 2: %v", err)
	}

	// --- Test transaction: archive own property, then recover (like
	// ArchiveProperty calling RecoverSuspended(actor) after archivePropertyInTx
	// in the same tx).
	testTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin test tx: %v", err)
	}
	defer func() {
		if err := testTx.Rollback(ctx); err != nil {
			// Isolation guard: a failed rollback would persist the test's
			// writes in the shared database.
			t.Errorf("rollback test tx: %v", err)
		}
	}()
	testQ := genpostgres.New(testTx)

	// Own-properties port over the POOL (production wiring: injected once at
	// startup over the pool, bound to the caller's tx only via WithTx).
	ownedProps := NewOwnedActivePropertiesAdapter(propertiespg.NewPropertyRepository(pool))
	limiter := &accessFakeLimiter{limits: map[uuid.UUID]int{recipient: 1}}
	slots := accessapp.NewSlotCoordinator(
		NewMembershipRepository(testTx),
		NewOwnerResolver(testTx),
		limiter,
		ownedProps,
		nil,
		auditapp.Noop{},
		accessBeginner{tx: testTx},
		nil,
		nil,
	)

	if _, err := testQ.ArchiveProperty(ctx, genpostgres.ArchivePropertyParams{
		ID: pgUUID(ownProperty), OwnerID: pgUUID(recipient),
	}); err != nil {
		t.Fatalf("ArchiveProperty: %v", err)
	}
	if err := slots.RecoverSuspended(ctx, accessNoCommitTx{testTx}, recipient); err != nil {
		t.Fatalf("RecoverSuspended: %v", err)
	}

	// The suspended membership on the shared object must be reactivated: the
	// own-property slot freed by the archive is visible to recovery because the
	// own-properties port is bound to the same transaction.
	active, err := NewMembershipRepository(testTx).ListActiveByUser(ctx, recipient)
	if err != nil {
		t.Fatalf("ListActiveByUser: %v", err)
	}
	found := false
	for _, m := range active {
		if m.PropertyID == sharedProperty {
			found = true
		}
	}
	if !found {
		t.Errorf("expected shared property %s to be active after own-object archive (tx-bound own-properties), active = %v",
			sharedProperty, active)
	}
}

// createActiveProperty inserts an active property via q (caller controls the tx).
func createActiveProperty(t *testing.T, ctx context.Context, q *genpostgres.Queries, owner uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	if _, err := q.CreateProperty(ctx, genpostgres.CreatePropertyParams{
		ID: pgUUID(id), OwnerID: pgUUID(owner), Name: name, Type: propertyTypeApartment,
		Address: "", Description: pgtype.Text{}, Attributes: []byte("{}"), Status: statusActive,
	}); err != nil {
		t.Fatalf("create property: %v", err)
	}
	return id
}

// Compile-time check: the pool-backed adapter must still satisfy the port.
var _ accessapp.OwnedActivePropertiesPort = (*OwnedActivePropertiesAdapter)(nil)

// keep imports referenced even when only the compile-time var above uses some.
var (
	_ *pgxpool.Pool
	_ pgx.Tx
)
