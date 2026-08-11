package postgres

import (
	"testing"

	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
)

// TestSharingAudit_FreeReminders_IncludesShared verifies the fix for the
// free-reminders aggregate list: a member sees free reminders of shared
// properties (GET /free-reminders), not just their own.
func TestSharingAudit_FreeReminders_IncludesShared(t *testing.T) {
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newFreeReminderPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	member := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addPolicyMembership(t, ctx, f.members, property, member, owner, accessdomain.RoleFullAccess)

	// Owner creates a free reminder (lands on owner's scope).
	created, err := f.service().Create(ctx, owner, owner, f.createInput(property))
	if err != nil {
		t.Fatalf("seed free reminder: %v", err)
	}

	// The member — with full access — sees it via the aggregate list.
	memberRems, err := f.serviceWithShared().ListByOwner(ctx, member, 100, 0)
	if err != nil {
		t.Fatalf("ListByOwner as member: %v", err)
	}
	found := false
	for _, r := range memberRems {
		if r.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("member did not see shared free reminder %s (got %d reminders)", created.ID, len(memberRems))
	}

	// Without the shared adapter injected, the member sees nothing (pre-T3 guard).
	noneRems, err := f.service().ListByOwner(ctx, member, 100, 0)
	if err != nil {
		t.Fatalf("ListByOwner (no shared) as member: %v", err)
	}
	if len(noneRems) != 0 {
		t.Errorf("member without shared adapter saw %d free reminders, expected 0", len(noneRems))
	}
}

// TestSharingAudit_Reminders_IncludesShared verifies the fix for the reminders
// aggregate list (GET /reminders): a member sees reminders of shared
// properties, not just their own.
func TestSharingAudit_Reminders_IncludesShared(t *testing.T) {
	pool := setupPolicyDB(t)
	ctx, tx, cleanup := beginPolicyTx(t, pool)
	defer cleanup()

	f := newReminderPolicyFixture(tx)
	owner := createPolicyTestUser(t, ctx, f.q)
	member := createPolicyTestUser(t, ctx, f.q)
	property := createPolicyTestProperty(t, ctx, f.q, owner)
	addPolicyMembership(t, ctx, f.members, property, member, owner, accessdomain.RoleFullAccess)

	// Seed a free-reminder concrete row on the owner's scope (has property_id).
	seeded := f.seedReminder(t, ctx, owner, property)

	// The member — with full access — sees it via the aggregate list.
	memberRems, err := f.serviceWithShared().ListByOwner(ctx, member, application.ListFilter{Limit: 100})
	if err != nil {
		t.Fatalf("ListByOwner as member: %v", err)
	}
	found := false
	for _, r := range memberRems {
		if r.ID == seeded.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("member did not see shared reminder %s (got %d reminders)", seeded.ID, len(memberRems))
	}

	// Without the shared adapter injected, the member sees nothing (pre-T3 guard).
	noneRems, err := f.service().ListByOwner(ctx, member, application.ListFilter{Limit: 100})
	if err != nil {
		t.Fatalf("ListByOwner (no shared) as member: %v", err)
	}
	if len(noneRems) != 0 {
		t.Errorf("member without shared adapter saw %d reminders, expected 0", len(noneRems))
	}
}
