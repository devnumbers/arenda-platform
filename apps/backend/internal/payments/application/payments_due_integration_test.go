//go:build integration

package application_test

// The «На оплату» read's integration tests (ticket #1346): the reminder
// window (D−N ≤ today ≤ D, today the property owner's — ADR 0048) over the
// reminder rules without auto-pay — the owner's decision — against real
// PostgreSQL. The matrix pins the window's both bounds, the statuses (paid
// and cancelled are not the section's material), the rule flags (auto-pay
// out, reminder-less out, completed out), the bucket order («Сначала
// новые» inside), the feed's visibility predicate and the scope counters.

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	paymentsapp "github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
)

// seedDueRule inserts a rule with the reminder lead and the flags the
// window matrix turns: reminderOffset nil — no reminder, autoPay the
// owner's decision knob, completed the terminal #1340 stamp.
func (h *paymentsHarness) seedDueRule(
	propertyID, ownerID uuid.UUID, title string, reminderOffset *int, autoPay, completed bool,
) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	var offset any
	if reminderOffset != nil {
		offset = *reminderOffset
	}
	var completedAt any
	if completed {
		completedAt = "2026-08-20 12:00:00+00"
	}
	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
		                       recurrence, since, auto_pay, category_slug, reminder_offset_days, completed_at)
		 VALUES ($1, $2, $3, 'expense', $4, 500000, '{"kind":"monthly","daysOfMonth":[25]}'::jsonb,
		         '2026-07-01', $5, 'utilities', $6, $7)`,
		id, ownerID, propertyID, title, autoPay, offset, completedAt,
	); err != nil {
		h.t.Fatalf("seed due rule %s: %v", title, err)
	}
	return id
}

// The window matrix (ticket #1346): the in-window dates ride the rule's own
// lead (D−N ≤ today ≤ D — today the Moscow owner's 2026-08-25), the bounds
// hold both sides, the statuses and the flag rules cut. The list returns
// the whole window in one read.
func TestListPaymentDue_Window(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	three := 3
	seven := 7
	one := 1

	// The matrix rule (N=3): today's operation, the next future, the upper
	// boundary (D = today + N) — and the out-by-one beyond it.
	matrix := h.seedDueRule(h.propID, h.owner, "ЖКУ", &three, false, false)
	h.seedRuleOperation(matrix, h.propID, h.owner, day25, opPlanned, "expense", "ЖКУ", 500000)
	h.seedRuleOperation(matrix, h.propID, h.owner, day26, opPlanned, "expense", "ЖКУ", 500000)
	h.seedRuleOperation(matrix, h.propID, h.owner, "2026-08-28", opPlanned, "expense", "ЖКУ", 500000)
	h.seedRuleOperation(matrix, h.propID, h.owner, "2026-08-29", opPlanned, "expense", "ЖКУ", 500000)
	// The overdue debt and the closed facts are not the section's material
	// (one operation per (rule, date) — the closed ones take other dates).
	h.seedRuleOperation(matrix, h.propID, h.owner, day22, opPlanned, "expense", "ЖКУ", 500000)
	h.seedRuleOperation(matrix, h.propID, h.owner, day23, opPaid, "expense", "ЖКУ", 500000)
	h.seedRuleOperation(matrix, h.propID, h.owner, "2026-08-21", "cancelled", "expense", "ЖКУ", 500000)

	// The lower boundary holds for a longer lead too: N=7 reaches the 1st of
	// September, not the 2nd.
	week := h.seedDueRule(h.propID, h.owner, "Страхование", &seven, false, false)
	h.seedRuleOperation(week, h.propID, h.owner, "2026-09-01", opPlanned, "expense", "Страхование", 500000)
	h.seedRuleOperation(week, h.propID, h.owner, "2026-09-02", opPlanned, "expense", "Страхование", 500000)

	// The owner's decision: an auto-pay rule with a reminder never enters —
	// its reminder fires, the section does not contain it.
	autoPay := h.seedDueRule(h.propID, h.owner, "Интернет автоплатёж", &one, true, false)
	h.seedRuleOperation(autoPay, h.propID, h.owner, day25, opPlanned, "expense", "Интернет", 500000)

	// A rule without a reminder is out whatever the dates.
	plain := h.seedDueRule(h.propID, h.owner, "Клининг", nil, false, false)
	h.seedRuleOperation(plain, h.propID, h.owner, day25, opPlanned, "expense", "Клининг", 500000)

	// A completed rule is cut by the feed's inherited gate (#1340).
	settled := h.seedDueRule(h.propID, h.owner, "Завершённый", &one, false, true)
	h.seedRuleOperation(settled, h.propID, h.owner, day25, opPlanned, "expense", "Завершённый", 500000)

	due, err := svc.ListPaymentDue(h.ctx(), h.owner)
	if err != nil {
		t.Fatalf("list payment due: %v", err)
	}
	if len(due.Items) != 4 {
		t.Fatalf("items = %d, want the four in-window rows (25th, 26th, 28th, 1st)", len(due.Items))
	}
	if due.DueOperationsCount != 4 {
		t.Errorf("dueOperationsCount = %d, want 4", due.DueOperationsCount)
	}
	days := map[int]paymentsapp.PaymentDueItem{}
	for _, item := range due.Items {
		days[item.DaysLeft] = item
	}
	for daysLeft, wantDate := range map[int]string{0: day25, 1: day26, 3: "2026-08-28", 7: "2026-09-01"} {
		item, ok := days[daysLeft]
		if !ok {
			t.Errorf("no row with daysLeft %d", daysLeft)
			continue
		}
		if got := dateOf(item.DueDate); got != wantDate {
			t.Errorf("daysLeft %d: dueDate = %s, want %s", daysLeft, got, wantDate)
		}
		if got := dateOf(item.Today); got != day25 {
			t.Errorf("daysLeft %d: today = %s, want the Moscow owner's %s", daysLeft, got, day25)
		}
	}
	if item := days[0]; item.Title != "ЖКУ" || item.ReminderOffsetDays != 3 {
		t.Errorf("today's row = (%q, N %d), want the matrix rule with N=3", item.Title, item.ReminderOffsetDays)
	}
	if item := days[7]; item.Title != "Страхование" || item.ReminderOffsetDays != 7 {
		t.Errorf("boundary row = (%q, N %d), want the weekly lead", item.Title, item.ReminderOffsetDays)
	}
}

// The reading order (ticket #1346, #1347): the rows run the real days
// ascending (решение владельца 13.10 — the page groups the rows per real
// day and the chip reverses the group order client-side), the operation's
// id the tiebreak of the same-day pair.
func TestListPaymentDue_Order(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	one := 1
	three := 3
	seven := 7
	ruleA := h.seedDueRule(h.propID, h.owner, "Аренда", &seven, false, false)
	h.seedRuleOperation(ruleA, h.propID, h.owner, day26, opPlanned, "expense", "Аренда", 500000)
	h.seedRuleOperation(ruleA, h.propID, h.owner, "2026-08-28", opPlanned, "expense", "Аренда", 500000)
	ruleB := h.seedDueRule(h.propID, h.owner, "ЖКУ", &one, false, false)
	h.seedRuleOperation(ruleB, h.propID, h.owner, day25, opPlanned, "expense", "ЖКУ", 500000)
	ruleC := h.seedDueRule(h.propID, h.owner, "Страхование", &three, false, false)
	h.seedRuleOperation(ruleC, h.propID, h.owner, "2026-08-27", opPlanned, "expense", "Страхование", 500000)
	h.seedRuleOperation(ruleC, h.propID, h.owner, "2026-08-28", opPlanned, "expense", "Страхование", 500000)

	due, err := svc.ListPaymentDue(h.ctx(), h.owner)
	if err != nil {
		t.Fatalf("list payment due: %v", err)
	}
	if len(due.Items) != 5 {
		t.Fatalf("items = %d, want 5", len(due.Items))
	}
	wantDays := []int{0, 1, 2, 3, 3}
	for i, want := range wantDays {
		if due.Items[i].DaysLeft != want {
			t.Fatalf("row %d daysLeft = %d, want the per-day order %v", i, due.Items[i].DaysLeft, wantDays)
		}
	}
	// The daysLeft-3 pair (rows 3 and 4) shares the due date; the same-day
	// tiebreak runs the newer (larger) operation id first.
	first, second := due.Items[3].OperationID, due.Items[4].OperationID
	if bytes.Compare(first[:], second[:]) <= 0 {
		t.Errorf("tie order = %s then %s, want the newer operation id first", first, second)
	}
}

// The feed's visibility predicate (ticket #521, the actor-scoped
// cross-property read) with the archive cut, and the per-owner today
// (ADR 0048): the shared Kamchatka property's row runs its schedule math
// against the Kamchatka owner's 26th, not the reading actor's 25th.
func TestListPaymentDue_Visibility(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	one := 1
	own := h.seedDueRule(h.propID, h.owner, "ЖКУ", &one, false, false)
	h.seedRuleOperation(own, h.propID, h.owner, day25, opPlanned, "expense", "ЖКУ", 500000)

	archived := h.seedGlobalProperty(h.owner, "Старый объект", "archived")
	archivedRule := h.seedDueRule(archived, h.owner, "ЖКУ архива", &one, false, false)
	h.seedRuleOperation(archivedRule, archived, h.owner, day25, opPlanned, "expense", "ЖКУ архива", 500000)

	foreignOwner := h.seedGlobalUser("Europe/Moscow")
	foreign := h.seedGlobalProperty(foreignOwner, "Без доступа", "active")
	foreignRule := h.seedDueRule(foreign, foreignOwner, "Чужое", &one, false, false)
	h.seedRuleOperation(foreignRule, foreign, foreignOwner, day25, opPlanned, "expense", "Чужое", 500000)

	suspendedProp := h.seedGlobalProperty(foreignOwner, "Заморожено", "active")
	h.seedMembership(suspendedProp, h.owner, foreignOwner, "viewer", "suspended")
	suspendedRule := h.seedDueRule(suspendedProp, foreignOwner, "Замороженное ЖКУ", &one, false, false)
	h.seedRuleOperation(suspendedRule, suspendedProp, foreignOwner, day25, opPlanned, "expense", "Замороженное ЖКУ", 500000)

	partner := h.seedGlobalUser("Asia/Kamchatka")
	shared := h.seedGlobalProperty(partner, "Чужая дача", "active")
	h.seedMembership(shared, h.owner, partner, "viewer", "active")
	sharedRule := h.seedDueRule(shared, partner, "Аренда", &one, false, false)
	h.seedRuleOperation(sharedRule, shared, partner, day26, opPlanned, "expense", "Аренда", 500000)

	due, err := svc.ListPaymentDue(h.ctx(), h.owner)
	if err != nil {
		t.Fatalf("list payment due: %v", err)
	}
	if len(due.Items) != 2 {
		t.Fatalf("items = %d, want the own and the shared-active rows", len(due.Items))
	}
	byTitle := map[string]paymentsapp.PaymentDueItem{}
	for _, item := range due.Items {
		byTitle[item.Title] = item
	}
	if _, ok := byTitle["ЖКУ"]; !ok {
		t.Error("the own rule's row is missing")
	}
	sharedRow, ok := byTitle["Аренда"]
	if !ok {
		t.Fatal("the shared-active rule's row is missing")
	}
	if got := dateOf(sharedRow.Today); got != day26 {
		t.Errorf("shared today = %s, want the Kamchatka owner's %s", got, day26)
	}
	if sharedRow.DaysLeft != 0 {
		t.Errorf("shared daysLeft = %d, want 0 — the row's own owner's today", sharedRow.DaysLeft)
	}
}

// The scope counters (ticket #1346): the «На оплату» card's number and the
// nearest days-left ride the feed response beside the favorites and the
// overdue — the due aggregates never narrow the older counters, and an
// auto-pay rule's window operation counts nowhere.
func TestListGlobalPayments_DueCounters(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	one := 1
	favorite := h.seedGlobalRule(h.propID, h.owner, "Страхование", "expense", 3200000, false, true, "insurance", nil)
	h.seedRuleOperation(favorite, h.propID, h.owner, day25, opPlanned, "expense", "Страхование", 3200000)

	reminded := h.seedDueRule(h.propID, h.owner, "ЖКУ", &one, false, false)
	h.seedRuleOperation(reminded, h.propID, h.owner, day25, opPlanned, "expense", "ЖКУ", 500000)
	h.seedRuleOperation(reminded, h.propID, h.owner, day26, opPlanned, "expense", "ЖКУ", 500000)
	autoPay := h.seedDueRule(h.propID, h.owner, "Интернет автоплатёж", &one, true, false)
	h.seedRuleOperation(autoPay, h.propID, h.owner, day25, opPlanned, "expense", "Интернет", 500000)

	feed, err := svc.ListGlobalPayments(h.ctx(), h.owner)
	if err != nil {
		t.Fatalf("list global payments: %v", err)
	}
	if feed.FavoriteCount != 1 {
		t.Errorf("favoriteCount = %d, want 1 — the due terms never narrow it", feed.FavoriteCount)
	}
	if feed.OverdueOperationsCount != 0 {
		t.Errorf("overdueOperationsCount = %d, want 0 — window operations are not overdue", feed.OverdueOperationsCount)
	}
	if feed.DueOperationsCount != 2 {
		t.Errorf("dueOperationsCount = %d, want 2 — the auto-pay row counts nowhere", feed.DueOperationsCount)
	}
	if feed.DueNearestDays == nil || *feed.DueNearestDays != 0 {
		t.Errorf("dueNearestDays = %v, want 0 — «Сегодня»", feed.DueNearestDays)
	}
}

// An empty window answers the zero count with the null nearest — the card
// hides its label.
func TestListGlobalPayments_DueCountersEmpty(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	svc := h.globalSvc()

	one := 1
	reminded := h.seedDueRule(h.propID, h.owner, "ЖКУ", &one, false, false)
	h.seedRuleOperation(reminded, h.propID, h.owner, "2026-09-15", opPlanned, "expense", "ЖКУ", 500000)

	feed, err := svc.ListGlobalPayments(h.ctx(), h.owner)
	if err != nil {
		t.Fatalf("list global payments: %v", err)
	}
	if feed.DueOperationsCount != 0 || feed.DueNearestDays != nil {
		t.Errorf("due counters = (%d, %v), want (0, nil) — outside the window", feed.DueOperationsCount, feed.DueNearestDays)
	}
}
