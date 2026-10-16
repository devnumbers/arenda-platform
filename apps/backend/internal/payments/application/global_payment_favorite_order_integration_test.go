//go:build integration

package application_test

// The favorites manual order's integration tests (ticket #576; the personal
// cutover #1308, D12): the save's personal-set lock and position writes, the
// reading's favoriteOrder carriage, the star's order maintenance, the
// per-actor isolation and the member scenarios, against real PostgreSQL
// through the shared harness.

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
)

// favoriteRowOf reads the given actor's personal favorite row for the rule:
// whether the star exists at all and its manual order position (nil when
// the row carries no position — the end of the order).
func (h *paymentsHarness) favoriteRowOf(userID, id uuid.UUID) (starred bool, position *int64) {
	h.t.Helper()
	var ord *int64
	err := h.pool.QueryRow(h.ctx(),
		`SELECT favorite_order FROM user_payment_favorites WHERE user_id = $1 AND payment_id = $2`,
		userID, id,
	).Scan(&ord)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		h.t.Fatalf("read favorite row of user %s for payment %s: %v", userID, id, err)
	}
	return true, ord
}

// favoriteOrderOf reads one rule's position in the given actor's personal
// favorite order (nil when the actor has not starred the rule or the row
// carries no position).
func (h *paymentsHarness) favoriteOrderOf(userID, id uuid.UUID) *int64 {
	starred, position := h.favoriteRowOf(userID, id)
	if !starred {
		return nil
	}
	return position
}

// favoriteOrderByID returns the feed row's FavoriteOrder for the rule.
func favoriteOrderByID(items []paymentsapp.GlobalPaymentItem, id uuid.UUID) *int64 {
	for _, item := range items {
		if item.ID == id {
			return item.FavoriteOrder
		}
	}
	return nil
}

// The happy path (ticket #576): the submitted list order lands as dense
// positions in one save, the feed rows carry the order out, and one audit
// entry describes the mass write.
func TestSaveFavoriteOrder_RoundTrip(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	a := h.seedGlobalRule(h.propID, h.owner, "Аренда", "income", 6000000, false, true, "rent", nil)
	b := h.seedGlobalRule(h.propID, h.owner, "Интернет", "expense", 70000, false, true, "internet", nil)
	c := h.seedGlobalRule(h.propID, h.owner, "Страхование", "expense", 3200000, false, true, "insurance", nil)
	svc := h.globalSvc()

	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{c, a, b}); err != nil {
		t.Fatalf("SaveFavoriteOrder: %v", err)
	}
	if got := h.favoriteOrderOf(h.owner, c); got == nil || *got != 1 {
		t.Errorf("position of c = %v, want 1", got)
	}
	if got := h.favoriteOrderOf(h.owner, a); got == nil || *got != 2 {
		t.Errorf("position of a = %v, want 2", got)
	}
	if got := h.favoriteOrderOf(h.owner, b); got == nil || *got != 3 {
		t.Errorf("position of b = %v, want 3", got)
	}

	feed, err := svc.ListGlobalPayments(h.ctx(), h.owner)
	if err != nil {
		t.Fatalf("ListGlobalPayments: %v", err)
	}
	if got := favoriteOrderByID(feed.Items, c); got == nil || *got != 1 {
		t.Errorf("feed favoriteOrder of c = %v, want 1", got)
	}
	if got := favoriteOrderByID(feed.Items, b); got == nil || *got != 3 {
		t.Errorf("feed favoriteOrder of b = %v, want 3", got)
	}

	var auditCount int
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT count(*) FROM audit_log WHERE actor_id = $1 AND action = 'payment.updated'
		 AND context->>'count' = '3'`, h.owner,
	).Scan(&auditCount); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("audit entries = %d, want 1", auditCount)
	}
}

// A foreign id — a stranger's rule — is the privacy-preserving 404 and the
// save writes nothing; a duplicate list is invalid input; an id the actor
// has not starred — visible but unstarred, the old «visible non-favorite»
// 400 — folds into the same privacy 404 (the personal rows answer the
// degenerate ids, the cutover #1308).
func TestSaveFavoriteOrder_Rejections(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	mine := h.seedGlobalRule(h.propID, h.owner, "Интернет", "expense", 70000, false, true, "internet", nil)
	svc := h.globalSvc()

	stranger := h.seedGlobalUser("Europe/Moscow")
	strangerProp := h.seedGlobalProperty(stranger, "Чужая", "active")
	foreign := h.seedGlobalRule(strangerProp, stranger, "Чужой платёж", "expense", 10000, false, true, "insurance", nil)

	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{mine, foreign}); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("foreign err = %v, want ErrNotFound", err)
	}
	if got := h.favoriteOrderOf(h.owner, mine); got != nil {
		t.Errorf("position of mine = %v, want nil (nothing written)", got)
	}

	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{mine, mine}); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("duplicate err = %v, want ErrInvalidInput", err)
	}

	// A visible rule the actor has not starred cannot take a position —
	// the lock answers nothing for it, the save is the privacy 404.
	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, mine, false); err != nil {
		t.Fatalf("unset favorite: %v", err)
	}
	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{mine}); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("unstarred err = %v, want ErrNotFound", err)
	}
}

// The reading rule «новое избранное — в конец» (ticket #576): a saved rule
// carries its position, a favorited-after rule carries nil and sorts last on
// the client; re-favoriting keeps it at the end.
func TestFavoriteOrder_NewFavoriteSortsLast(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()
	saved := h.seedGlobalRule(h.propID, h.owner, "Интернет", "expense", 70000, false, true, "internet", nil)

	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{saved}); err != nil {
		t.Fatalf("SaveFavoriteOrder: %v", err)
	}

	// Favorite a second rule after the save: star on, no position.
	fresh := h.seedGlobalRule(h.propID, h.owner, "Страхование", "expense", 3200000, false, false, "insurance", nil)
	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, fresh, true); err != nil {
		t.Fatalf("set favorite: %v", err)
	}
	if got := h.favoriteOrderOf(h.owner, fresh); got != nil {
		t.Errorf("new favorite's position = %v, want nil", got)
	}

	feed, err := svc.ListGlobalPayments(h.ctx(), h.owner)
	if err != nil {
		t.Fatalf("ListGlobalPayments: %v", err)
	}
	if got := favoriteOrderByID(feed.Items, saved); got == nil || *got != 1 {
		t.Errorf("saved favoriteOrder = %v, want 1", got)
	}
	if got := favoriteOrderByID(feed.Items, fresh); got != nil {
		t.Errorf("new favoriteOrder = %v, want nil (sorts last)", got)
	}

	// Re-favoriting after an unfavorite lands at the end again, not at the
	// rule's old spot: the star's row leaves with the unstar (the position
	// with it), the re-star inserts a fresh NULL row.
	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, saved, false); err != nil {
		t.Fatalf("unset favorite: %v", err)
	}
	if got := h.favoriteOrderOf(h.owner, saved); got != nil {
		t.Errorf("unfavorited position = %v, want nil (the row left)", got)
	}
	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, saved, true); err != nil {
		t.Fatalf("re-favorite: %v", err)
	}
	if got := h.favoriteOrderOf(h.owner, saved); got != nil {
		t.Errorf("re-favorited position = %v, want nil (sorts last again)", got)
	}
	feed, err = svc.ListGlobalPayments(h.ctx(), h.owner)
	if err != nil {
		t.Fatalf("ListGlobalPayments after re-favorite: %v", err)
	}
	if got := favoriteOrderByID(feed.Items, saved); got != nil {
		t.Errorf("re-favorited feed favoriteOrder = %v, want nil", got)
	}
}

// A re-star of an already-starred rule is idempotent: the row stays, the
// position never shifts (the PUT's idempotency, the pin's canon).
func TestSetFavorite_ReStarKeepsPosition(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	a := h.seedGlobalRule(h.propID, h.owner, "Аренда", "income", 6000000, false, true, "rent", nil)
	b := h.seedGlobalRule(h.propID, h.owner, "Интернет", "expense", 70000, false, true, "internet", nil)
	svc := h.globalSvc()

	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{b, a}); err != nil {
		t.Fatalf("SaveFavoriteOrder: %v", err)
	}
	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, a, true); err != nil {
		t.Fatalf("re-star: %v", err)
	}
	if got := h.favoriteOrderOf(h.owner, a); got == nil || *got != 2 {
		t.Errorf("position of re-starred a = %v, want 2 (kept)", got)
	}
}

// Full replacement acts in the limits of the actor's personal set (ticket
// #576; the cutover #1308): a favorite left out of the save loses its
// position and falls to the end; an empty save resets the whole personal
// order.
func TestSaveFavoriteOrder_FullReplacement(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	a := h.seedGlobalRule(h.propID, h.owner, "Аренда", "income", 6000000, false, true, "rent", nil)
	b := h.seedGlobalRule(h.propID, h.owner, "Интернет", "expense", 70000, false, true, "internet", nil)
	c := h.seedGlobalRule(h.propID, h.owner, "Страхование", "expense", 3200000, false, true, "insurance", nil)
	svc := h.globalSvc()

	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{c, a, b}); err != nil {
		t.Fatalf("SaveFavoriteOrder: %v", err)
	}
	// The next save places only a: b and c lose their positions.
	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{a}); err != nil {
		t.Fatalf("partial save: %v", err)
	}
	if got := h.favoriteOrderOf(h.owner, a); got == nil || *got != 1 {
		t.Errorf("position of a = %v, want 1", got)
	}
	for _, id := range []uuid.UUID{b, c} {
		starred, position := h.favoriteRowOf(h.owner, id)
		if !starred {
			t.Errorf("omitted favorite %s lost its row, want the star kept", id)
		} else if position != nil {
			t.Errorf("position of omitted favorite %s = %d, want nil (falls to the end)", id, *position)
		}
	}

	// An empty save resets everything.
	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, nil); err != nil {
		t.Fatalf("empty save: %v", err)
	}
	for _, id := range []uuid.UUID{a, b, c} {
		if got := h.favoriteOrderOf(h.owner, id); got != nil {
			t.Errorf("position after reset = %v, want nil", got)
		}
	}
}

// Personal states publish no realtime frames (D18, the cutover #1308): the
// star and the order save dispatch nothing — the actor's other tabs re-read
// on their next load.
func TestFavoriteMutations_PublishNoRealtimeFrames(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	a := h.seedGlobalRule(h.propID, h.owner, "Аренда", "income", 6000000, false, true, "rent", nil)
	b := h.seedGlobalRule(h.propID, h.owner, "Интернет", "expense", 70000, false, true, "internet", nil)

	h.realtime.Publications = nil
	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, a, true); err != nil {
		t.Fatalf("star: %v", err)
	}
	if err := h.globalSvc().SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{b, a}); err != nil {
		t.Fatalf("order save: %v", err)
	}
	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, b, false); err != nil {
		t.Fatalf("unstar: %v", err)
	}
	if got := h.realtime.Pairs(); len(got) != 0 {
		t.Errorf("favorite mutations published %v, want no frames (D18)", got)
	}
}

// The save is the feed's scope, not the whole table: a favorite on an
// archived property is invisible to the save (404), the active ones keep
// working.
func TestSaveFavoriteOrder_ArchivedPropertyIsInvisible(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	active := h.seedGlobalRule(h.propID, h.owner, "Интернет", "expense", 70000, false, true, "internet", nil)
	archivedProp := h.seedGlobalProperty(h.owner, "Архив", "archived")
	archived := h.seedGlobalRule(archivedProp, h.owner, "Старый платёж", "expense", 10000, false, true, "insurance", nil)
	svc := h.globalSvc()

	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{active, archived}); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("archived err = %v, want ErrNotFound", err)
	}
	if got := h.favoriteOrderOf(h.owner, active); got != nil {
		t.Errorf("position of active = %v, want nil (rolled back)", got)
	}
}

// starAndOrder stars the listed rules for the actor (the view-capability
// PUT favorite) and saves the actor's personal order in that list order.
func starAndOrder(t *testing.T, h *paymentsHarness, actor, prop uuid.UUID, ordered []uuid.UUID) {
	t.Helper()
	for _, id := range ordered {
		if _, err := h.svc.SetPaymentFavorite(h.ctx(), actor, prop, id, true); err != nil {
			t.Fatalf("star of %s by %s: %v", id, actor, err)
		}
	}
	if err := h.globalSvc().SaveFavoriteOrder(h.ctx(), actor, ordered); err != nil {
		t.Fatalf("order save by %s: %v", actor, err)
	}
}

// wantOrder asserts one actor's stored position of the rule.
func wantOrder(t *testing.T, h *paymentsHarness, actor, id uuid.UUID, want int64) {
	t.Helper()
	if got := h.favoriteOrderOf(actor, id); got == nil || *got != want {
		t.Errorf("position of %s for %s = %v, want %d", id, actor, got, want)
	}
}

// The favorites are personal (D12): each actor's star and order live on
// their own rows — a member's save reshuffles only the member's set and
// never the owner's; the owner's existing favorites and their order survive
// untouched. A suspended membership is the privacy 404; a viewer orders
// their own set on par with everyone (the star's right is the view one).
func TestSaveFavoriteOrder_ActorSetsAreIsolated(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarnessWithRealPolicy(t).withOwner("Europe/Moscow")
	shared := h.seedGlobalRule(h.propID, h.owner, "Интернет", "expense", 70000, false, true, "internet", nil)
	second := h.seedGlobalRule(h.propID, h.owner, "Страхование", "expense", 3200000, false, true, "insurance", nil)
	// The owner's personal order as it stood before any member acted.
	if err := h.globalSvc().SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{second, shared}); err != nil {
		t.Fatalf("owner order save: %v", err)
	}

	member := h.seedGlobalUser("Europe/Moscow")
	h.seedMembership(h.propID, member, h.owner, "full_access", "active")
	viewer := h.seedGlobalUser("Europe/Moscow")
	h.seedMembership(h.propID, viewer, h.owner, "viewer", "active")
	stranger := h.seedGlobalUser("Europe/Moscow")
	h.seedMembership(h.propID, stranger, h.owner, "full_access", "suspended")

	// The member stars the shared rules (CanView) and orders their own
	// two-rule set their own way.
	starAndOrder(t, h, member, h.propID, []uuid.UUID{shared, second})
	wantOrder(t, h, member, shared, 1)

	// The member's reshuffle never moves the owner's rows (D12).
	wantOrder(t, h, h.owner, second, 1)
	wantOrder(t, h, h.owner, shared, 2)

	// The viewer stars and orders their own set too.
	starAndOrder(t, h, viewer, h.propID, []uuid.UUID{shared})
	wantOrder(t, h, viewer, shared, 1)

	// A suspended membership is the privacy 404 — no access, no set.
	if err := h.globalSvc().SaveFavoriteOrder(h.ctx(), stranger, []uuid.UUID{shared}); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("suspended err = %v, want ErrNotFound", err)
	}
}
