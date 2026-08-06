package postgres

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// These integration tests run against a real Postgres via TEST_DATABASE_URL and
// are skipped when it is unset (same convention as the membership repository
// integration tests).

// ListActiveRecipientIDs feeds the reminder fan-out (issue #159): it must
// return the active members of the property and exclude suspended ones.
func TestMemberRecipientAdapter_ListActiveRecipientIDs(t *testing.T) {
	pool := setupAccessDB(t)
	ctx, tx, cleanup := beginAccessTx(t, pool)
	defer cleanup()

	q := genpostgres.New(tx)
	owner := createAccessTestUser(t, ctx, q)
	activeMember := createAccessTestUser(t, ctx, q)
	suspendedMember := createAccessTestUser(t, ctx, q)
	property := createAccessTestProperty(t, ctx, q, owner)

	repo := NewMembershipRepository(tx)
	activeID, _ := uuid.NewV7()
	if _, err := repo.Create(ctx, domain.Membership{
		ID: activeID, PropertyID: property, UserID: activeMember, Role: domain.RoleViewer, GrantedBy: owner,
	}); err != nil {
		t.Fatalf("create active membership: %v", err)
	}
	suspendedID, _ := uuid.NewV7()
	if _, err := repo.CreateWithStatus(ctx, domain.Membership{
		ID: suspendedID, PropertyID: property, UserID: suspendedMember, Role: domain.RoleFullAccess,
		GrantedBy: owner, Status: domain.MemberStatusSuspended,
	}); err != nil {
		t.Fatalf("create suspended membership: %v", err)
	}

	adapter := NewMemberRecipientAdapter(repo)
	ids, err := adapter.ListActiveRecipientIDs(ctx, property)
	if err != nil {
		t.Fatalf("ListActiveRecipientIDs: %v", err)
	}

	if len(ids) != 1 || ids[0] != activeMember {
		t.Errorf("ListActiveRecipientIDs = %v, want [%v] (suspended member excluded)", ids, activeMember)
	}
	if slices.Contains(ids, suspendedMember) {
		t.Errorf("ListActiveRecipientIDs must not include the suspended member %v", suspendedMember)
	}
	if slices.Contains(ids, owner) {
		t.Errorf("ListActiveRecipientIDs must not include the owner %v (added by the worker)", owner)
	}
}
