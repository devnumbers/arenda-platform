//go:build integration

// The global payments feed's real-SQL visibility predicate (ticket #575,
// the actor-scoped cross-property read): «(владелец OR активный участник)
// неархивного объекта» — the archive cut applies to BOTH legs, the owner
// included (карта #573: «правила архивных объектов — вне каждого чтения»),
// which is the stricter relative of the history canon's
// actor_can_read_property (000142, ADR 0028): the history owner reads the
// archive, the global payments feed's owner does not. The matrix runs the
// same seeded world through the reads and the favorites order save's lock
// and clear passes, so a predicate drift (a bare function call without the
// feed's own archive cut, a dropped member leg) fails here, not silently.

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	paymentspg "github.com/nambers/arenda-planform/apps/backend/internal/payments/adapters/postgres"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The seeded world's statuses — the schema CHECK's vocabulary, spelled out
// once: the property status drives the feed's archive cut, the membership
// status the member leg.
const (
	propStatusActive      = "active"
	propStatusArchived    = "archived"
	memberStatusActive    = "active"
	memberStatusSuspended = "suspended"
)

// globalVisibilityHarness seeds one owner, three membership states and four
// properties: visible-owner (A), active-member (B), suspended-member (C) and
// active-member-but-archived (D) — every leg of the visibility matrix.
type globalVisibilityHarness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	store  *paymentspg.GlobalPaymentStore
	owner  uuid.UUID
	member uuid.UUID
	propA  uuid.UUID // The owner's own, active property.
	propB  uuid.UUID // The member's active membership, active property.
	propC  uuid.UUID // The member's suspended membership, active property.
	propD  uuid.UUID // The member's active membership, archived property.
	ruleA  uuid.UUID
	ruleB  uuid.UUID
	ruleD  uuid.UUID
}

func newGlobalVisibilityHarness(t *testing.T) *globalVisibilityHarness {
	t.Helper()

	pool := testdb.Setup(t)
	ctx := context.Background()

	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	for _, u := range []uuid.UUID{owner, member} {
		phone := fmt.Sprintf("+7999%010d", time.Now().UnixNano()%10000000000)
		_, err := pool.Exec(ctx,
			`INSERT INTO users (id, phone, role, timezone) VALUES ($1, $2, $3, 'Europe/Moscow')`,
			u, phone, actor.RoleOwner)
		require.NoError(t, err)
	}

	propA := uuid.Must(uuid.NewV7())
	propB := uuid.Must(uuid.NewV7())
	propC := uuid.Must(uuid.NewV7())
	propD := uuid.Must(uuid.NewV7())
	for _, prop := range []struct {
		id     uuid.UUID
		status string
	}{
		{propA, propStatusActive},
		{propB, propStatusActive},
		{propC, propStatusActive},
		{propD, propStatusArchived},
	} {
		_, err := pool.Exec(ctx,
			`INSERT INTO properties (id, owner_id, name, type, address, status)
			 VALUES ($1, $2, 'Квартира', 'apartment', 'Москва, Тверская 1', $3)`,
			prop.id, owner, prop.status)
		require.NoError(t, err)
	}

	for _, m := range []struct {
		prop   uuid.UUID
		status string
	}{{propB, memberStatusActive}, {propC, memberStatusSuspended}, {propD, memberStatusActive}} {
		_, err := pool.Exec(ctx,
			`INSERT INTO property_members (id, property_id, user_id, role, granted_by, status)
			 VALUES ($1, $2, $3, 'viewer', $4, $5)`,
			uuid.Must(uuid.NewV7()), m.prop, member, owner, m.status)
		require.NoError(t, err)
	}

	seedRule := func(t *testing.T, prop uuid.UUID, favorite bool, order int64) uuid.UUID {
		t.Helper()
		id := uuid.Must(uuid.NewV7())
		_, err := pool.Exec(ctx,
			`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
			                       recurrence, since, auto_pay, payment_form, category_slug,
			                       is_favorite, favorite_order)
			 VALUES ($1, $2, $3, 'expense', 'ЖКУ', 500000, '{"kind":"daily"}'::jsonb, $4,
			         false, 'transfer', 'utilities', $5, NULLIF($6, 0))`,
			id, owner, prop, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), favorite, order)
		require.NoError(t, err)
		return id
	}
	ruleA := seedRule(t, propA, true, 1)
	ruleB := seedRule(t, propB, true, 2)
	ruleD := seedRule(t, propD, true, 3)

	return &globalVisibilityHarness{
		t: t, pool: pool, store: paymentspg.NewGlobalPaymentStore(pool),
		owner: owner, member: member,
		propA: propA, propB: propB, propC: propC, propD: propD,
		ruleA: ruleA, ruleB: ruleB, ruleD: ruleD,
	}
}

// todays feeds the owner→today map the reads join on — one owner in this
// world, so one entry covers every visible row.
func (h *globalVisibilityHarness) todays() map[uuid.UUID]time.Time {
	return map[uuid.UUID]time.Time{
		h.owner: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
	}
}

func TestGlobalPaymentStore_VisibilityPredicateMatrix(t *testing.T) {
	t.Parallel()
	h := newGlobalVisibilityHarness(t)
	ctx := context.Background()

	t.Run("owner todays carry only visible non-archived properties", func(t *testing.T) {
		t.Parallel()
		memberOwners, err := h.store.ListGlobalPaymentOwnerTodays(ctx, h.member)
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{h.owner}, memberOwners,
			"the active membership's owner rides, the archived property's does not")

		ownerOwners, err := h.store.ListGlobalPaymentOwnerTodays(ctx, h.owner)
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{h.owner}, ownerOwners,
			"the owner's own archived property drops out of the owner's map too — the cut binds both legs")
	})

	t.Run("the merged feed lists visible rules only", func(t *testing.T) {
		t.Parallel()
		memberRows, err := h.store.ListGlobalPaymentRules(ctx, h.member, h.todays(), paymentsapp.GlobalPaymentRulesQuery{})
		require.NoError(t, err)
		require.Len(t, memberRows, 1)
		assert.Equal(t, h.ruleB, memberRows[0].ID,
			"the member reads the active shared property, not the archived one, not the owner's own")

		ownerRows, err := h.store.ListGlobalPaymentRules(ctx, h.owner, h.todays(), paymentsapp.GlobalPaymentRulesQuery{})
		require.NoError(t, err)
		gotIDs := make([]uuid.UUID, 0, len(ownerRows))
		for _, row := range ownerRows {
			gotIDs = append(gotIDs, row.ID)
		}
		assert.ElementsMatch(t, []uuid.UUID{h.ruleA, h.ruleB}, gotIDs,
			"the owner reads own and shared active rules — the archived property's rule stays out for the owner too")
	})

	t.Run("the objects listing follows the same predicate", func(t *testing.T) {
		t.Parallel()
		memberObjects, err := h.store.ListGlobalPaymentObjects(ctx, h.member, "")
		require.NoError(t, err)
		require.Len(t, memberObjects, 1)
		assert.Equal(t, h.propB, memberObjects[0].PropertyID)

		ownerObjects, err := h.store.ListGlobalPaymentObjects(ctx, h.owner, "")
		require.NoError(t, err)
		gotProps := make([]uuid.UUID, 0, len(ownerObjects))
		for _, obj := range ownerObjects {
			gotProps = append(gotProps, obj.PropertyID)
		}
		assert.ElementsMatch(t, []uuid.UUID{h.propA, h.propB, h.propC}, gotProps,
			"the archived object is invisible to its owner in the global surface (карта #573)")
	})

	t.Run("the favorites lock pass returns visible rows with the per-row role", func(t *testing.T) {
		t.Parallel()
		memberLocks, err := h.store.LockVisibleFavorites(ctx, h.member, []uuid.UUID{h.ruleB, h.ruleD})
		require.NoError(t, err)
		require.Len(t, memberLocks, 1)
		assert.Equal(t, h.ruleB, memberLocks[0].ID)
		assert.Equal(t, sharedpolicy.RoleViewer, memberLocks[0].Role,
			"the active membership's role travels beside the data")

		ownerLocks, err := h.store.LockVisibleFavorites(ctx, h.owner, []uuid.UUID{h.ruleA, h.ruleD})
		require.NoError(t, err)
		require.Len(t, ownerLocks, 1)
		assert.Equal(t, h.ruleA, ownerLocks[0].ID)
		assert.Equal(t, sharedpolicy.RoleOwner, ownerLocks[0].Role)
	})

	t.Run("the favorites clear pass touches visible favorites only", func(t *testing.T) {
		t.Parallel()
		err := h.store.ClearFavoriteOrdersOutside(ctx, h.member, nil)
		require.NoError(t, err)

		var bOrder, dOrder *int64
		require.NoError(t, h.pool.QueryRow(ctx,
			`SELECT favorite_order FROM payments WHERE id = $1`, h.ruleB).Scan(&bOrder))
		require.NoError(t, h.pool.QueryRow(ctx,
			`SELECT favorite_order FROM payments WHERE id = $1`, h.ruleD).Scan(&dOrder))
		assert.Nil(t, bOrder, "the visible favorite loses its position")
		assert.NotNil(t, dOrder, "the archived property's favorite is invisible — its position survives")
	})
}
