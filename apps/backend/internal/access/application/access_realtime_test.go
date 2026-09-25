package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
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
