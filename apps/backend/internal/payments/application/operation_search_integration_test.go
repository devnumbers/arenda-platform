//go:build integration

package application_test

// The search-scope integration tests of the operations listing (ticket #476):
// the property operations `search` is the whole Figma hint «Введите название
// операции, сумму, категорию» — a case-insensitive substring over the title
// and the operation's category snapshot, plus an amount match when the query
// reads as an amount (digits and separators only): its digits are searched in
// the amount's decimal digits in kopecks, the display amount without
// separators. The summary runs the same predicate — the search screen's
// «Категории» chips are its breakdown — so both tests share one fixture.

import (
	"testing"

	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// Fixture titles and slugs repeated across the fixture and the expectations
// (goconst keeps them as constants).
const (
	searchSlugSecurity  = "security"
	searchTitleDeposit  = "Залог за ключи"
	searchTitleCleaning = "Клининг холла"
	searchTitlePatrol   = "Патруль территории"
	searchTitleGuard    = "Охрана дома"
)

// searchFixture seeds four paid manual facts and one planned row:
//
//	«Залог за ключи»     2 500,50 ₽ (deposit, «Залог») — the only 250050 digits
//	«Клининг холла»     12 500,00 ₽ (cleaning, «Клининг») — the amount digits hold «2500»
//	«Патруль территории» 3 000,00 ₽ (security, «Охрана») — only the label matches «охра»
//	«Охрана дома»        2 500,00 ₽ (security, «Охрана») — title and label match «охра»
//	«Охрана» (planned)   2 500,00 ₽ (security, «Охрана») — never in the paid scope
//
// The paid rows are listed newest-first (the contract's desc default), so the
// expectations below follow that order.
func (h *paymentsHarness) searchFixture() {
	h.t.Helper()
	h.seedOperationRow(nil, "2026-08-13", opPaid, "income", searchTitleDeposit, 250050, "deposit", "Залог")
	h.seedOperationRow(nil, "2026-08-12", opPaid, "expense", searchTitleCleaning, 1250000, "cleaning", "Клининг")
	h.seedOperationRow(nil, "2026-08-11", opPaid, "expense", searchTitlePatrol, 300000, searchSlugSecurity, "Охрана")
	h.seedOperationRow(nil, "2026-08-10", opPaid, "expense", searchTitleGuard, 250000, searchSlugSecurity, "Охрана")
	h.seedOperationRow(nil, "2026-08-20", opPlanned, "expense", "Охрана", 250000, searchSlugSecurity, "Охрана")
}

// searchList runs the paid property listing with the given query — the search
// screen's scope (only paid, no period bound) — and returns the row titles.
func (h *paymentsHarness) searchList(query string) []string {
	h.t.Helper()
	paid := domain.ViewStatusPaid
	items, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Status: &paid, Search: query, Limit: 50})
	if err != nil {
		h.t.Fatalf("list search %q: %v", query, err)
	}
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Operation.Title)
	}
	return titles
}

// searchSummary runs the paid summary with the given query and returns the
// breakdown slugs and the expense total.
func (h *paymentsHarness) searchSummary(query string) (slugs []string, expenseTotal int64) {
	h.t.Helper()
	paid := domain.ViewStatusPaid
	summary, err := h.ops.SummarizePropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsSummaryQuery{Status: &paid, Search: query})
	if err != nil {
		h.t.Fatalf("summary search %q: %v", query, err)
	}
	slugs = make([]string, 0, len(summary.Categories))
	for _, category := range summary.Categories {
		slugs = append(slugs, category.Slug)
	}
	return slugs, summary.ExpenseTotalKopecks
}

func TestOperationsListing_SearchMatchesTitleCategoryAndAmount(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.searchFixture()

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{
			"title substring, case-insensitive", "ОХРА",
			[]string{searchTitlePatrol, searchTitleGuard},
		},
		{
			"category snapshot substring", "клин",
			[]string{searchTitleCleaning},
		},
		{
			"amount digits full match", "2500",
			[]string{searchTitleDeposit, searchTitleCleaning, searchTitleGuard},
		},
		{
			"amount digits with separators", "2 500",
			[]string{searchTitleDeposit, searchTitleCleaning, searchTitleGuard},
		},
		{
			"amount digits cover kopecks", "250050",
			[]string{searchTitleDeposit},
		},
		{
			"letters with digits keep amount matching off", "клининг 500",
			nil,
		},
		{
			"like metacharacters are literals", "%",
			nil,
		},
		{
			"no match", "ипотека",
			nil,
		},
	}
	for _, tc := range cases {
		got := h.searchList(tc.query)
		if len(got) != len(tc.want) {
			t.Errorf("search %q listed %v, want %v", tc.query, got, tc.want)
			continue
		}
		for i, title := range tc.want {
			if got[i] != title {
				t.Errorf("search %q listed %v, want %v", tc.query, got, tc.want)
				break
			}
		}
	}
}

func TestOperationsListing_EmptySearchDisablesTheFilter(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.searchFixture()

	got := h.searchList("")
	if len(got) != 4 {
		t.Fatalf("empty search listed %v, want all four paid rows", got)
	}
}

func TestSummarizePropertyOperations_SearchNarrowsBreakdownToMatches(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.searchFixture()

	slugs, expenseTotal := h.searchSummary("охра")
	if len(slugs) != 1 || slugs[0] != searchSlugSecurity {
		t.Fatalf("search breakdown = %v, want [security] — matching rows only", slugs)
	}
	if expenseTotal != 550000 {
		t.Fatalf("search expense total = %d, want 550 000 (2 500 + 3 000)", expenseTotal)
	}

	slugs, _ = h.searchSummary("250050")
	if len(slugs) != 1 || slugs[0] != "deposit" {
		t.Fatalf("amount search breakdown = %v, want [deposit]", slugs)
	}
}
