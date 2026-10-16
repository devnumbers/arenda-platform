package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/realtime/realtimetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The realtime seam's tests of the access context (карта #714, #716; ADR
// 0062): every committed membership or invitation transition dispatches its
// access pair — the history pair piggybacking when the transition journaled a
// row — strictly post-commit.

// TestAddMemberPublishesAccessAndHistoryFrames pins the grant's frames: the
// active landing journals member.added — the history pair piggybacks.
func TestAddMemberPublishesAccessAndHistoryFrames(t *testing.T) {
	t.Parallel()

	f := newMutationFixture()
	prop := uuid.Must(uuid.NewV7())
	f.addProperty(prop, f.owner, testFirstTitle)
	member := uuid.Must(uuid.NewV7())
	f.limiter.set(member, 1) // A free recipient slot: the landing lands active.

	_, err := f.access.AddMember(context.Background(), f.owner, prop, member, domain.RoleViewer)
	require.NoError(t, err)

	require.NotEmpty(t, f.realtime.Publications)
	assert.Equal(t,
		[]string{"access:" + prop.String(), "history:" + prop.String()},
		f.realtime.Pairs())
}

// TestChangeMemberRolePublishesFrames pins the role change's frames.
func TestChangeMemberRolePublishesFrames(t *testing.T) {
	t.Parallel()

	f := newMutationFixture()
	prop := uuid.Must(uuid.NewV7())
	f.addProperty(prop, f.owner, testFirstTitle)
	member := uuid.Must(uuid.NewV7())
	f.limiter.set(member, 1)
	_, err := f.access.AddMember(context.Background(), f.owner, prop, member, domain.RoleViewer)
	require.NoError(t, err)
	f.realtime.Publications = nil

	members, err := f.access.ListMembers(context.Background(), f.owner, prop)
	require.NoError(t, err)
	membershipID := membershipOf(t, members, member)
	require.NotEqual(t, uuid.Nil, membershipID)

	_, err = f.access.ChangeMemberRole(context.Background(), f.owner, prop, membershipID, domain.RoleFullAccess)
	require.NoError(t, err)

	assert.Equal(t,
		[]string{"access:" + prop.String(), "history:" + prop.String()},
		f.realtime.Pairs())
}

// TestSameRoleChangeMemberRolePublishesNothing extends the same-role no-op
// canon (ADR 0061 §3) to the realtime stream: a role re-set that changes
// nothing journals no row and dispatches no frame — no change, no access-view
// delta for the subscribers to re-read.
func TestSameRoleChangeMemberRolePublishesNothing(t *testing.T) {
	t.Parallel()

	f := newMutationFixture()
	prop := uuid.Must(uuid.NewV7())
	f.addProperty(prop, f.owner, testFirstTitle)
	member := uuid.Must(uuid.NewV7())
	f.limiter.set(member, 1)
	_, err := f.access.AddMember(t.Context(), f.owner, prop, member, domain.RoleViewer)
	require.NoError(t, err)
	f.realtime.Publications = nil

	members, err := f.access.ListMembers(t.Context(), f.owner, prop)
	require.NoError(t, err)
	membershipID := membershipOf(t, members, member)
	require.NotEqual(t, uuid.Nil, membershipID)

	_, err = f.access.ChangeMemberRole(t.Context(), f.owner, prop, membershipID, domain.RoleViewer)
	require.NoError(t, err)
	assert.Empty(t, f.realtime.Publications,
		"the same-role no-op dispatches no realtime frame")
}

// TestRevokeAccessPublishesFrames pins the revocation's frames; the revoked
// member's audience resolves out at the next publication — the frames just
// stop.
func TestRevokeAccessPublishesFrames(t *testing.T) {
	t.Parallel()

	f := newMutationFixture()
	prop := uuid.Must(uuid.NewV7())
	f.addProperty(prop, f.owner, testFirstTitle)
	member := uuid.Must(uuid.NewV7())
	f.limiter.set(member, 1)
	_, err := f.access.AddMember(context.Background(), f.owner, prop, member, domain.RoleViewer)
	require.NoError(t, err)
	f.realtime.Publications = nil

	members, err := f.access.ListMembers(context.Background(), f.owner, prop)
	require.NoError(t, err)
	membershipID := membershipOf(t, members, member)
	require.NotEqual(t, uuid.Nil, membershipID)

	require.NoError(t, f.access.RevokeMember(context.Background(), f.owner, prop, membershipID))

	assert.Equal(t,
		[]string{"access:" + prop.String(), "history:" + prop.String()},
		f.realtime.Pairs())
}

// TestSuspendedRevokePublishesNothing pins the suspended-leg canon: the
// suspended membership's revoke publishes no frame — the object was already
// hidden from its holder (issue #162, T6 canon), the access view did not
// change and the journal records no row for it either.
func TestSuspendedRevokePublishesNothing(t *testing.T) {
	t.Parallel()

	f := newMutationFixture()
	prop := uuid.Must(uuid.NewV7())
	f.addProperty(prop, f.owner, testFirstTitle)
	member := uuid.Must(uuid.NewV7())
	// No free slot: the landing lands suspended.
	_, err := f.access.AddMember(context.Background(), f.owner, prop, member, domain.RoleViewer)
	require.NoError(t, err)
	require.NotEmpty(t, f.realtime.Publications, "the suspended landing still dirties the participants view")
	f.realtime.Publications = nil

	members, err := f.access.ListMembers(context.Background(), f.owner, prop)
	require.NoError(t, err)
	membershipID := membershipOf(t, members, member)
	require.NotEqual(t, uuid.Nil, membershipID)

	require.NoError(t, f.access.RevokeMember(context.Background(), f.owner, prop, membershipID))
	assert.Empty(t, f.realtime.Publications,
		"the suspended leg's revoke publishes nothing")
}

// TestRollbackPublishesNothing pins the post-commit canon: a rejected
// transition dispatches no frames.
func TestRollbackPublishesNothing(t *testing.T) {
	t.Parallel()

	f := newMutationFixture()
	prop := uuid.Must(uuid.NewV7())
	f.addProperty(prop, f.owner, testFirstTitle)
	member := uuid.Must(uuid.NewV7())

	_, err := f.access.AddMember(context.Background(), f.owner, prop, member, domain.RoleViewer)
	require.NoError(t, err)
	f.realtime.Publications = nil

	_, err = f.access.AddMember(context.Background(), f.owner, prop, member, domain.RoleViewer)
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrMemberAlreadyExists)
	assert.Empty(t, f.realtime.Publications)
}

// TestRemovePendingLegPublishesAccessPair extends the removal canon to the
// unregistered invitee: a person holding only pending legs is removed, and
// every affected object's participants view is dirtied — the pending row
// leaves the list; the journal row of the cancellation piggybacks.
func TestRemovePendingLegPublishesAccessPair(t *testing.T) {
	t.Parallel()

	f := newMutationFixture()
	prop := uuid.Must(uuid.NewV7())
	f.addProperty(prop, f.owner, testFirstTitle)
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: prop, Email: testNewUserEmail,
		Role: domain.RoleViewer, InvitedBy: f.owner, LastSentAt: f.clk.now, CreatedAt: f.clk.now,
	}); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	require.NoError(t, f.svc.Remove(t.Context(), f.owner, testNewUserEmail))

	assert.Equal(t,
		[]string{"access:" + prop.String(), "history:" + prop.String()},
		f.realtime.Pairs())
}

// TestRevokeMemberRecoversFifoPair extends the revocation dispatch with the
// FIFO recovery: freeing the revoked member's slot restores their oldest
// suspended access on another object, and that object's participants view is
// dirtied in the same dispatch (the journal row belongs to the revocation
// only — the coordinator journals no recovery row).
func TestRevokeMemberRecoversFifoPair(t *testing.T) {
	t.Parallel()

	f := newMutationFixture()
	prop := uuid.Must(uuid.NewV7())
	f.addProperty(prop, f.owner, testFirstTitle)
	other := uuid.Must(uuid.NewV7())
	restored := uuid.Must(uuid.NewV7())
	f.addProperty(restored, other, testSecondTitle)
	member := uuid.Must(uuid.NewV7())
	f.limiter.set(member, 1) // One free slot: the landing lands active.
	_, err := f.access.AddMember(context.Background(), f.owner, prop, member, domain.RoleViewer)
	require.NoError(t, err)
	// The member's suspended leg on another object waits in the FIFO queue.
	if _, err := f.repo.CreateWithStatus(t.Context(), domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: restored, UserID: member,
		Role: domain.RoleViewer, GrantedBy: other, Status: domain.MemberStatusSuspended,
	}); err != nil {
		t.Fatalf("seed suspended leg: %v", err)
	}
	f.realtime.Publications = nil

	members, err := f.access.ListMembers(context.Background(), f.owner, prop)
	require.NoError(t, err)
	membershipID := membershipOf(t, members, member)
	require.NotEqual(t, uuid.Nil, membershipID)

	require.NoError(t, f.access.RevokeMember(context.Background(), f.owner, prop, membershipID))

	assert.Equal(t,
		[]string{
			"access:" + prop.String(),
			"access:" + restored.String(),
			"history:" + prop.String(),
		},
		f.realtime.Pairs())
}

// TestLeavePropertyRecoversFifoPair extends the self-exit dispatch with the
// FIFO recovery: the leaver's freed slot restores their oldest suspended
// access on another object — its pair rides the same dispatch, and the
// leaver stays in the audience at publication (ADR 0062 §4).
func TestLeavePropertyRecoversFifoPair(t *testing.T) {
	t.Parallel()

	f := newMutationFixture()
	prop := uuid.Must(uuid.NewV7())
	f.addProperty(prop, f.owner, testFirstTitle)
	other := uuid.Must(uuid.NewV7())
	restored := uuid.Must(uuid.NewV7())
	f.addProperty(restored, other, testSecondTitle)
	member := uuid.Must(uuid.NewV7())
	f.limiter.set(member, 1)
	_, err := f.access.AddMember(context.Background(), f.owner, prop, member, domain.RoleViewer)
	require.NoError(t, err)
	if _, err := f.repo.CreateWithStatus(t.Context(), domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: restored, UserID: member,
		Role: domain.RoleViewer, GrantedBy: other, Status: domain.MemberStatusSuspended,
	}); err != nil {
		t.Fatalf("seed suspended leg: %v", err)
	}
	f.realtime.Publications = nil

	require.NoError(t, f.access.LeaveProperty(context.Background(), member, prop))

	assert.Equal(t,
		[]string{
			"access:" + prop.String(),
			"access:" + restored.String(),
			"history:" + prop.String(),
		},
		f.realtime.Pairs())
}

// flakyOwnerResolver resolves through the embedded static map; failAfter is
// an absolute call number — calls beyond it fail with ErrMemberNotFound. The
// policy shares the resolver (its role checks must keep resolving), so the
// trip is armed between mutations: one success is left for the
// pre-transaction role check, the next call — the post-commit owner lookup —
// fails.
type flakyOwnerResolver struct {
	staticResolver
	calls     int
	failAfter int
}

func (f *flakyOwnerResolver) GetOwnerID(ctx context.Context, propertyID uuid.UUID) (uuid.UUID, error) {
	f.calls++
	if f.failAfter > 0 && f.calls > f.failAfter {
		return uuid.Nil, domain.ErrMemberNotFound
	}
	return f.staticResolver.GetOwnerID(ctx, propertyID)
}

// TestLeavePropertyPublishesFramesWhenOwnerLookupFails pins the dispatch's
// independence from the owner lookup (карта #714, #716): the lookup serves
// only the owner notification, so its post-commit failure skips the email
// event — never the realtime pairs, which a swallowed dispatch would silence
// on the leaver's and the restored legs' screens until the next stream open.
func TestLeavePropertyPublishesFramesWhenOwnerLookupFails(t *testing.T) {
	t.Parallel()

	owners := &flakyOwnerResolver{staticResolver: staticResolver{}}
	repo := newMemRepo()
	statuses := fakeStatuses{}
	policy := NewMembershipPolicy(owners, repo)
	events := &fakeEventPublisher{}
	clk := &fixedClock{now: time.Now()}
	limiter := newFakeRecipientLimiter()
	audit := &capturingRecorder{}
	coordinator := NewSlotCoordinator(repo, owners, limiter, newFakeOwnedProps(),
		events, audit, noopBeginner{}, clk, nil)
	realtime := &realtimetest.RecordingPublisher{}
	access := NewAccessService(repo, owners, statuses, newFakeLookup(), policy,
		coordinator, events,
		newTestFactory(repo, &memInvitationsRepo{removalScope: repo.inManageScope}, audit), nil)
	access.SetRealtimePublisher(realtime)

	owner := uuid.Must(uuid.NewV7())
	prop := uuid.Must(uuid.NewV7())
	owners.staticResolver[prop] = owner
	repo.SetOwner(prop, owner)
	member := uuid.Must(uuid.NewV7())
	limiter.set(member, 1)
	_, err := access.AddMember(context.Background(), owner, prop, member, domain.RoleViewer)
	require.NoError(t, err)
	realtime.Publications = nil
	events.events = nil
	// The next call is LeaveProperty's role check — it must still resolve;
	// the one after it is the post-commit owner lookup — it fails.
	owners.failAfter = owners.calls + 1

	require.NoError(t, access.LeaveProperty(context.Background(), member, prop))

	assert.Equal(t,
		[]string{"access:" + prop.String(), "history:" + prop.String()},
		realtime.Pairs())
	assert.Empty(t, events.events, "the failed owner lookup skips only the notification")
}

// TestInviteByEmailPendingPublishesFrames pins the pending invitation's
// frames: the participants view gains the pending row, the journal row
// piggybacks.
func TestInviteByEmailPendingPublishesFrames(t *testing.T) {
	t.Parallel()

	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	prop := f.addProperty(owner)

	outcome, err := f.svc.InviteByEmail(context.Background(), owner, prop, "newperson@example.ru", domain.RoleViewer)
	require.NoError(t, err)
	require.NotNil(t, outcome.Invitation)

	assert.Equal(t,
		[]string{"access:" + prop.String(), "history:" + prop.String()},
		f.realtime.Pairs())
}

// TestSameRoleChangeInvitationRolePublishesNothing extends the same-role
// no-op canon to the pending invitation: re-setting the invitation's role to
// the current one journals no row (ADR 0061 §3) and dispatches no frame.
func TestSameRoleChangeInvitationRolePublishesNothing(t *testing.T) {
	t.Parallel()

	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	prop := f.addProperty(owner)

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, prop, testNewUserEmail, domain.RoleViewer)
	require.NoError(t, err)
	require.NotNil(t, outcome.Invitation)
	f.realtime.Publications = nil

	_, err = f.svc.ChangeInvitationRole(t.Context(), owner, prop, outcome.Invitation.ID, domain.RoleViewer)
	require.NoError(t, err)
	assert.Empty(t, f.realtime.Publications,
		"the same-role invitation no-op dispatches no realtime frame")
}

// membershipOf finds the member's membership row in the participants list.
func membershipOf(t *testing.T, members []Member, userID uuid.UUID) uuid.UUID {
	t.Helper()
	for _, m := range members {
		if !m.IsOwner && m.UserID == userID {
			return m.ID
		}
	}
	return uuid.Nil
}
