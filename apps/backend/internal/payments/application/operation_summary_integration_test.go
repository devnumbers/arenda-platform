//go:build integration

package application_test

// The summary integration tests of the operations screens slice (ticket #473):
// the period totals and the per-category breakdown behind the new «Операции
// объекта» screens, plus the type/category filters the screens put on the
// property listing — against real PostgreSQL through the shared harness.

import (
	"testing"
	"time"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// Shared fixture slugs of the integration tests: the summary fixtures reuse
// the default-catalog slug the CRUD fixtures already rent with.
const testIntegrationSlugRent = "rent"

// opCancelled is the stored tombstone status (CONTEXT.md «Отменённая
// операция»); the harness consts only carry the listing statuses.
const opCancelled = "cancelled"

// testIntegrationSlugUtilities is the default-catalog slug of the ЖКУ fixtures.
const testIntegrationSlugUtilities = "utilities"

// seedOperationRow inserts one operation row with the full fixture control —
// direction, category snapshot, amount — the summary tests need a diversity
// the seedOperation helper fixes to expense/ЖКУ. A nil paymentID seeds a
// manual fact; a nil slug seeds a row without a category snapshot.
func (h *paymentsHarness) seedOperationRow(
	paymentID *uuid.UUID, date, status, typ, title string, amountKopecks int64, slug, label string,
) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	var paid any
	if status == opPaid {
		paid = date
	}
	var pay any
	if paymentID != nil {
		pay = *paymentID
	}
	var slugArg any
	if slug != "" {
		slugArg = slug
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date, paid_date,
		                       status, type, title, amount_kopecks, category_label, category_slug)
		 VALUES ($1, $2, $3, $4, 'payment', $5, $6::date,
		         $7, $8, $9, $10, $11, $12)`,
		id, h.owner, h.propID, pay, date, paid, status, typ, title, amountKopecks, label, slugArg,
	); err != nil {
		h.t.Fatalf("seed operation row %s (%s): %v", date, status, err)
	}
	return id
}

// summaryFixture seeds the month's bookkeeping on one rule: three paid rows
// across three categories (one category twice), a planned row inside the
// window (must not leak into a paid summary) and a manual paid income.
func (h *paymentsHarness) summaryFixture(pay uuid.UUID) {
	h.t.Helper()
	h.seedOperationRow(&pay, "2026-08-10", opPaid, "income", "Аренда", 5650000, testIntegrationSlugRent, "Арендная плата")
	h.seedOperationRow(&pay, "2026-08-12", opPaid, "expense", "ЖКУ", 1050000, testIntegrationSlugUtilities, "Коммунальные услуги")
	h.seedOperationRow(&pay, "2026-08-18", opPaid, "expense", "Охрана", 250000, "security", "Охрана")
	h.seedOperationRow(&pay, "2026-08-21", opPaid, "expense", "ЖКУ", 400000, testIntegrationSlugUtilities, "Коммунальные услуги")
	h.seedOperationRow(&pay, "2026-08-20", opPlanned, "expense", "ЖКУ", 300000, testIntegrationSlugUtilities, "Коммунальные услуги")
	h.seedOperationRow(nil, "2026-08-15", opPaid, "income", "Залог", 1000000, "deposit", "Залог")
}

// augustWindow is the inclusive [1st, 31st] period the summary tests read.
func augustWindow() (from, to time.Time) {
	return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
}

func TestSummarizePropertyOperations_TotalsAndCategories(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	h.summaryFixture(pay)

	from, to := augustWindow()
	paid := domain.ViewStatusPaid
	summary, err := h.ops.SummarizePropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsSummaryQuery{Status: &paid, DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}

	if summary.IncomeTotalKopecks != 6650000 {
		t.Errorf("income total = %d, want 6 650 000 (5 650 000 rent + 1 000 000 deposit)", summary.IncomeTotalKopecks)
	}
	if summary.ExpenseTotalKopecks != 1700000 {
		t.Errorf("expense total = %d, want 1 700 000 — the planned row must not leak", summary.ExpenseTotalKopecks)
	}
	if len(summary.Categories) != 4 {
		t.Fatalf("categories = %d rows (%+v), want 4", len(summary.Categories), summary.Categories)
	}
	wantOrder := []struct {
		slug  string
		total int64
	}{
		{testIntegrationSlugRent, 5650000},
		{testIntegrationSlugUtilities, 1450000},
		{"deposit", 1000000},
		{"security", 250000},
	}
	for i, want := range wantOrder {
		got := summary.Categories[i]
		if got.Slug != want.slug || got.TotalKopecks != want.total {
			t.Errorf("categories[%d] = %s/%d, want %s/%d", i, got.Slug, got.TotalKopecks, want.slug, want.total)
		}
		if got.Type != domain.TypeIncome && got.Type != domain.TypeExpense {
			t.Errorf("categories[%d].type = %q, want income|expense", i, got.Type)
		}
		if got.Label == "" {
			t.Errorf("categories[%d].label = empty, want the snapshot label", i)
		}
	}
}

func TestSummarizePropertyOperations_TypeFiltersCategoriesOnly(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	h.summaryFixture(pay)

	from, to := augustWindow()
	paid := domain.ViewStatusPaid
	expense := domain.TypeExpense
	summary, err := h.ops.SummarizePropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsSummaryQuery{Status: &paid, DateFrom: &from, DateTo: &to, Type: &expense})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}

	// Totals always report both directions — the cards read them together —
	// while the type filter narrows only the category breakdown.
	if summary.IncomeTotalKopecks != 6650000 || summary.ExpenseTotalKopecks != 1700000 {
		t.Errorf("totals = %d/%d, want 6 650 000/1 700 000",
			summary.IncomeTotalKopecks, summary.ExpenseTotalKopecks)
	}
	for _, row := range summary.Categories {
		if row.Type != domain.TypeExpense {
			t.Errorf("category %s has type %s, want only expense rows", row.Slug, row.Type)
		}
	}
	if len(summary.Categories) != 2 {
		t.Fatalf("categories = %d rows (%+v), want the 2 expense categories", len(summary.Categories), summary.Categories)
	}
}

func TestSummarizePropertyOperations_OverdueIsAViewStatus(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	// Planned since the 12th — overdue against the Moscow today (the 25th).
	h.seedOperationRow(&pay, "2026-08-12", opPlanned, "expense", "ЖКУ", 700000, testIntegrationSlugUtilities, "Коммунальные услуги")

	from, to := augustWindow()
	overdue := domain.ViewStatusOverdue
	summary, err := h.ops.SummarizePropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsSummaryQuery{Status: &overdue, DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary.ExpenseTotalKopecks != 700000 || summary.IncomeTotalKopecks != 0 {
		t.Fatalf("overdue summary = %d/%d, want 0/700 000",
			summary.IncomeTotalKopecks, summary.ExpenseTotalKopecks)
	}

	paid := domain.ViewStatusPaid
	summary, err = h.ops.SummarizePropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsSummaryQuery{Status: &paid, DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("paid summarize: %v", err)
	}
	if summary.ExpenseTotalKopecks != 0 || summary.IncomeTotalKopecks != 0 {
		t.Fatalf("paid summary = %d/%d, want 0/0 — the overdue row is not a paid fact",
			summary.IncomeTotalKopecks, summary.ExpenseTotalKopecks)
	}
}

func TestSummarizePropertyOperations_EmptyPeriodIsEmptySummary(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	from, to := augustWindow()
	summary, err := h.ops.SummarizePropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsSummaryQuery{DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary.IncomeTotalKopecks != 0 || summary.ExpenseTotalKopecks != 0 || len(summary.Categories) != 0 {
		t.Fatalf("summary = %+v, want the zero summary", summary)
	}
}

func TestSummarizePropertyOperations_CancelledNeverCounts(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	h.summaryFixture(pay)
	// Надгробие удалённой операции: факт и долг исчезли — из сводки тоже
	// (CONTEXT.md «Отменённая операция»: для всех чтений её не существует).
	h.seedOperationRow(&pay, "2026-08-14", opCancelled, "expense", "Клининг", 900000, "cleaning", "Клининг")

	from, to := augustWindow()
	paid := domain.ViewStatusPaid
	summary, err := h.ops.SummarizePropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsSummaryQuery{Status: &paid, DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary.IncomeTotalKopecks != 6650000 || summary.ExpenseTotalKopecks != 1700000 {
		t.Fatalf("totals = %d/%d, want 6 650 000/1 700 000 — cancelled must not count",
			summary.IncomeTotalKopecks, summary.ExpenseTotalKopecks)
	}
	for _, row := range summary.Categories {
		if row.Slug == "cleaning" {
			t.Fatalf("cancelled category %s leaked into the breakdown", row.Slug)
		}
	}
}

func TestOperationsSummaryAndListing_UserCategoryRows(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	// Операция с пользовательской категорией: снапшот слага отсутствует
	// (category_slug NULL), подпись — имя пользовательской категории.
	h.seedOperationRow(&pay, "2026-08-13", opPaid, "expense", "Свой расход", 123000, "", "Своя категория")

	from, to := augustWindow()
	paid := domain.ViewStatusPaid

	// Сводка: сумма считается в тотале, но строки-чипа у категории нет.
	summary, err := h.ops.SummarizePropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsSummaryQuery{Status: &paid, DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary.ExpenseTotalKopecks != 123000 {
		t.Fatalf("expense total = %d, want 123 000 — no snapshot still counts", summary.ExpenseTotalKopecks)
	}
	if len(summary.Categories) != 0 {
		t.Fatalf("categories = %+v, want none — a row without a snapshot has no chip identity", summary.Categories)
	}

	// Список: категории-фильтр её не находит, без фильтра — видно.
	filtered, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Categories: []string{"cleaning", testIntegrationSlugUtilities}, DateFrom: &from, DateTo: &to, Limit: 50})
	if err != nil {
		t.Fatalf("category filter: %v", err)
	}
	if len(filtered) != 0 {
		t.Fatalf("category filter matched %+v, want nothing — NULL slug matches no value", filtered)
	}
	unfiltered, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Status: &paid, DateFrom: &from, DateTo: &to, Limit: 50})
	if err != nil {
		t.Fatalf("unfiltered list: %v", err)
	}
	if len(unfiltered) != 1 || unfiltered[0].Operation.Title != "Свой расход" {
		t.Fatalf("unfiltered list = %+v, want the user-category operation", unfiltered)
	}
}

func TestSummarizePropertyOperations_HidesForeignReaders(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	stranger := uuid.Must(uuid.NewV7())

	_, err := h.ops.SummarizePropertyOperations(h.ctx(), stranger, h.propID, paymentsapp.OperationsSummaryQuery{})
	wantAppError(t, err, paymentsapp.ErrNotFound)
}

func TestSummarizePropertyOperations_ViewerReads(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarnessWithPolicy(t, stubPropertyPolicy{role: sharedpolicy.RoleViewer}).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	h.summaryFixture(pay)

	from, to := augustWindow()
	summary, err := h.ops.SummarizePropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsSummaryQuery{DateFrom: &from, DateTo: &to})
	if err != nil {
		t.Fatalf("viewer summarize: %v", err)
	}
	if summary.ExpenseTotalKopecks == 0 {
		t.Fatal("viewer summary lost the expenses — the read must mirror the listing's scope")
	}
}

func TestOperationsListing_TypeAndCategoryFilters(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	pay := h.seedPayment(day22, `{"kind": "monthly", "dayOfMonth": 15}`, false)
	h.summaryFixture(pay)

	from, to := augustWindow()
	expense := domain.TypeExpense
	expenses, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Type: &expense, DateFrom: &from, DateTo: &to, Limit: 50})
	if err != nil {
		t.Fatalf("type filter: %v", err)
	}
	if len(expenses) != 4 {
		t.Fatalf("type=expense listed %d rows, want 4 (3 paid + the planned one)", len(expenses))
	}
	for _, item := range expenses {
		if item.Operation.Type != domain.TypeExpense {
			t.Fatalf("type=expense leaked an income row %q", item.Operation.Title)
		}
	}

	security := []string{"security"}
	guarded, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Categories: security, DateFrom: &from, DateTo: &to, Limit: 50})
	if err != nil {
		t.Fatalf("category filter: %v", err)
	}
	if len(guarded) != 1 || guarded[0].Operation.Title != "Охрана" {
		t.Fatalf("categories=[security] listed %+v, want the single Охрана row", guarded)
	}

	mixed := []string{"rent", testIntegrationSlugUtilities}
	both, err := h.ops.ListPropertyOperations(h.ctx(), h.owner, h.propID,
		paymentsapp.OperationsListQuery{Categories: mixed, DateFrom: &from, DateTo: &to, Limit: 50})
	if err != nil {
		t.Fatalf("multi-category filter: %v", err)
	}
	if len(both) != 4 {
		t.Fatalf("categories=[rent,utilities] listed %d rows, want 4", len(both))
	}
}
