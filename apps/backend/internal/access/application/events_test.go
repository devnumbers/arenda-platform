package application

import (
	"context"
	"errors"
	"slices"
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
	lookup  *fakeLookup
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
	lookup := newFakeLookup()
	clk := &fixedClock{now: time.Now()}
	limiter := newFakeRecipientLimiter()
	events := &fakeEventPublisher{}
	coordinator := NewSlotCoordinator(repo, owners, limiter, newFakeOwnedProps(),
		events, auditapp.Noop{}, noopBeginner{}, clk, nil)
	factory := newTestFactory(repo, invitations, auditapp.Noop{})
	access := NewAccessService(repo, owners, statuses, lookup, policy, coordinator,
		events, factory, nil)
	invites := NewInvitationService(access, repo, invitations, owners, statuses, lookup, policy,
		coordinator, nil, events, fakeTitles("Квартира на Невском"), factory, clk, nil, nil)
	return &eventsFixture{
		repo: repo, owners: owners, lookup: lookup, limiter: limiter, events: events, clk: clk,
		access: access, invites: invites,
	}
}

// TestAddMemberActivePublishesGranted checks the instant landing's event
// (issue #829): an active grant notifies the new member — the event carries
// the created membership, the property, the recipient and the granting actor
// (the subscriber renders the №5 «Приглашение в объект» row).
func TestAddMemberActivePublishesGranted(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, property)
	f.limiter.set(member, 5)

	m, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	e := f.events.last(t, kindMembershipGranted).grant
	if e.MembershipID != m.ID || e.PropertyID != property || e.RecipientID != member {
		t.Errorf("event ids = (%s, %s, %s), want (%s, %s, %s)",
			e.MembershipID, e.PropertyID, e.RecipientID, m.ID, property, member)
	}
	if e.ActorID != owner {
		t.Errorf("event actor = %s, want the granting owner %s", e.ActorID, owner)
	}
}

// TestAddMemberGrantedPublishFailureSwallowed checks the best-effort canon on
// the instant landing's event: a publisher failure never fails the committed
// transition.
func TestAddMemberGrantedPublishFailureSwallowed(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, property)
	f.limiter.set(member, 5)
	f.events.errFor = map[string]error{kindMembershipGranted: errors.New("queue down")}

	if _, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember must survive a publish failure, got %v", err)
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

// TestInviteRegisteredUserPublishesGranted checks the InviteByEmail input of
// the instant landing (issue #829): a registered email lands active through
// AddMember and the invitee is notified — with no invite email and no
// «Приглашение принято», an event that exists only for an acceptance.
func TestInviteRegisteredUserPublishesGranted(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	inviter, invitee, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(inviter, property)
	f.limiter.set(invitee, 5)
	email := "invitee@example.com"
	f.lookup.add(email, invitee)

	outcome, err := f.invites.InviteByEmail(t.Context(), inviter, property, email, domain.RoleViewer)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	if outcome.Member == nil {
		t.Fatalf("a registered email must land a membership, got %+v", outcome)
	}
	if f.events.count("invitation_activated") != 0 {
		t.Error("an instant landing publishes no activation event — there was no acceptance")
	}
	e := f.events.last(t, kindMembershipGranted).grant
	if e.MembershipID != outcome.Member.ID || e.PropertyID != property ||
		e.RecipientID != invitee || e.ActorID != inviter {
		t.Errorf("granted event = %+v, want the landing membership %s for %s by %s",
			e, outcome.Member.ID, invitee, inviter)
	}
}

// TestChangeMemberRolePublishesRoleChanged checks the role-change event
// (карта #828, #830): the changed member learns the manager's change — the
// event carries the membership, the property, the recipient, the changing
// actor, the new role and the change instant (the row's updated_at) the
// dedup key stamps.
func TestChangeMemberRolePublishesRoleChanged(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, property)
	f.limiter.set(member, 5)

	m, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	f.events.events = nil
	updated, err := f.access.ChangeMemberRole(t.Context(), owner, property, m.ID, domain.RoleFullAccess)
	if err != nil {
		t.Fatalf("ChangeMemberRole: %v", err)
	}
	e := f.events.last(t, kindMembershipRoleChanged).role
	if e.MembershipID != m.ID || e.PropertyID != property || e.RecipientID != member || e.ActorID != owner {
		t.Errorf("role-changed event = %+v, want membership %s for %s by %s",
			e, m.ID, member, owner)
	}
	if e.Role != domain.RoleFullAccess {
		t.Errorf("event role = %v, want the new role full_access", e.Role)
	}
	if e.ChangedAt.IsZero() || !e.ChangedAt.Equal(updated.UpdatedAt) {
		t.Errorf("event ChangedAt = %v, want the row's updated_at %v — the dedup key's stamp", e.ChangedAt, updated.UpdatedAt)
	}
}

// TestChangeMemberRoleSuspendedPublishesNothing checks the #162 canon on the
// role change (тикет #830): a suspended membership's role change is silent —
// the object was already hidden from the recipient.
func TestChangeMemberRoleSuspendedPublishesNothing(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, property)
	f.limiter.set(member, 0)

	m, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if !m.IsSuspended() {
		t.Fatalf("member must land suspended, got %v", m.Status)
	}
	f.events.events = nil
	if _, err := f.access.ChangeMemberRole(t.Context(), owner, property, m.ID, domain.RoleFullAccess); err != nil {
		t.Fatalf("ChangeMemberRole: %v", err)
	}
	if got := len(f.events.events); got != 0 {
		t.Errorf("a suspended membership's role change publishes nothing, got %d events", got)
	}
}

// TestChangeMemberRolePublishFailureSwallowed checks the best-effort canon:
// a publisher failure never fails the committed transition.
func TestChangeMemberRolePublishFailureSwallowed(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member, property := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, property)
	f.limiter.set(member, 5)

	m, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	f.events.errFor = map[string]error{kindMembershipRoleChanged: errors.New("queue down")}
	if _, err := f.access.ChangeMemberRole(t.Context(), owner, property, m.ID, domain.RoleFullAccess); err != nil {
		t.Fatalf("ChangeMemberRole must survive a publish failure, got %v", err)
	}
}

// kindMembershipGranted is the granted event's kind at the recording fake —
// the lookup key of the granted-event assertions and the errFor failure key.
const kindMembershipGranted = "membership_granted"

// kindMembershipRoleChanged is the role-change event's kind at the recording
// fake (карта #828, #830) — the same contract as kindMembershipGranted.
const kindMembershipRoleChanged = "membership_role_changed"

// fakeEventPublisher records the lifecycle events the services publish
// (карта #734, #751); errFor fails a kind on demand — the publications are
// best-effort, so a failure must never surface from the transition itself.
type fakeEventPublisher struct {
	events []recordedEvent
	errFor map[string]error
}

type recordedEvent struct {
	kind  string
	email *InvitationActivated
	pause *MembershipSuspended
	resum *MembershipResumed
	revo  *MembershipRevoked
	left  *MemberLeft
	grant *MembershipGranted
	role  *MembershipRoleChanged
}

func (f *fakeEventPublisher) PublishInvitationActivated(_ context.Context, e InvitationActivated) error {
	if err := f.errFor["invitation_activated"]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: "invitation_activated", email: &e})
	return nil
}

func (f *fakeEventPublisher) PublishMembershipSuspended(_ context.Context, e MembershipSuspended) error {
	if err := f.errFor["membership_suspended"]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: "membership_suspended", pause: &e})
	return nil
}

func (f *fakeEventPublisher) PublishMembershipResumed(_ context.Context, e MembershipResumed) error {
	if err := f.errFor["membership_resumed"]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: "membership_resumed", resum: &e})
	return nil
}

func (f *fakeEventPublisher) PublishMembershipRevoked(_ context.Context, e MembershipRevoked) error {
	if err := f.errFor["membership_revoked"]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: "membership_revoked", revo: &e})
	return nil
}

func (f *fakeEventPublisher) PublishMemberLeft(_ context.Context, e MemberLeft) error {
	if err := f.errFor["member_left"]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: "member_left", left: &e})
	return nil
}

func (f *fakeEventPublisher) PublishMembershipGranted(_ context.Context, e MembershipGranted) error {
	if err := f.errFor[kindMembershipGranted]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: kindMembershipGranted, grant: &e})
	return nil
}

func (f *fakeEventPublisher) PublishMembershipRoleChanged(_ context.Context, e MembershipRoleChanged) error {
	if err := f.errFor[kindMembershipRoleChanged]; err != nil {
		return err
	}
	f.events = append(f.events, recordedEvent{kind: kindMembershipRoleChanged, role: &e})
	return nil
}

var _ AccessEventPublisher = (*fakeEventPublisher)(nil)

// count returns how many events of the given kind were published.
func (f *fakeEventPublisher) count(kind string) int {
	n := 0
	for _, e := range f.events {
		if e.kind == kind {
			n++
		}
	}
	return n
}

// last returns the last recorded event of the given kind, failing the test
// otherwise.
func (f *fakeEventPublisher) last(t *testing.T, kind string) recordedEvent {
	t.Helper()
	if f.count(kind) == 0 {
		t.Fatalf("expected at least one %s event, got none (all: %+v)", kind, f.events)
	}
	for _, v := range slices.Backward(f.events) {
		if v.kind == kind {
			return v
		}
	}
	panic("unreachable")
}

// TestRevokeMemberRecoversSuspendedFIFO pins the #158 T4 recovery seam at the
// service level: revoking the recipient's ACTIVE membership frees their slot
// and the oldest suspended membership comes back active. The delete must hit
// the database before the recovery pass counts the free slots — an earlier
// recovery silently sees the freed slot as still occupied (regression guard
// for the removeMembershipInTx call order).
func TestRevokeMemberRecoversSuspendedFIFO(t *testing.T) {
	t.Parallel()
	f := newEventsFixture()
	owner, member := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	propA, propB := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.linkOwner(owner, propA)
	f.linkOwner(owner, propB)
	f.limiter.set(member, 1) // One slot: the first grant fits, the second suspends.

	active, err := f.access.AddMember(t.Context(), owner, propA, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember propA: %v", err)
	}
	if _, err := f.access.AddMember(t.Context(), owner, propB, member, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember propB: %v", err)
	}
	f.events.events = nil

	// Revoke the ACTIVE propA leg: its slot frees, propB's suspended one recovers.
	if err := f.access.RevokeMember(t.Context(), owner, propA, active.ID); err != nil {
		t.Fatalf("RevokeMember: %v", err)
	}

	revived, err := f.repo.GetByPropertyAndUser(t.Context(), propB, member)
	if err != nil {
		t.Fatalf("get propB membership: %v", err)
	}
	if revived.Status != domain.MemberStatusActive {
		t.Errorf("propB membership = %s, want active after the FIFO recovery", revived.Status)
	}
}
