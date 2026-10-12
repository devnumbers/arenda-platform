package application

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
)

// This file covers the global «Отозвать доступ всем» (issue #1310): one call
// closes the owner's whole sharing — every membership and every pending
// invitation on all of their properties, archived included — with the
// per-item effects of the single revoke and the invitation cancellation, the
// FIFO recovery per affected recipient, and the idempotent no-op on an empty
// state.

// fakeRevokeAllScope is the gate double: the fixture default speaks owner;
// the gate tests flip the probes.
type fakeRevokeAllScope struct {
	owns  bool
	holds bool
}

func (f *fakeRevokeAllScope) OwnsAnyProperty(context.Context, uuid.UUID) (bool, error) {
	return f.owns, nil
}

func (f *fakeRevokeAllScope) HoldsAnyMembership(context.Context, uuid.UUID) (bool, error) {
	return f.holds, nil
}

// seedMembership creates one membership row in the in-memory repo.
func seedMembership(t *testing.T, f *mutationFixture, propertyID, userID uuid.UUID, status domain.MemberStatus) domain.Membership {
	t.Helper()
	m := domain.Membership{
		ID:         uuid.Must(uuid.NewV7()),
		PropertyID: propertyID,
		UserID:     userID,
		Role:       domain.RoleViewer,
		GrantedBy:  f.owner,
		Status:     status,
	}
	var err error
	if status == domain.MemberStatusSuspended {
		_, err = f.repo.CreateWithStatus(t.Context(), m)
	} else {
		_, err = f.repo.Create(t.Context(), m)
	}
	if err != nil {
		t.Fatalf("seed membership on %s: %v", propertyID, err)
	}
	return m
}

// seedInvitation creates one pending invitation row in the in-memory repo.
func seedInvitation(t *testing.T, f *mutationFixture, propertyID uuid.UUID, email string) domain.Invitation {
	t.Helper()
	inv, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID:         uuid.Must(uuid.NewV7()),
		PropertyID: propertyID,
		Email:      email,
		Role:       domain.RoleViewer,
		InvitedBy:  f.owner,
		LastSentAt: f.clk.now,
		CreatedAt:  f.clk.now,
	})
	if err != nil {
		t.Fatalf("seed invitation on %s: %v", propertyID, err)
	}
	return inv
}

// legsOf filters the in-memory rows by (user, property) — the leg-existence
// assertion helper.
func legsOf(r *memRepo, userID, propertyID uuid.UUID) []domain.Membership {
	var out []domain.Membership
	for _, m := range r.rows {
		if m.UserID == userID && m.PropertyID == propertyID {
			out = append(out, m)
		}
	}
	return out
}

// mixedSharingFixture is the seeded state of the multi-property revoke test:
// the owner's two properties (p1 archived) plus a foreign one, two members,
// one pending invitation per owner.
type mixedSharingFixture struct {
	p1, p2, p3       uuid.UUID
	memberA, memberB uuid.UUID
	other            uuid.UUID
}

// seedMixedSharing seeds the fixture: A holds active legs on p1/p2 and a
// foreign one on p3, B a suspended leg on p2; the owner has one pending
// invitation, the foreign owner another.
func seedMixedSharing(t *testing.T, f *mutationFixture) mixedSharingFixture {
	t.Helper()
	out := mixedSharingFixture{
		p1:    uuid.Must(uuid.NewV7()),
		p2:    uuid.Must(uuid.NewV7()),
		p3:    uuid.Must(uuid.NewV7()),
		other: uuid.Must(uuid.NewV7()),
	}
	f.addProperty(out.p1, f.owner, testFirstTitle)
	f.addProperty(out.p2, f.owner, testSecondTitle)
	f.addProperty(out.p3, out.other, testForeignTitle)
	f.statuses[out.p1] = true // Archived objects hold revocable legs too (issue #163).
	out.memberA = uuid.Must(uuid.NewV7())
	out.memberB = uuid.Must(uuid.NewV7())
	f.lookup.byID[out.memberA] = MemberUser{ID: out.memberA, Name: new("Аня"), Surname: new("Иванова")}
	f.lookup.byID[out.memberB] = MemberUser{ID: out.memberB, Name: new("Борис")}
	f.emails[out.memberA] = testMemberEmail

	seedMembership(t, f, out.p1, out.memberA, domain.MemberStatusActive)
	seedMembership(t, f, out.p2, out.memberA, domain.MemberStatusActive)
	seedMembership(t, f, out.p2, out.memberB, domain.MemberStatusSuspended)
	seedMembership(t, f, out.p3, out.memberA, domain.MemberStatusActive) // Foreign owner's leg: stays.
	seedInvitation(t, f, out.p1, testPendingEmail)
	seedInvitation(t, f, out.p3, testPendingEmail) // Foreign owner's invitation: stays.
	return out
}

func TestParticipantMutation_RevokeAll_RemovesEveryLegAcrossOwnedProperties(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	s := seedMixedSharing(t, f)

	if err := f.svc.RevokeAll(t.Context(), f.owner); err != nil {
		t.Fatalf("revoke all: %v", err)
	}

	// Every leg on the actor's properties is gone; foreign legs stay.
	if got := legsOf(f.repo, s.memberA, s.p1); len(got) != 0 {
		t.Errorf("p1 membership of A must be removed, found %+v", got)
	}
	if _, err := f.repo.GetByPropertyAndUser(t.Context(), s.p2, s.memberA); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("p2 membership of A must be removed, got %v", err)
	}
	if _, err := f.repo.GetByPropertyAndUser(t.Context(), s.p2, s.memberB); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("p2 suspended membership of B must be removed, got %v", err)
	}
	if _, err := f.repo.GetByPropertyAndUser(t.Context(), s.p3, s.memberA); err != nil {
		t.Errorf("foreign-owner membership must stay, got %v", err)
	}
	if len(f.invitations.rows) != 1 || f.invitations.rows[0].PropertyID != s.p3 {
		t.Errorf("invitations = %+v, want only the foreign-owner one", f.invitations.rows)
	}
	assertPerItemAudit(t, f, 3, 1)
	assertNoEmails(t, f)
}

// assertPerItemAudit checks the audit and journal shapes: one entry per
// removed row, mirroring the per-item flows — the membership removals and
// the invitation cancellations counted separately.
func assertPerItemAudit(t *testing.T, f *mutationFixture, wantRemoved, wantCancelled int) {
	t.Helper()
	if got := len(f.audit.actions(auditdomain.ActionPropertyMemberRemoved)); got != wantRemoved {
		t.Errorf("membership audit entries = %d, want %d", got, wantRemoved)
	}
	if got := len(f.audit.actions(auditdomain.ActionPropertyMemberInvitationCancelled)); got != wantCancelled {
		t.Errorf("invitation audit entries = %d, want %d", got, wantCancelled)
	}
	// The journal tells the per-item story: «участник удалён» per membership
	// leg, «приглашение отменено» per invitation — no new action kinds.
	var removedRows, cancelledRows int
	for _, e := range f.history.Entries {
		if e.Action == historydomain.ActionMemberRemoved {
			removedRows++
		}
		if e.Action == historydomain.ActionMemberInvitationCancelled {
			cancelledRows++
		}
	}
	if removedRows != wantRemoved || cancelledRows != wantCancelled {
		t.Errorf("journal rows: removed=%d (want %d), invitation_cancelled=%d (want %d)",
			removedRows, wantRemoved, cancelledRows, wantCancelled)
	}
}

// assertNoEmails pins the no-lifecycle-email rule: the notifications leave
// as events (issue #695), the mailer stays silent.
func assertNoEmails(t *testing.T, f *mutationFixture) {
	t.Helper()
	if f.mailer.count("access_revoked") != 0 || f.mailer.count("invite") != 0 {
		t.Errorf("revoke-all must not send emails, sent %+v", f.mailer.sent)
	}
}

func TestParticipantMutation_RevokeAll_IdempotentNoOpOnEmptyState(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1 := uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testFirstTitle)

	// First call on an owner with no participants: success.
	if err := f.svc.RevokeAll(t.Context(), f.owner); err != nil {
		t.Fatalf("first revoke all: %v", err)
	}
	// Second call after everything is gone: the repeated no-op.
	if err := f.svc.RevokeAll(t.Context(), f.owner); err != nil {
		t.Fatalf("repeat revoke all: %v", err)
	}

	if len(f.audit.entries) != 0 {
		t.Errorf("no-op must write no audit entries, got %+v", f.audit.entries)
	}
	if len(f.history.Entries) != 0 {
		t.Errorf("no-op must write no journal rows, got %+v", f.history.Entries)
	}
	if got := len(f.events.events); got != 0 {
		t.Errorf("no-op must publish no events, got %+v", f.events.events)
	}
	if got := len(f.realtime.Publications); got != 0 {
		t.Errorf("no-op must dispatch no frames, got %+v", f.realtime.Publications)
	}
	if f.mailer.count("access_revoked") != 0 {
		t.Errorf("no-op must send no emails, sent %+v", f.mailer.sent)
	}
}

func TestParticipantMutation_RevokeAll_EffectsMatchPerItemFlows(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1, p4 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testFirstTitle)
	f.addProperty(p4, other, testForeignTitle)
	member := uuid.Must(uuid.NewV7())
	f.lookup.byID[member] = MemberUser{ID: member, Name: new("Аня")}
	f.emails[member] = testMemberEmail
	f.limiter.set(member, 1)
	// The member's only slot is occupied by the leg on the actor's object; a
	// suspended leg on another owner's object waits in the FIFO queue.
	active := seedMembership(t, f, p1, member, domain.MemberStatusActive)
	seedMembership(t, f, p4, member, domain.MemberStatusSuspended)
	inv := seedInvitation(t, f, p1, testPendingEmail)

	if err := f.svc.RevokeAll(t.Context(), f.owner); err != nil {
		t.Fatalf("revoke all: %v", err)
	}

	// The freed slot recovers the person's suspended access FIFO — the same
	// effect the single revoke and the batch Remove produce (issue #158, T4).
	recovered, err := f.repo.GetByPropertyAndUser(t.Context(), p4, member)
	if err != nil || recovered.Status != domain.MemberStatusActive {
		t.Errorf("suspended leg must be recovered FIFO, got %+v (%v)", recovered, err)
	}

	// The active leg speaks the revocation event with the actor; the
	// suspended leg and the invitation speak nothing (issue #162 canon) —
	// no duplicate notifications.
	if got := f.events.count(kindMembershipRevoked); got != 1 {
		t.Fatalf("membership_revoked events = %d, want exactly 1 (the active leg)", got)
	}
	revoke := f.events.last(t, kindMembershipRevoked).revo
	if revoke.MembershipID != active.ID || revoke.PropertyID != p1 || revoke.RecipientID != member {
		t.Errorf("revoke event = %+v, want the active leg %s on %s for %s", revoke, active.ID, p1, member)
	}
	if revoke.ActorID != f.owner {
		t.Errorf("revoke actor = %s, want the owner %s", revoke.ActorID, f.owner)
	}

	// One realtime dispatch dirtying every touched object's participants
	// view: the two removed legs' objects plus the recovered one, and the
	// two p1 journal rows piggyback their history pairs (the recovered p4
	// leg journals no row — the coordinator writes none) — the same batch
	// shape publishRemovedRealtime gives the single Remove.
	pairs := f.realtime.Pairs()
	slices.Sort(pairs)
	want := []string{
		"access:" + p1.String(), "access:" + p1.String(), "access:" + p4.String(),
		"history:" + p1.String(), "history:" + p1.String(),
	}
	if !slices.Equal(pairs, want) {
		t.Errorf("realtime pairs = %v, want %v", pairs, want)
	}

	// The cancelled invitation's audit entry carries the invitation id — the
	// same shape CancelInvitation writes.
	cancelled := f.audit.actions(auditdomain.ActionPropertyMemberInvitationCancelled)
	if len(cancelled) != 1 || cancelled[0].EntityID == nil || *cancelled[0].EntityID != inv.ID {
		t.Errorf("invitation audit entries = %+v, want one for %s", cancelled, inv.ID)
	}
}

// TestParticipantMutation_RevokeAll_GateRefusesNonOwnerAndStranger pins the
// gate's verdict table (issue #1310): the probes, not the revocation,
// decide — owner proceeds, the non-owner participant gets the loud refusal,
// the stranger the privacy-preserving not-found; neither refusal touches a
// leg.
func TestParticipantMutation_RevokeAll_GateRefusesNonOwnerAndStranger(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1 := uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	f.addProperty(p1, other, testForeignTitle)
	participant := uuid.Must(uuid.NewV7())
	f.lookup.byID[participant] = MemberUser{ID: participant, Name: new("Вика")}
	seedMembership(t, f, p1, participant, domain.MemberStatusActive)

	for _, tc := range []struct {
		name      string
		owns      bool
		holds     bool
		actor     uuid.UUID
		wantError error
	}{
		{
			name:  "owner proceeds",
			owns:  true,
			actor: f.owner,
		},
		{
			name:      "non-owner participant refused loudly",
			owns:      false,
			holds:     true,
			actor:     participant,
			wantError: domain.ErrNotPropertyOwner,
		},
		{
			name:      "stranger gets the privacy not-found",
			owns:      false,
			holds:     false,
			actor:     uuid.Must(uuid.NewV7()),
			wantError: domain.ErrParticipantNotFound,
		},
	} {
		f.scope.owns, f.scope.holds = tc.owns, tc.holds
		err := f.svc.RevokeAll(t.Context(), tc.actor)
		if !errors.Is(err, tc.wantError) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.wantError)
		}
	}

	// No call removed the leg on the foreign object.
	if got := legsOf(f.repo, participant, p1); len(got) != 1 {
		t.Errorf("gate must not remove legs, found %+v", got)
	}
}
