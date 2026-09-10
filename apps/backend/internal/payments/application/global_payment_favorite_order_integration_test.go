//go:build integration

package application_test

// The favorites manual order's integration tests (ticket #576): the save's
// visibility predicate and position writes, the reading's favoriteOrder
// carriage, the favorite flag's order maintenance and the member scenarios,
// against real PostgreSQL through the shared harness.

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
)

// favoriteOrderOf reads one rule's stored favorite order (nil when unset).
func (h *paymentsHarness) favoriteOrderOf(id uuid.UUID) *int64 {
	h.t.Helper()
	var ord *int64
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT favorite_order FROM payments WHERE id = $1`, id,
	).Scan(&ord); err != nil {
		h.t.Fatalf("read favorite order of %s: %v", id, err)
	}
	return ord
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
	if got := h.favoriteOrderOf(c); got == nil || *got != 1 {
		t.Errorf("position of c = %v, want 1", got)
	}
	if got := h.favoriteOrderOf(a); got == nil || *got != 2 {
		t.Errorf("position of a = %v, want 2", got)
	}
	if got := h.favoriteOrderOf(b); got == nil || *got != 3 {
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
// save writes nothing; a visible non-favorite and a duplicate list are
// invalid input.
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
	if got := h.favoriteOrderOf(mine); got != nil {
		t.Errorf("position of mine = %v, want nil (nothing written)", got)
	}

	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{mine, mine}); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("duplicate err = %v, want ErrInvalidInput", err)
	}

	// A visible rule that is not favorited cannot take a position.
	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, mine, false); err != nil {
		t.Fatalf("unset favorite: %v", err)
	}
	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, []uuid.UUID{mine}); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("non-favorite err = %v, want ErrInvalidInput", err)
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
	if got := h.favoriteOrderOf(fresh); got != nil {
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
	// rule's old spot.
	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, saved, false); err != nil {
		t.Fatalf("unset favorite: %v", err)
	}
	if got := h.favoriteOrderOf(saved); got != nil {
		t.Errorf("unfavorited position = %v, want nil (cleared with the flag)", got)
	}
	if _, err := h.svc.SetPaymentFavorite(h.ctx(), h.owner, h.propID, saved, true); err != nil {
		t.Fatalf("re-favorite: %v", err)
	}
	if got := h.favoriteOrderOf(saved); got != nil {
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

// Full replacement (ticket #576): a favorite left out of the save loses
// its position and falls to the end; an empty save resets the whole order.
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
	if got := h.favoriteOrderOf(a); got == nil || *got != 1 {
		t.Errorf("position of a = %v, want 1", got)
	}
	for _, id := range []uuid.UUID{b, c} {
		if got := h.favoriteOrderOf(id); got != nil {
			t.Errorf("position of omitted favorite = %v, want nil (falls to the end)", got)
		}
	}

	// An empty save resets everything.
	if err := svc.SaveFavoriteOrder(h.ctx(), h.owner, nil); err != nil {
		t.Fatalf("empty save: %v", err)
	}
	if got := h.favoriteOrderOf(a); got != nil {
		t.Errorf("position after reset = %v, want nil", got)
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
	if got := h.favoriteOrderOf(active); got != nil {
		t.Errorf("position of active = %v, want nil (rolled back)", got)
	}
}

// Members: an active full-access member saves the order of the shared
// property's favorites; a suspended membership is the privacy 404.
func TestSaveFavoriteOrder_MemberScopes(t *testing.T) {
	t.Parallel()

	h := newPaymentsHarnessWithRealPolicy(t).withOwner("Europe/Moscow")
	shared := h.seedGlobalRule(h.propID, h.owner, "Интернет", "expense", 70000, false, true, "internet", nil)

	member := h.seedGlobalUser("Europe/Moscow")
	h.seedMembership(h.propID, member, h.owner, "full_access", "active")
	stranger := h.seedGlobalUser("Europe/Moscow")
	h.seedMembership(h.propID, stranger, h.owner, "full_access", "suspended")
	viewer := h.seedGlobalUser("Europe/Moscow")
	h.seedMembership(h.propID, viewer, h.owner, "viewer", "active")
	svc := h.globalSvc()

	if err := svc.SaveFavoriteOrder(h.ctx(), member, []uuid.UUID{shared}); err != nil {
		t.Fatalf("member SaveFavoriteOrder: %v", err)
	}
	if got := h.favoriteOrderOf(shared); got == nil || *got != 1 {
		t.Errorf("position of shared = %v, want 1", got)
	}

	if err := svc.SaveFavoriteOrder(h.ctx(), stranger, []uuid.UUID{shared}); !errors.Is(err, paymentsapp.ErrNotFound) {
		t.Fatalf("suspended err = %v, want ErrNotFound", err)
	}

	// The write gate is the favorite star's (#461): a viewer reads the
	// favorites but does not reorder them.
	if err := svc.SaveFavoriteOrder(h.ctx(), viewer, []uuid.UUID{shared}); !errors.Is(err, paymentsapp.ErrForbidden) {
		t.Fatalf("viewer err = %v, want ErrForbidden", err)
	}
	if got := h.favoriteOrderOf(shared); got == nil || *got != 1 {
		t.Errorf("position of shared after viewer attempt = %v, want 1 (untouched)", got)
	}
}
