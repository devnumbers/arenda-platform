package domain

import (
	"slices"
	"testing"
	"time"
)

// todaySep10 anchors every tick fixture: Thursday 2026-09-10.
var todaySep10 = d(2026, time.September, 10)

func TestPlanTaskTick_MaterializesDueAndSingleFuture(t *testing.T) {
	t.Parallel()

	rule := datedRule(RepeatWeekly, 2026, time.September, 3) // Thursdays: 09-03, 09-10, 09-17.

	plan := PlanTaskTick(rule, todaySep10, TaskExistence{})

	if len(plan.Materialize) != 2 || dateOnly(plan.Materialize[0]) != sep03 || dateOnly(plan.Materialize[1]) != sep10 {
		t.Fatalf("Materialize = %v, want [%s %s]", plan.Materialize, sep03, sep10)
	}
	if plan.InsertFuture == nil || dateOnly(*plan.InsertFuture) != sep17 {
		t.Fatalf("InsertFuture = %v, want %s", plan.InsertFuture, sep17)
	}
	if plan.KeepFuture == nil || !plan.KeepFuture.Equal(*plan.InsertFuture) {
		t.Fatalf("KeepFuture = %v, want equal to InsertFuture", plan.KeepFuture)
	}
	if plan.MaterializeUndated {
		t.Fatal("dated rule must not materialize undated.")
	}
}

func TestPlanTaskTick_TodayCountsAsDue(t *testing.T) {
	t.Parallel()

	rule := datedRule(RepeatOnce, 2026, time.September, 10)

	plan := PlanTaskTick(rule, todaySep10, TaskExistence{})

	if len(plan.Materialize) != 1 || dateOnly(plan.Materialize[0]) != sep10 {
		t.Fatalf("Materialize = %v, want [%s]", plan.Materialize, sep10)
	}
	if plan.InsertFuture != nil {
		t.Fatalf("once rule anchored today must have no future, got %v", plan.InsertFuture)
	}
}

func TestPlanTaskTick_IdempotentRerunIsNoop(t *testing.T) {
	t.Parallel()

	rule := datedRule(RepeatWeekly, 2026, time.September, 3)

	first := PlanTaskTick(rule, todaySep10, TaskExistence{})
	existing := TaskExistence{Dated: map[time.Time]bool{}}
	for _, day := range first.Materialize {
		existing.Dated[day] = false
	}
	if first.InsertFuture != nil {
		existing.Dated[*first.InsertFuture] = false
	}

	second := PlanTaskTick(rule, todaySep10, existing)
	if len(second.Materialize) != 0 || second.InsertFuture != nil || second.MaterializeUndated {
		t.Fatalf("rerun plan = %+v, want a no-op", second)
	}
	if second.KeepFuture == nil || dateOnly(*second.KeepFuture) != sep17 {
		t.Fatalf("KeepFuture = %v, want standing %s", second.KeepFuture, sep17)
	}
}

func TestPlanTaskTick_CompletedAheadFutureIsSkipped(t *testing.T) {
	t.Parallel()

	// The next Thursday 09-17 was completed ahead of its date: the standing
	// future moves to the occurrence after it.
	rule := datedRule(RepeatWeekly, 2026, time.September, 3)
	existing := TaskExistence{Dated: map[time.Time]bool{
		d(2026, time.September, 3):  true, // Completed (due in the past).
		d(2026, time.September, 10): false,
		d(2026, time.September, 17): true, // Completed ahead.
	}}

	plan := PlanTaskTick(rule, todaySep10, existing)

	if len(plan.Materialize) != 0 {
		t.Fatalf("Materialize = %v, want empty (rows exist)", plan.Materialize)
	}
	if plan.KeepFuture == nil || dateOnly(*plan.KeepFuture) != sep24 {
		t.Fatalf("KeepFuture = %v, want %s", plan.KeepFuture, sep24)
	}
	if plan.InsertFuture == nil || dateOnly(*plan.InsertFuture) != sep24 {
		t.Fatalf("InsertFuture = %v, want %s", plan.InsertFuture, sep24)
	}
}

func TestPlanTaskTick_UndatedRuleMaterializesImmediately(t *testing.T) {
	t.Parallel()

	rule := TaskRule{Repeat: RepeatOnce} // Undated.

	plan := PlanTaskTick(rule, todaySep10, TaskExistence{})

	if !plan.MaterializeUndated {
		t.Fatal("undated rule without a row must materialize undated.")
	}
	if len(plan.Materialize) != 0 || plan.InsertFuture != nil || plan.KeepFuture != nil {
		t.Fatalf("undated plan = %+v, want only MaterializeUndated", plan)
	}
}

func TestPlanTaskTick_UndatedRerunIsNoop(t *testing.T) {
	t.Parallel()

	rule := TaskRule{Repeat: RepeatOnce}

	plan := PlanTaskTick(rule, todaySep10, TaskExistence{Undated: true})
	if plan.MaterializeUndated {
		t.Fatal("existing undated row must not materialize again.")
	}
}

func TestPlanTaskTick_HorizonSuppressesDeletedPast(t *testing.T) {
	t.Parallel()

	// The «Удалить все выполненные» horizon: the completed 09-03 was cleared
	// and the rule's history horizon moved to 09-10 — the cleared 09-03 must
	// not re-materialize, while today's 09-10 (at the horizon itself) and the
	// single future stand as usual.
	rule := datedRule(RepeatWeekly, 2026, time.September, 3)
	horizon := d(2026, time.September, 10)
	rule.HistoryBefore = &horizon

	plan := PlanTaskTick(rule, todaySep10, TaskExistence{})

	if len(plan.Materialize) != 1 || dateOnly(plan.Materialize[0]) != sep10 {
		t.Fatalf("Materialize = %v, want [%s] (the cleared 09-03 stays deleted)", plan.Materialize, sep10)
	}
	if plan.InsertFuture == nil || dateOnly(*plan.InsertFuture) != sep17 {
		t.Fatalf("InsertFuture = %v, want %s", plan.InsertFuture, sep17)
	}
}

func TestPlanTaskTick_HorizonKeepsStandingFutureOnSuppressedDate(t *testing.T) {
	t.Parallel()

	// A completed-ahead 09-24 was cleared (horizon 10-01) while the standing
	// 09-17 stays uncompleted: the horizon suppresses creation, never the
	// existing rows — the standing row still occupies its suppressed gap, and
	// the walk resumes after it.
	rule := datedRule(RepeatWeekly, 2026, time.September, 3)
	horizon := d(2026, time.October, 1)
	rule.HistoryBefore = &horizon
	existing := TaskExistence{Dated: map[time.Time]bool{
		d(2026, time.September, 17): false, // The standing future task.
	}}

	plan := PlanTaskTick(rule, todaySep10, existing)

	if len(plan.Materialize) != 0 {
		t.Fatalf("Materialize = %v, want empty", plan.Materialize)
	}
	if plan.KeepFuture == nil || dateOnly(*plan.KeepFuture) != sep17 {
		t.Fatalf("KeepFuture = %v, want the standing %s", plan.KeepFuture, sep17)
	}
	if plan.InsertFuture != nil {
		t.Fatalf("InsertFuture = %v, want nil (the standing row occupies 09-17)", plan.InsertFuture)
	}
}

func TestPlanTaskTick_HorizonSuppressesClearedFutureInsertion(t *testing.T) {
	t.Parallel()

	// The completed-ahead 09-17 was cleared (horizon 09-24) and no rows are
	// left: the suppressed 09-17 must not re-materialize — neither as due
	// debt nor as the single future; the next non-suppressed occurrence stands.
	rule := datedRule(RepeatWeekly, 2026, time.September, 3)
	horizon := d(2026, time.September, 24)
	rule.HistoryBefore = &horizon

	plan := PlanTaskTick(rule, todaySep10, TaskExistence{})

	if len(plan.Materialize) != 0 {
		t.Fatalf("Materialize = %v, want empty (09-03..09-10 are suppressed)", plan.Materialize)
	}
	if plan.KeepFuture == nil || dateOnly(*plan.KeepFuture) != sep24 {
		t.Fatalf("KeepFuture = %v, want %s (the next non-suppressed occurrence)", plan.KeepFuture, sep24)
	}
	if plan.InsertFuture == nil || dateOnly(*plan.InsertFuture) != sep24 {
		t.Fatalf("InsertFuture = %v, want %s", plan.InsertFuture, sep24)
	}
}

func TestPlanTaskTick_HorizonKeepsWeekdayPhase(t *testing.T) {
	t.Parallel()

	// The horizon lands right after a cleared occurrence (max deleted due
	// 09-10 + 1 = Friday 09-11) while the rule runs on Thursdays (anchor
	// 09-03): the suppressed window must filter occurrences, never restart
	// the weekly phase from the horizon — the due materialization stays on
	// Thursdays (09-17, 09-24), no Fridays appear.
	rule := datedRule(RepeatWeekly, 2026, time.September, 3)
	horizon := d(2026, time.September, 11)
	rule.HistoryBefore = &horizon

	plan := PlanTaskTick(rule, d(2026, time.September, 24), TaskExistence{})

	got := make([]string, len(plan.Materialize))
	for i, day := range plan.Materialize {
		got[i] = dateOnly(day)
	}
	want := []string{sep17, sep24}
	if !slices.Equal(got, want) {
		t.Fatalf("Materialize = %v, want %v (Thursday phase kept)", got, want)
	}
}

func TestPlanTaskTick_UndatedHorizonStaysDormant(t *testing.T) {
	t.Parallel()

	// An undated rule whose cleared task is gone stays dormant: the horizon
	// marker (its date part is meaningless here) suppresses the undated
	// materialization until a rule edit resets it.
	rule := TaskRule{Repeat: RepeatOnce, HistoryBefore: &todaySep10}

	plan := PlanTaskTick(rule, todaySep10, TaskExistence{})

	if plan.MaterializeUndated {
		t.Fatal("cleared undated rule must stay dormant (no task rows left).")
	}
}

func TestPlanTaskTick_OncePastAnchorHasNoFuture(t *testing.T) {
	t.Parallel()

	rule := datedRule(RepeatOnce, 2026, time.September, 3)

	plan := PlanTaskTick(rule, todaySep10, TaskExistence{Dated: map[time.Time]bool{d(2026, time.September, 3): false}})

	if len(plan.Materialize) != 0 || plan.InsertFuture != nil || plan.KeepFuture != nil {
		t.Fatalf("spent once plan = %+v, want empty", plan)
	}
}
