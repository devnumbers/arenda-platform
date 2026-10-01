//go:build integration

package application_test

// The object payments list's next payment date (ticket #991): one server
// resolution lands on every row — the stored planned wins, the prepaid
// nearest moves the date to the next one (the #967 bug's shape), the open
// pause and the settled rule resolve to null, the never-materialized rule
// projects. The harness clock (2026-08-25 20:00 UTC) makes Moscow's today
// the 25th; the fixtures seed rules directly, the reads never tick.

import (
	"testing"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
)

// The fixtures' recurring literals (goconst).
const (
	monthlyDay30 = `{"kind":"monthly","daysOfMonth":[30]}`
	monthlyDay1  = `{"kind":"monthly","daysOfMonth":[1]}`
	dailyKind    = `{"kind":"daily"}`

	dayAug15 = "2026-08-15"
	dayAug20 = "2026-08-20"
	dayAug25 = "2026-08-25"
	dayAug30 = "2026-08-30"
	daySep1  = "2026-09-01"
	daySep30 = "2026-09-30"

	nearestAmount = int64(500000)
)

// seedNearestRule inserts a rule with the fixture's own since, recurrence
// and optional end date (” = open-ended) — the nearest-date fixtures' row.
func (h *paymentsHarness) seedNearestRule(title, since, recurrence, endDate string) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	var endArg any
	if endDate != "" {
		endArg = endDate
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
		                       recurrence, since, end_date, auto_pay, category_slug)
		 VALUES ($1, $2, $3, 'expense', $4, $5, $6::jsonb, $7, $8, false, 'utilities')`,
		id, h.owner, h.propID, title, nearestAmount, recurrence, since, endArg,
	); err != nil {
		h.t.Fatalf("seed nearest rule %s: %v", title, err)
	}
	return id
}

func TestListPayments_NearestDate(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")

	stored := h.seedNearestRule("Хранимая", "2026-08-01", monthlyDay30, "")
	h.seedRuleOperation(stored, h.propID, h.owner, dayAug15, opPlanned, typeExpense, "Хранимая", nearestAmount)
	h.seedRuleOperation(stored, h.propID, h.owner, dayAug30, opPlanned, typeExpense, "Хранимая", nearestAmount)

	prepaid := h.seedNearestRule("Предоплата", "2026-08-01", monthlyDay30, "")
	h.seedRuleOperation(prepaid, h.propID, h.owner, dayAug30, opPaid, typeExpense, "Предоплата", nearestAmount)
	h.seedRuleOperation(prepaid, h.propID, h.owner, daySep30, opPlanned, typeExpense, "Предоплата", nearestAmount)

	prepaidOnly := h.seedNearestRule("Предоплата без плановой", "2026-08-01", monthlyDay30, "")
	h.seedRuleOperation(prepaidOnly, h.propID, h.owner, dayAug30, opPaid, typeExpense, "Предоплата без плановой", nearestAmount)

	h.seedNearestRule("Проекция", dayAug25, monthlyDay1, "")

	paused := h.seedNearestRule("Пауза", "2026-08-01", dailyKind, "")
	h.seedRuleOperation(paused, h.propID, h.owner, dayAug20, opPaid, typeExpense, "Пауза", nearestAmount)
	h.seedPause(paused, dayAug25, nil)

	settled := h.seedNearestRule("Завершённый", "2026-08-01", dailyKind, "2026-08-10")
	h.seedRuleOperation(settled, h.propID, h.owner, "2026-08-10", opPaid, typeExpense, "Завершённый", nearestAmount)

	items, err := h.svc.ListPayments(h.ctx(), h.owner, h.propID, "")
	if err != nil {
		t.Fatalf("list payments: %v", err)
	}
	if len(items) != 6 {
		t.Fatalf("list = %d items, want 6", len(items))
	}
	byTitle := map[string]paymentsapp.PaymentListItem{}
	order := make([]string, 0, len(items))
	for _, item := range items {
		byTitle[item.Payment.Title] = item
		order = append(order, item.Payment.Title)
	}
	wantOrder := []string{"Хранимая", "Предоплата", "Предоплата без плановой", "Проекция", "Пауза", "Завершённый"}
	for i := range wantOrder {
		if order[i] != wantOrder[i] {
			t.Fatalf("order = %v, want %v (creation order intact)", order, wantOrder)
		}
	}

	cases := []struct {
		title string
		want  string // '' — the true null.
	}{
		// The stored planned wins; the overdue 08-15 never counts.
		{"Хранимая", dayAug30},
		// The prepaid nearest is not «следующая» — the next stored planned is.
		{"Предоплата", daySep30},
		// Without a stored planned the projection runs from the prepaid cursor.
		{"Предоплата без плановой", daySep30},
		// Nothing materialized — the pure projection from yesterday.
		{"Проекция", daySep1},
		// The open pause cuts the occurrences — the true null.
		{"Пауза", ""},
		// The settled rule has nothing past its cursor — the true null.
		{"Завершённый", ""},
	}
	for _, tc := range cases {
		got := byTitle[tc.title].NearestDate
		if tc.want == "" {
			if got != nil {
				t.Errorf("%s: nearest = %s, want null", tc.title, got.Format("2006-01-02"))
			}
			continue
		}
		if got == nil || got.Format("2006-01-02") != tc.want {
			t.Errorf("%s: nearest = %v, want %s", tc.title, got, tc.want)
		}
	}
}
