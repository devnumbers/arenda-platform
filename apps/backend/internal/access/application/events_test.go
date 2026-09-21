package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
)

// The access lifecycle events' capture tests (карта #734, #751): the real
// AccessService / InvitationService over the in-memory ports (the same seam
// as service_integration_test.go), with a recording publisher at the
// AccessEventPublisher port. The publications are post-commit best-effort:
// a transition succeeds even when the publisher fails.

// eventsFixture bundles the access and invitation services wired with the
// recording event publisher and the slot coordinator.
type eventsFixture struct {
	repo    *memRepo
	owners  staticResolver
	lookup  *stubLookup
	limiter *fakeRecipientLimiter
	events  *fakeEventPublisher
	clk     *fixedClock
	access  *AccessService
	invites *InvitationService
}

// linkOwner records the property's owner in both owner views (the repo join
// and the resolver), the same shape the coordinator fixture's linkOwner takes.
func (f *eventsFixture) linkOwner(ownerID, propertyID uuid.UUID) {
	f.repo.SetOwner(propertyID, ownerID)
	f.owners[propertyID] = ownerID
}

func newEventsFixture() *eventsFixture {
	repo := newMemRepo()
	invitations := &memInvitationsRepo{}
	owners := staticResolver{}
	statuses := fakeStatuses{}
	policy := NewMembershipPolicy(owners, repo)
	lookup := &stubLookup{}
	clk := &fixedClock{now: time.Now()}
	limiter := newFakeRecipientLimiter()
	events := &fakeEventPublisher{}
	coordinator := NewSlotCoordinator(repo, owners, limiter, newFakeOwnedProps(),
		events, auditapp.Noop{}, noopBeginner{}, clk, nil)
	factory := newTestFactory(repo, invitations, auditapp.Noop{})
	access := NewAccessService(repo, owners, statuses, lookup, policy, coordinator,
		events, factory, nil)
	invites := NewInvitationService(access, repo, invitations, owners, statuses, lookup, policy,
		coordinator, nil, events, fakeTitles("Квартира на Невском"), factory, clk, nil)
	return &eventsFixture{
		repo: repo, owners: owners, lookup: lookup, limiter: limiter, events: events, clk: clk,
		access: access, invites: invites,
	}
}

// TestAddMemberActivePublishesNothing checks that a grant the recipient's slot
// accommodates stays silent — the catalog has no "access granted" event.
func TestAddMemberActivePublishesNothing(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, property)
	f.limiter.set(member, 5)

	if _, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if got := len(f.events.events); got != 0 {
		t.Errorf("expected no events for an active grant, got %d: %+v", got, f.events.events)
	}
}

// TestAddMemberSuspendedPublishesSystemPause checks the no-slot grant: the
// system «Доступ приостановлен» event (no actor) with the recipient and the
// transition instant.
func TestAddMemberSuspendedPublishesSystemPause(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, property)
	f.limiter.set(member, 0)

	m, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if m.Status != domain.MemberStatusSuspended {
		t.Fatalf("member must land suspended, got %v", m.Status)
	}
	if got := f.events.count("membership_suspended"); got != 1 {
		t.Fatalf("expected exactly one membership_suspended event, got %d", got)
	}
	e := f.events.last(t, "membership_suspended").pause
	if e.MembershipID != m.ID || e.PropertyID != property || e.RecipientID != member {
		t.Errorf("event ids = (%s, %s, %s)", e.MembershipID, e.PropertyID, e.RecipientID)
	}
	if e.ActorID != uuid.Nil {
		t.Errorf("a slot suspension has no actor, got %s", e.ActorID)
	}
	if e.SuspendedAt.IsZero() {
		t.Error("the event must carry the transition instant for the dedup key")
	}
}

// TestAddMemberSuspendedPublishFailureSwallowed checks the best-effort canon:
// a publisher failure never fails the committed transition.
func TestAddMemberSuspendedPublishFailureSwallowed(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, property)
	f.limiter.set(member, 0)
	f.events.errFor = map[string]error{"membership_suspended": errors.New("queue down")}

	if _, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember must survive a publish failure, got %v", err)
	}
}

// TestRevokeMemberPublishesRevoked checks the active membership's revoke: the
// event names the removed member as the recipient and the revoking owner as
// the actor.
func TestRevokeMemberPublishesRevoked(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, property)
	f.limiter.set(member, 5)

	m, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if err := f.access.RevokeMember(t.Context(), owner, property, m.ID); err != nil {
		t.Fatalf("RevokeMember: %v", err)
	}
	e := f.events.last(t, "membership_revoked").revo
	if e.MembershipID != m.ID || e.RecipientID != member || e.ActorID != owner {
		t.Errorf("event = (%s, %s, %s), want (%s, %s, %s)",
			e.MembershipID, e.RecipientID, e.ActorID, m.ID, member, owner)
	}
}

// TestRevokeSuspendedPublishesNothing checks the #162 canon: revoking a
// suspended membership is silent — the object was already hidden from the
// recipient.
func TestRevokeSuspendedPublishesNothing(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, property)
	f.limiter.set(member, 0)

	m, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	f.events.events = nil
	if err := f.access.RevokeMember(t.Context(), owner, property, m.ID); err != nil {
		t.Fatalf("RevokeMember: %v", err)
	}
	if got := len(f.events.events); got != 0 {
		t.Errorf("a suspended revoke publishes nothing, got %d events", got)
	}
}

// TestLeavePropertyPublishesMemberLeft checks the self-exit: the owner is the
// recipient, the leaving member the actor.
func TestLeavePropertyPublishesMemberLeft(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, property)
	f.limiter.set(member, 5)

	if _, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleFullAccess); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if err := f.access.LeaveProperty(t.Context(), member, property); err != nil {
		t.Fatalf("LeaveProperty: %v", err)
	}
	e := f.events.last(t, "member_left").left
	if e.PropertyID != property || e.OwnerID != owner || e.MemberID != member {
		t.Errorf("event = (%s, %s, %s)", e.PropertyID, e.OwnerID, e.MemberID)
	}
}

// TestActivateInvitationPublishesActivated checks the invitation activation:
// the event carries the inviter (the accepted-event recipient), the invitee
// (its actor) and the landing state.
func TestActivateInvitationPublishesActivated(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		limit     int
		suspended bool
	}{
		{"active landing", 5, false},
		{"suspended landing", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newEventsFixture()
			inviter, invitee, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
			f.linkOwner(inviter, property)
			f.limiter.set(invitee, tc.limit)
			email := "invitee@example.com"

			invitation := domain.Invitation{
				ID:         uuid.Must(uuid.NewV7()),
				PropertyID: property,
				Email:      email,
				Role:       domain.RoleViewer,
				InvitedBy:  inviter,
				LastSentAt: f.clk.Now(),
			}
			if err := f.invites.runInTx(context.Background(), func(stores *txStores) error {
				_, err := stores.invitations.Create(context.Background(), invitation)
				return err
			}); err != nil {
				t.Fatalf("seed invitation: %v", err)
			}

			if err := f.invites.ActivatePendingInvitations(t.Context(), invitee, email); err != nil {
				t.Fatalf("ActivatePendingInvitations: %v", err)
			}
			e := f.events.last(t, "invitation_activated").email
			if e.InviterID != inviter || e.InviteeID != invitee || e.PropertyID != property {
				t.Errorf("event parties = (%s, %s, %s)", e.InviterID, e.InviteeID, e.PropertyID)
			}
			if e.MembershipID == uuid.Nil {
				t.Error("the event must carry the created membership id")
			}
			if e.Suspended != tc.suspended {
				t.Errorf("Suspended = %v, want %v", e.Suspended, tc.suspended)
			}
			if e.At.IsZero() {
				t.Error("the event must carry the activation instant")
			}
		})
	}
}
