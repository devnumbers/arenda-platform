//go:build integration

package application_test

// The materialization tick tests run on the shared payments harness
// (integration_harness_test.go): pre-seeded rules driven through the
// worker-path TickService.

import (
	"testing"

	"github.com/google/uuid"
)

func TestTick_MaterializesDueAndSingleFutureThenRerunIsNoop(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day22, `{"kind":"daily"}`, false)

	h.runTick()

	ops := h.operationsOf(paymentID)
	// Daily since T-3 (today is the 25th in Moscow): 4 due (3 overdue + today)
	// + exactly one future planned (prototype smoke "догон по старому since").
	want := map[string]string{
		day22:        opPlanned,
		day23:        opPlanned,
		"2026-08-24": opPlanned,
		day25:        opPlanned,
		day26:        opPlanned,
	}
	if len(ops) != len(want) {
		t.Fatalf("operations count = %d, want %d: %+v", len(ops), len(want), ops)
	}
	for date, status := range want {
		if got := statusesOf(ops)[date]; got != status {
			t.Fatalf("operation %s status = %q, want %q", date, got, status)
		}
	}

	// The rerun is a full no-op: same rows, same timestamps — the dedup index
	// plus the idempotent plan converge without writing.
	before := h.operationsOf(paymentID)
	h.runTick()
	after := h.operationsOf(paymentID)
	if len(before) != len(after) {
		t.Fatalf("rerun changed row count: %d → %d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("rerun rewrote row %+v → %+v", before[i], after[i])
		}
	}
}

func TestTick_AutoPayClosesOnlyToday(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day23, `{"kind":"daily"}`, true)

	h.runTick()

	ops := statusesOf(h.operationsOf(paymentID))
	if got := ops[day23]; got != opPlanned {
		t.Fatalf("the day before yesterday-equivalent (2026-08-23) = %q, want planned — no backdated catch-up (ADR 0049 §2)", got)
	}
	if got := ops["2026-08-24"]; got != opPlanned {
		t.Fatalf("2026-08-24 = %q, want planned — yesterday remains debt", got)
	}
	if got := ops[day25]; got != opPaid {
		t.Fatalf("today = %q, want paid — the auto-pay closes exactly the current day", got)
	}
	if got := ops[day26]; got != opPlanned {
		t.Fatalf("tomorrow = %q, want planned — exactly one future planned", got)
	}
	for _, op := range h.operationsOf(paymentID) {
		if op.status == opPaid {
			if op.paidDate == nil || *op.paidDate != day25 {
				t.Fatalf("paid operation %s has paid_date %v, want 2026-08-25", op.date, op.paidDate)
			}
		}
	}
}

func TestTick_WorkerMissedTheDayLeavesDebt(t *testing.T) {
	t.Parallel()
	// The tick did not run on the 24th (clock jumped from the 23rd straight
	// to the 25th): the missed occurrence stays planned debt even though the
	// rule is an auto-pay.
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment("2026-08-20", `{"kind":"daily"}`, true)

	h.runTick()

	ops := statusesOf(h.operationsOf(paymentID))
	for _, missed := range []string{"2026-08-20", "2026-08-21", day22, day23, "2026-08-24"} {
		if got := ops[missed]; got != opPlanned {
			t.Fatalf("missed day %s = %q, want planned — a worker outage leaves honest debt", missed, got)
		}
	}
	if got := ops[day25]; got != opPaid {
		t.Fatalf("today = %q, want paid", got)
	}
}

func TestTick_ActivePauseExcludesOccurrences(t *testing.T) {
	t.Parallel()
	// Daily since T-3, paused from T-1 on: occurrences before the pause stay,
	// nothing generates during the pause, no future planned remains, and the
	// debt accumulated before the pause is untouched.
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day22, `{"kind":"daily"}`, true)
	h.seedPause(paymentID, "2026-08-24", nil)

	h.runTick()

	ops := statusesOf(h.operationsOf(paymentID))
	want := map[string]string{
		day22: opPlanned,
		day23: opPlanned,
	}
	if len(ops) != len(want) {
		t.Fatalf("operations = %+v, want exactly %+v (pause cuts the rest)", ops, want)
	}
	for date, status := range want {
		if ops[date] != status {
			t.Fatalf("operation %s = %q, want %q", date, ops[date], status)
		}
	}

	// Days advance inside the pause: still nothing new, no debt accrues.
	h.clock.now = tickBaseTime.AddDate(0, 0, 5)
	h.runTick()
	if got := len(h.operationsOf(paymentID)); got != len(want) {
		t.Fatalf("operations after 5 paused days = %d, want %d", got, len(want))
	}
}

func TestTick_PausedAutoPayDoesNotCloseToday(t *testing.T) {
	t.Parallel()
	// Yesterday's tick stood the future planned on today; today the rule is
	// paused before the tick runs: the standing occurrence stays planned
	// (future debt), the pause stops the auto-pay day payment and removes the
	// future planned.
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day23, `{"kind":"daily"}`, true)

	h.clock.now = tickBaseTime.AddDate(0, 0, -1)
	h.runTick()

	if _, err := h.pool.Exec(h.ctx(),
		`INSERT INTO payment_pauses (id, payment_id, from_date) VALUES ($1, $2, '2026-08-25')`,
		uuid.Must(uuid.NewV7()), paymentID,
	); err != nil {
		t.Fatalf("seed pause: %v", err)
	}

	h.clock.now = tickBaseTime
	h.runTick()

	ops := statusesOf(h.operationsOf(paymentID))
	if got := ops[day25]; got != opPlanned {
		t.Fatalf("today = %q, want planned — an active pause stops the auto-pay day payment", got)
	}
	if got := ops[day26]; got != "" {
		t.Fatalf("2026-08-26 = %q, want absent — the pause removes the future planned", got)
	}
}

func TestTick_TodayFollowsOwnerTimezone(t *testing.T) {
	t.Parallel()
	// Same instant (2026-08-25 20:00 UTC), two owners in different zones:
	// Kamchatka is already on the 26th, Moscow still on the 25th. Each
	// owner's materialization follows their own calendar date (ADR 0048).
	kamchatka := newPaymentsHarness(t).withOwner("Asia/Kamchatka")
	kPayment := kamchatka.seedPayment("2026-08-20", `{"kind":"daily"}`, false)

	moscow := newPaymentsHarness(t).withOwner("Europe/Moscow")
	mPayment := moscow.seedPayment("2026-08-20", `{"kind":"daily"}`, false)

	kamchatka.runTick()
	moscow.runTick()

	kOps := statusesOf(kamchatka.operationsOf(kPayment))
	if got := kOps[day26]; got != opPlanned {
		t.Fatalf("Kamchatka today (2026-08-26) = %q, want planned — the 26th is already due in UTC+12", got)
	}
	if got := kOps["2026-08-27"]; got != opPlanned {
		t.Fatalf("Kamchatka future planned (2026-08-27) = %q, want planned", got)
	}

	mOps := statusesOf(moscow.operationsOf(mPayment))
	if got := mOps[day25]; got != opPlanned {
		t.Fatalf("Moscow today (2026-08-25) = %q, want planned", got)
	}
	if got := mOps[day26]; got != opPlanned {
		t.Fatalf("Moscow future planned (2026-08-26) = %q, want planned", got)
	}
	if got := mOps["2026-08-27"]; got != "" {
		t.Fatalf("Moscow 2026-08-27 = %q, want absent — Moscow is still on the 25th", got)
	}
}

func TestTick_RebuildsFuturePlannedAfterEarlyPayment(t *testing.T) {
	t.Parallel()
	// Monthly on the 10th; today is the 25th, so the future planned stands on
	// Sep 10. "Оплатить сейчас" (simulated directly) closes it ahead of time:
	// the next tick must stand Oct 10 as the new single future planned without
	// touching the paid fact.
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment("2026-01-10", `{"kind":"monthly","dayOfMonth":10}`, false)

	h.runTick()

	if _, err := h.pool.Exec(h.ctx(), `
		UPDATE operations SET status = 'paid', paid_date = '2026-08-25'
		WHERE payment_id = $1 AND date = '2026-09-10'`, paymentID,
	); err != nil {
		t.Fatalf("pay ahead: %v", err)
	}

	h.runTick()

	ops := statusesOf(h.operationsOf(paymentID))
	if got := ops["2026-09-10"]; got != opPaid {
		t.Fatalf("the paid-ahead occurrence = %q, want paid — facts are untouchable", got)
	}
	if got := ops["2026-10-10"]; got != opPlanned {
		t.Fatalf("the occurrence after the paid one = %q, want planned — the schedule does not shift", got)
	}
	if got := ops["2026-11-10"]; got != "" {
		t.Fatalf("2026-11-10 = %q, want absent — exactly one future planned", got)
	}
}

func TestTick_SkipsArchivedProperty(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day22, `{"kind":"daily"}`, true)

	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE properties SET status = 'archived' WHERE id = $1`, h.propID,
	); err != nil {
		t.Fatalf("archive property: %v", err)
	}

	h.runTick()

	if ops := h.operationsOf(paymentID); len(ops) != 0 {
		t.Fatalf("archived property got %d operations, want 0 — the tick skips archived properties", len(ops))
	}
}

func TestTick_CategorySnapshotFromCatalog(t *testing.T) {
	t.Parallel()
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day25, `{"kind":"monthly","dayOfMonth":25}`, false)

	h.runTick()

	var label string
	if err := h.pool.QueryRow(h.ctx(),
		`SELECT category_label FROM operations WHERE payment_id = $1 AND date = '2026-08-25'`,
		paymentID,
	).Scan(&label); err != nil {
		t.Fatalf("read snapshot label: %v", err)
	}
	if label != "Коммунальные услуги" {
		t.Fatalf("category label = %q, want the catalog label for slug utilities", label)
	}
}

func TestTickZoneSweep_MaterializesEveryZoneOnItsOwnToday(t *testing.T) {
	t.Parallel()
	// The worker's hourly sweep (ADR 0048 p.3, ticket #458) over one
	// database: the same instant is already the 26th in Kamchatka and still
	// the 25th in Moscow, and every zone's owners materialize on their own
	// zone's date in their own unit of work.
	h := newPaymentsHarness(t)
	kamchatka := h.withOwner("Asia/Kamchatka")
	kPayment := kamchatka.seedPayment("2026-08-20", `{"kind":"daily"}`, false)
	moscow := h.withOwner("Europe/Moscow")
	mPayment := moscow.seedPayment("2026-08-20", `{"kind":"daily"}`, false)

	if err := h.tick.RunZoneTicks(h.ctx(), h.clock.now); err != nil {
		t.Fatalf("run zone sweep: %v", err)
	}

	kOps := statusesOf(h.operationsOf(kPayment))
	if got := kOps[day26]; got != opPlanned {
		t.Fatalf("Kamchatka today (2026-08-26) = %q, want planned — the sweep's today follows the zone", got)
	}
	if got := kOps["2026-08-27"]; got != opPlanned {
		t.Fatalf("Kamchatka future planned (2026-08-27) = %q, want planned", got)
	}
	if len(kOps) != 8 {
		t.Fatalf("Kamchatka operations = %d, want 8 (due 20th–26th + one future)", len(kOps))
	}

	mOps := statusesOf(h.operationsOf(mPayment))
	if got := mOps[day25]; got != opPlanned {
		t.Fatalf("Moscow today (2026-08-25) = %q, want planned", got)
	}
	if got := mOps[day26]; got != opPlanned {
		t.Fatalf("Moscow future planned (2026-08-26) = %q, want planned", got)
	}
	if got := mOps["2026-08-27"]; got != "" {
		t.Fatalf("Moscow 2026-08-27 = %q, want absent — Moscow is still on the 25th", got)
	}

	// The rerun between the zones' midnights is a full no-op (ADR 0048 p.3):
	// same rows, same timestamps — the dedup key converges without writing.
	before := append([]opRow(nil), h.operationsOf(kPayment)...)
	before = append(before, h.operationsOf(mPayment)...)
	if err := h.tick.RunZoneTicks(h.ctx(), h.clock.now); err != nil {
		t.Fatalf("rerun zone sweep: %v", err)
	}
	after := append([]opRow(nil), h.operationsOf(kPayment)...)
	after = append(after, h.operationsOf(mPayment)...)
	if len(before) != len(after) {
		t.Fatalf("rerun changed row count: %d → %d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("rerun rewrote row %+v → %+v", before[i], after[i])
		}
	}
}

func TestTickZoneSweep_ArchivedPropertyOwnerIsNotATarget(t *testing.T) {
	t.Parallel()
	// The zone listing follows the tick's own scope (ADR 0049 §3): an owner
	// whose only rules hang on an archived property is not a sweep target at
	// all — no unit of work is opened for the zone-less owner.
	h := newPaymentsHarness(t).withOwner("Europe/Moscow")
	paymentID := h.seedPayment(day22, `{"kind":"daily"}`, false)
	if _, err := h.pool.Exec(h.ctx(),
		`UPDATE properties SET status = 'archived' WHERE id = $1`, h.propID,
	); err != nil {
		t.Fatalf("archive property: %v", err)
	}

	if err := h.tick.RunZoneTicks(h.ctx(), h.clock.now); err != nil {
		t.Fatalf("run zone sweep: %v", err)
	}

	if ops := h.operationsOf(paymentID); len(ops) != 0 {
		t.Fatalf("archived property got %d operations, want 0 — its owner is not a sweep target", len(ops))
	}
}

func TestTickZoneSweep_NoZonesIsSuccessfulNoop(t *testing.T) {
	t.Parallel()
	// An empty zone list is a successful no-op run, not an error: the
	// heartbeat of a healthy-but-empty platform still advances (ADR 0048
	// p.3 — the alert watches a silent worker, not an empty one).
	h := newPaymentsHarness(t)

	if err := h.tick.RunZoneTicks(h.ctx(), h.clock.now); err != nil {
		t.Fatalf("run zone sweep over no zones: %v", err)
	}
}
