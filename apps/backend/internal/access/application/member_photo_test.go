package application

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	accessdomain "github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/stretchr/testify/require"
)

// TestAccessService_ListMembersCarriesPhotoUrl proves every row of the
// object's members list — the synthesized owner and the membership rows —
// carries the profile photo's streaming path (ADR 0065, решение #1286): the
// reader of the list shares the property with every listed participant, and
// a member without a photo key stays empty.
func TestAccessService_ListMembersCarriesPhotoUrl(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	prop := uuid.Must(uuid.NewV7())
	f.addProperty(prop, f.owner, testFirstTitle)

	member := uuid.Must(uuid.NewV7())
	f.lookup.byID[member] = MemberUser{ID: member, PhotoKey: new("photos/m.jpg")}
	f.lookup.byID[f.owner] = MemberUser{ID: f.owner, PhotoKey: new("photos/o.jpg")}
	f.limiter.set(member, 1)
	_, err := f.access.AddMember(context.Background(), f.owner, prop, member, accessdomain.RoleViewer)
	require.NoError(t, err)

	members, err := f.access.ListMembers(context.Background(), f.owner, prop)
	require.NoError(t, err)
	require.Len(t, members, 2)

	ownerRow, memberRow := members[0], members[1]
	require.True(t, ownerRow.IsOwner)
	require.True(t, strings.HasSuffix(ownerRow.PhotoURL, "/api/v1/users/"+f.owner.String()+"/photo"),
		"owner photoURL = %q", ownerRow.PhotoURL)
	require.True(t, strings.HasSuffix(memberRow.PhotoURL, "/api/v1/users/"+member.String()+"/photo"),
		"member photoURL = %q", memberRow.PhotoURL)
}

// TestAccessService_ListMembersWithoutPhotoStaysEmpty proves a photoless
// user keeps the empty path — the nullable contract's null.
func TestAccessService_ListMembersWithoutPhotoStaysEmpty(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	prop := uuid.Must(uuid.NewV7())
	f.addProperty(prop, f.owner, testFirstTitle)

	members, err := f.access.ListMembers(context.Background(), f.owner, prop)
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Empty(t, members[0].PhotoURL)
}
