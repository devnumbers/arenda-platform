//go:build integration

package application_test

// The paid_date sort family (ticket #992, баг #933): the operational feeds
// read by the actual payment date — sort=paid_date flips the listing order,
// the global feed's keyset cursor and the period filters and summaries onto
// paid_date — while the planned date stays the payment history's reading
// (the contract default). The owner's acceptance seed (#992): A planned
// 09-01 paid 09-20, B planned 09-10 paid 09-15 — desc by the fact is A, B,
// desc by the plan is B, A.

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// seedPaidOnOperation inserts one operation whose planned date and payment
// fact diverge — the #933 skew the paid_date sort exists for. Planned rows
// carry no fact (paid_date NULL), paid rows pay on the given date.
func (h *paymentsHarness) seedPaidOnOperation(
	propertyID, ownerID uuid.UUID, date, paidDate, status, title string,
	amountKopecks int64, slug, label string,
) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	var paid any
	if status == opPaid {
		paid = paidDate
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO operations (id, owner_id, property_id, origin, date, paid_date,
		                       status, type, title, amount_kopecks, category_label, category_slug)
		 VALUES ($1, $2, $3, 'payment', $4, $5::date, $6, 'expense', $7, $8, $9, $10)`,
		id, ownerID, propertyID, date, paid, status, title, amountKopecks, label, slug,
	); err != nil {
		h.t.Fatalf("seed operation %s (paid %s): %v", date, paidDate, err)
	}
	return id
}

// paidDateFixture is the ticket's acceptance book on one property: A
// (plan 09-01, paid 09-20, expense), B (plan 09-10, paid 09-15, expense)
// and C standing planned on 09-05 with no fact — the mixed-scope row.
func (h *paymentsHarness) paidDateFixture() {
	h.t.Helper()
	h.seedPaidOnOperation(h.propID, h.owner, "2026-09-01", "2026-09-20", opPaid,
		"Аренда", 2500000, testIntegrationSlugRent, "Арендная плата")
	h.seedPaidOnOperation(h.propID, h.owner, "2026-09-10", "2026-09-15", opPaid,
		"ЖКУ", 500000, testIntegrationSlugUtilities, "Коммунальные услуги")
	h.seedPaidOnOperation(h.propID, h.owner, "2026-09-05", "", opPlanned,
		"Клининг", 300000, "cleaning", "Клининг")
}

// propertyTitles projects a property listing onto titles in feed order.
func propertyTitles(items []paymentsapp.OperationListItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Operation.Title)
	}
	return out
}

// The owner's acceptance: desc by the fact is A, B — the plan order would
// read B, A. The planned row sinks below both facts in either direction.
func TestListPropertyOperations_PaidDateSort(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.paidDateFixture()
	paid := paymentsapp.SortByPaidDate

	desc, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Sort: paid, Limit: 10})
	if err != nil {
		t.Fatalf("paid desc: %v", err)
	}
	if !titlesEqual(propertyTitles(desc), "Аренда", "ЖКУ", "Клининг") {
		t.Fatalf("paid_date desc = %v, want A, B then the factless planned row (the plan order would be ЖКУ, Аренда)",
			propertyTitles(desc))
	}

	asc, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Sort: paid, Asc: true, Limit: 10})
	if err != nil {
		t.Fatalf("paid asc: %v", err)
	}
	if !titlesEqual(propertyTitles(asc), "ЖКУ", "Аренда", "Клининг") {
		t.Fatalf("paid_date asc = %v, want B, A with the planned row still last", propertyTitles(asc))
	}

	// The default stays the payment history's order: the planned date.
	def, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Limit: 10})
	if err != nil {
		t.Fatalf("default: %v", err)
	}
	if !titlesEqual(propertyTitles(def), "ЖКУ", "Клининг", "Аренда") {
		t.Fatalf("default desc = %v, want the plan order (B 09-10, C 09-05, A 09-01)", propertyTitles(def))
	}
}

// The period bounds follow the sort key: a window over the facts holds A
// (paid 09-20) and misses B (paid 09-15, plan 09-10 inside the window) —
// under the default sort the same window is empty.
func TestListPropertyOperations_PaidDatePeriod(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.paidDateFixture()
	paid := paymentsapp.SortByPaidDate

	from := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	byFact, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Sort: paid, DateFrom: &from, DateTo: &to, Limit: 10})
	if err != nil {
		t.Fatalf("paid period: %v", err)
	}
	if !titlesEqual(propertyTitles(byFact), "Аренда") {
		t.Fatalf("paid_date window [17–22] = %v, want A only (paid 09-20)", propertyTitles(byFact))
	}

	byPlan, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{DateFrom: &from, DateTo: &to, Limit: 10})
	if err != nil {
		t.Fatalf("default period: %v", err)
	}
	if len(byPlan) != 0 {
		t.Fatalf("default window [17–22] = %v, want empty — no plan falls inside", propertyTitles(byPlan))
	}
}

// The summary's period follows the sort key the same way (ticket #992): the
// totals aggregate the facts inside the window at sort=paid_date, the plan
// vocabulary at the default.
func TestSummarizePropertyOperations_PaidDatePeriod(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.paidDateFixture()
	paid := paymentsapp.SortByPaidDate

	from := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	byFact, err := h.ops.SummarizePropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsSummaryQuery{Sort: paid, DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("paid summary: %v", err)
	}
	if byFact.ExpenseTotalKopecks != 2500000 {
		t.Fatalf("paid_date summary expense = %d, want 2 500 000 — only A's fact falls in the window",
			byFact.ExpenseTotalKopecks)
	}
	// The category breakdown follows the same fact period: the single rent
	// chip (ticket #992 — the summary reads paid_date under the paid sort).
	if len(byFact.Categories) != 1 || byFact.Categories[0].Slug != testIntegrationSlugRent ||
		byFact.Categories[0].TotalKopecks != 2500000 {
		t.Fatalf("paid_date breakdown = %+v, want the single rent chip at 2 500 000", byFact.Categories)
	}

	byPlan, err := h.ops.SummarizePropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsSummaryQuery{DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("default summary: %v", err)
	}
	if byPlan.ExpenseTotalKopecks != 0 || len(byPlan.Categories) != 0 {
		t.Fatalf("default summary = %d/%+v, want zero — no plan falls in the window",
			byPlan.ExpenseTotalKopecks, byPlan.Categories)
	}
}

// globalPaidFixture seeds three paid rows whose fact order inverts the plan
// order on the harness's own property: X (plan 09-01, paid 09-25), Y (plan
// 09-10, paid 09-20), Z (plan 09-20, paid 09-05).
func (h *paymentsHarness) globalPaidFixture() {
	h.t.Helper()
	h.seedPaidOnOperation(h.propID, h.owner, "2026-09-01", "2026-09-25", opPaid,
		"Аренда", 2500000, testIntegrationSlugRent, "Арендная плата")
	h.seedPaidOnOperation(h.propID, h.owner, "2026-09-10", "2026-09-20", opPaid,
		"ЖКУ", 500000, testIntegrationSlugUtilities, "Коммунальные услуги")
	h.seedPaidOnOperation(h.propID, h.owner, "2026-09-20", "2026-09-05", opPaid,
		"Охрана", 250000, searchSlugSecurity, "Охрана")
}

func TestListGlobalOperations_PaidDateSort(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.globalPaidFixture()
	paid := paymentsapp.SortByPaidDate

	desc, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Sort: paid})
	if err != nil {
		t.Fatalf("paid desc: %v", err)
	}
	if !titlesEqual(feedTitles(desc.Items), "Аренда", "ЖКУ", "Охрана") {
		t.Fatalf("paid_date desc = %v, want the fact order (the plan order would be Охрана, ЖКУ, Аренда)",
			feedTitles(desc.Items))
	}

	asc, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Sort: paid, Asc: true})
	if err != nil {
		t.Fatalf("paid asc: %v", err)
	}
	if !titlesEqual(feedTitles(asc.Items), "Охрана", "ЖКУ", "Аренда") {
		t.Fatalf("paid_date asc = %v, want the fact order oldest-first", feedTitles(asc.Items))
	}

	// The period bounds read the fact under the same sort.
	from := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	window, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Sort: paid, DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("paid window: %v", err)
	}
	if !titlesEqual(feedTitles(window.Items), "Аренда", "ЖКУ") {
		t.Fatalf("paid_date window [15–30] = %v, want the two late facts", feedTitles(window.Items))
	}
	if window.Total != 2 {
		t.Fatalf("total = %d, want 2 — «найдено N» counts the same fact predicate", window.Total)
	}

	// The default sort reads the plan: the same window holds only Z's plan.
	byPlan, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("default window: %v", err)
	}
	if !titlesEqual(feedTitles(byPlan.Items), "Охрана") {
		t.Fatalf("default window [15–30] = %v, want Z only (plan 09-20)", feedTitles(byPlan.Items))
	}
}

func TestSummarizeGlobalOperations_PaidDatePeriod(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.globalPaidFixture()
	paid := paymentsapp.SortByPaidDate

	from := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	byFact, err := h.ops.SummarizeGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsSummaryQuery{Sort: paid, DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("paid summary: %v", err)
	}
	if byFact.ExpenseTotalKopecks != 3000000 {
		t.Fatalf("paid_date summary expense = %d, want 3 000 000 (Аренда + ЖКУ facts)",
			byFact.ExpenseTotalKopecks)
	}
	// The breakdown follows the fact period: rent first, utilities second.
	if len(byFact.Categories) != 2 ||
		byFact.Categories[0].Slug != testIntegrationSlugRent || byFact.Categories[0].TotalKopecks != 2500000 ||
		byFact.Categories[1].Slug != testIntegrationSlugUtilities || byFact.Categories[1].TotalKopecks != 500000 {
		t.Fatalf("paid_date breakdown = %+v, want rent 2 500 000 then utilities 500 000", byFact.Categories)
	}

	byPlan, err := h.ops.SummarizeGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsSummaryQuery{DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("default summary: %v", err)
	}
	if byPlan.ExpenseTotalKopecks != 250000 || len(byPlan.Categories) != 1 ||
		byPlan.Categories[0].Slug != searchSlugSecurity {
		t.Fatalf("default summary = %d/%+v, want the single security chip at 250 000 — only Z's plan falls in the window",
			byPlan.ExpenseTotalKopecks, byPlan.Categories)
	}
}

// The keyset cursor is bound to the sort it was issued under: a cursor from
// a planned-date page echoed into a paid-date walk (and the other way) is
// the contract's 400 — pagination would walk one field and sort by another.
func TestListGlobalOperations_CursorBoundToSort(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.globalPaidFixture()
	paid := paymentsapp.SortByPaidDate

	datePage, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Limit: 1})
	if err != nil {
		t.Fatalf("date page: %v", err)
	}
	if datePage.NextCursor == "" {
		t.Fatal("date page nextCursor = '', want a continuation")
	}
	if _, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Sort: paid, Limit: 1, Cursor: datePage.NextCursor}); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("paid_date walk with a date cursor err = %v, want ErrInvalidInput", err)
	}

	paidPage, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Sort: paid, Limit: 1})
	if err != nil {
		t.Fatalf("paid page: %v", err)
	}
	if _, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
		paymentsapp.GlobalOperationsListQuery{Limit: 1, Cursor: paidPage.NextCursor}); !errors.Is(err, paymentsapp.ErrInvalidInput) {
		t.Fatalf("date walk with a paid_date cursor err = %v, want ErrInvalidInput", err)
	}
}

// The paid_date walk (the #597 guarantee under the new key): the fact dates
// collide in pairs, so the walk exercises the (paid_date, id) keyset key —
// the full feed in fact order, every tie resolved by the descending id, no
// duplicates and no drops across the pages.
func TestListGlobalOperationsKeyset_PaidDateWalk(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	const total = 30
	const pageSize = 7
	// Plan ascending, fact exactly reversed and folded into pairs: two rows
	// share every fact date, and the plan order inverts the fact order —
	// walking by the wrong field would mangle the sequence visibly.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	factOf := func(i int) time.Time { return base.Add(time.Duration((total-1-i)/2) * 24 * time.Hour) }
	factByTitle := make(map[string]time.Time, total)
	for i := range total {
		title := fmt.Sprintf("Операция %03d", i)
		h.seedPaidOnOperation(h.propID, h.owner,
			base.Add(time.Duration(i)*24*time.Hour).Format("2006-01-02"),
			factOf(i).Format("2006-01-02"), opPaid,
			title, 100000+int64(i), testIntegrationSlugUtilities, "Коммунальные услуги")
		factByTitle[title] = factOf(i)
	}

	assertWalk := func(asc bool) {
		h.t.Helper()
		var got []paymentsapp.OperationListItem
		cursor := ""
		for {
			page, err := h.ops.ListGlobalOperations(h.ctx(), h.owner,
				paymentsapp.GlobalOperationsListQuery{
					Sort: paymentsapp.SortByPaidDate, Limit: pageSize, Cursor: cursor, Asc: asc,
				})
			if err != nil {
				h.t.Fatalf("paid walk page (cursor %q): %v", cursor, err)
			}
			got = append(got, page.Items...)
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		if len(got) != total {
			h.t.Fatalf("%s walk carries %d rows, want %d — dropped or duplicated",
				walkDirection(asc), len(got), total)
		}
		assertPaidDateKeyOrder(h.t, got, factByTitle, asc)
	}

	assertWalk(false)
	assertWalk(true)
}

// walkDirection names the walk in the assertion messages.
func walkDirection(asc bool) string {
	if asc {
		return "asc"
	}
	return "desc"
}

// assertPaidDateKeyOrder asserts the walked feed reads the (paid_date, id)
// keyset key: the fact order follows the direction, every tie resolves by
// the descending id (the tiebreak runs DESC in both directions), and no row
// arrives twice.
func assertPaidDateKeyOrder(
	t *testing.T, items []paymentsapp.OperationListItem, factByTitle map[string]time.Time, asc bool,
) {
	t.Helper()
	seen := make(map[string]bool, len(items))
	for i, item := range items {
		if seen[item.Operation.Title] {
			t.Fatalf("%s row %q arrived twice", walkDirection(asc), item.Operation.Title)
		}
		seen[item.Operation.Title] = true
		if i == 0 {
			continue
		}
		prev := items[i-1].Operation
		prevFact := factByTitle[prev.Title]
		curFact := factByTitle[item.Operation.Title]
		factMisordered := prevFact.Before(curFact)
		if asc {
			factMisordered = prevFact.After(curFact)
		}
		if factMisordered || (prevFact.Equal(curFact) && bytes.Compare(prev.ID[:], item.Operation.ID[:]) < 0) {
			t.Fatalf("%s rows %d/%d break the (paid_date, id desc) key: %s then %s",
				walkDirection(asc), i-1, i, prev.Title, item.Operation.Title)
		}
	}
}

// The domain view status is not the sort's business: the status filter keeps
// its planning vocabulary (overdue splits on the plan) under paid_date too.
func TestListPropertyOperations_PaidDateKeepsStatusVocabulary(t *testing.T) {
	t.Parallel()
	// Harness today is 2026-08-25 (Moscow): the 08-12 plan is overdue.
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	h.seedPaidOnOperation(h.propID, h.owner, "2026-08-12", "2026-09-01", opPlanned,
		"Просрочка", 400000, testIntegrationSlugUtilities, "Коммунальные услуги")
	h.seedPaidOnOperation(h.propID, h.owner, "2026-08-05", "2026-08-06", opPaid,
		"Факт", 500000, testIntegrationSlugUtilities, "Коммунальные услуги")
	overdue := domain.ViewStatusOverdue
	paid := paymentsapp.SortByPaidDate

	rows, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Sort: paid, Status: &overdue, Limit: 10})
	if err != nil {
		t.Fatalf("paid+overdue: %v", err)
	}
	if !titlesEqual(propertyTitles(rows), "Просрочка") {
		t.Fatalf("paid_date + status=overdue = %v, want the overdue plan row — the status split stays on the plan",
			propertyTitles(rows))
	}
}
