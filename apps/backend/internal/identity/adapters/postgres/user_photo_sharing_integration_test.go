//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedSharingUser creates a user by phone, returning the id.
func seedSharingUser(t *testing.T, ctx context.Context, repo *UserRepository, phone string) uuid.UUID {
	t.Helper()
	return seedUser(t, ctx, repo, phone, phone+"@example.com").ID
}

// seedSharingProperty inserts an owned property row.
func seedSharingProperty(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx,
		`INSERT INTO properties (id, owner_id, name, type, address, status)
		 VALUES ($1, $2, 'Квартира', 'apartment', 'Москва, Тверская 1', $3)`,
		id, owner, status)
	require.NoError(t, err)
	return id
}

// seedMembership inserts a membership row with the given status.
func seedMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, propertyID, userID uuid.UUID, status string) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO property_members (id, property_id, user_id, role, granted_by, status)
		 VALUES ($1, $2, $3, 'viewer', $3, $4)`,
		uuid.Must(uuid.NewV7()), propertyID, userID, status)
	require.NoError(t, err)
}

// TestUserRepository_ShareReadableProperty_Matrix pins the foreign
// profile-photo gate (ADR 0065, решение владельца #1286) against every
// relation shape, each on its own property pair: symmetric owner↔member and
// co-members read, suspended and unrelated do not, and the archive follows
// the canonical function — the owner reads their archived property's
// members, a member gets nothing from an archived property.
func TestUserRepository_ShareReadableProperty_Matrix(t *testing.T) {
	t.Parallel()
	pool := testdb.Setup(t)
	ctx := context.Background()
	repo := NewUserRepository(pool, noopEncryptor(t))

	const (
		active    = "active"
		suspended = "suspended"
		live      = "active"
		archived  = "archived"
	)

	// Owner ↔ member on a live property: both directions read.
	owner := seedSharingUser(t, ctx, repo, "+79990001201")
	member := seedSharingUser(t, ctx, repo, "+79990001202")
	liveProperty := seedSharingProperty(t, ctx, pool, owner, live)
	seedMembership(t, ctx, pool, liveProperty, member, active)

	// Co-members of one live property read each other.
	coOwner := seedSharingUser(t, ctx, repo, "+79990001203")
	coMemberA := seedSharingUser(t, ctx, repo, "+79990001204")
	coMemberB := seedSharingUser(t, ctx, repo, "+79990001205")
	coProperty := seedSharingProperty(t, ctx, pool, coOwner, live)
	seedMembership(t, ctx, pool, coProperty, coMemberA, active)
	seedMembership(t, ctx, pool, coProperty, coMemberB, active)

	// A member gets nothing from an archived property; the owner of the
	// archived property still reads their member (the archive canon).
	archivedOwner := seedSharingUser(t, ctx, repo, "+79990001206")
	archivedMember := seedSharingUser(t, ctx, repo, "+79990001207")
	archivedProperty := seedSharingProperty(t, ctx, pool, archivedOwner, archived)
	seedMembership(t, ctx, pool, archivedProperty, archivedMember, active)

	// A suspended leg shares nothing.
	suspendedOwner := seedSharingUser(t, ctx, repo, "+79990001208")
	suspendedMember := seedSharingUser(t, ctx, repo, "+79990001209")
	suspendedProperty := seedSharingProperty(t, ctx, pool, suspendedOwner, live)
	seedMembership(t, ctx, pool, suspendedProperty, suspendedMember, suspended)

	// A fully unrelated pair.
	unrelatedA := seedSharingUser(t, ctx, repo, "+79990001210")
	unrelatedB := seedSharingUser(t, ctx, repo, "+79990001211")

	cases := []struct {
		name   string
		viewer uuid.UUID
		target uuid.UUID
		want   bool
	}{
		{"member sees owner", member, owner, true},
		{"owner sees member (symmetric)", owner, member, true},
		{"co-members see each other", coMemberA, coMemberB, true},
		{"member of archived property", archivedMember, archivedOwner, false},
		{"owner reads archived property's member", archivedOwner, archivedMember, true},
		{"suspended member", suspendedMember, suspendedOwner, false},
		{"unrelated users", unrelatedA, unrelatedB, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := repo.ShareReadableProperty(ctx, tc.viewer, tc.target)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got, "%s -> %s", tc.viewer, tc.target)
		})
	}
}
