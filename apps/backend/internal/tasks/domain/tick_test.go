package domain

import (
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

func TestPlanTaskTick_OncePastAnchorHasNoFuture(t *testing.T) {
	t.Parallel()

	rule := datedRule(RepeatOnce, 2026, time.September, 3)

	plan := PlanTaskTick(rule, todaySep10, TaskExistence{Dated: map[time.Time]bool{d(2026, time.September, 3): false}})

	if len(plan.Materialize) != 0 || plan.InsertFuture != nil || plan.KeepFuture != nil {
		t.Fatalf("spent once plan = %+v, want empty", plan)
	}
}
