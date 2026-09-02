package domain

import "time"

// TaskExistence is the tick's dedup input for one rule: which of its tasks
// already stand. A row of any status — active or completed — occupies its
// occurrence: a completed task keeps the dedup key so the tick never
// re-materializes the date (the tasks analogue of the payments tombstone is
// simply the completed row itself).
type TaskExistence struct {
	// Dated keys the rule's dated tasks by due date; the value is the row's
	// completed flag — completed-ahead future rows are skipped by the
	// single-future walk, exactly like the payments future-planned rebuild.
	Dated map[time.Time]bool
	// Undated reports whether the rule's single undated task row exists.
	Undated bool
}

// TaskTickPlan is what the tick must execute for one rule at a given today —
// a pure computation over the rule, today and the existing task keys
// (resolution #496). Applying it is idempotent.
type TaskTickPlan struct {
	// Materialize lists due occurrence dates (≤ today) that have no task yet;
	// each becomes a planned (active) task. Missed days materialize on the
	// next run — overdue debt of the to-do world.
	Materialize []time.Time
	// MaterializeUndated inserts the undated task of an undated rule when no
	// row stands; the undated task materializes at rule creation time.
	MaterializeUndated bool
	// KeepFuture is the only future (date > today) active task allowed to
	// remain; every other future uncompleted task of the rule is removed
	// (stale rows left by rule edits). Nil means no future task may remain —
	// for an undated rule that is the steady state.
	KeepFuture *time.Time
	// InsertFuture is the missing future task to insert; when set, it is
	// always equal to KeepFuture.
	InsertFuture *time.Time
}

// PlanTaskTick computes the tick plan for one rule. Completed tasks are never
// touched by the plan: completed rows only occupy their dates (and, dated
// ahead of today, push the single future to the next occurrence).
func PlanTaskTick(rule TaskRule, today time.Time, existing TaskExistence) TaskTickPlan {
	var plan TaskTickPlan

	// 1. The undated rule: its single task materializes immediately; there is
	// no schedule and no future.
	if rule.Undated() {
		plan.MaterializeUndated = !existing.Undated
		return plan
	}

	// 2. Materialize everything due: occurrences ≤ today with no task yet.
	for _, day := range OccurrencesBetween(rule, *rule.DueDate, today) {
		if _, ok := existing.Dated[day]; !ok {
			plan.Materialize = append(plan.Materialize, day)
		}
	}

	// 3. Rebuild the single future: the first occurrence after today without
	// a task (to insert) or with an uncompleted one (already standing).
	// Completed-ahead occurrences are skipped — the task already happened,
	// look at the next.
	horizon := nextDay(addYearsClamped(today, 5))
	for _, day := range OccurrencesBetween(rule, today, horizon) {
		if !day.After(today) {
			continue
		}
		completed, exists := existing.Dated[day]
		if !exists {
			plan.KeepFuture = &day
			plan.InsertFuture = &day
		} else if !completed {
			plan.KeepFuture = &day
		}
		if plan.KeepFuture != nil {
			break
		}
	}
	return plan
}
