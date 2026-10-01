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
// ahead of today, push the single future to the next occurrence). The rule's
// history horizon (rule.HistoryBefore — the «Удалить все выполненные» mark)
// suppresses creation of the cleared occurrences: dates earlier than the
// horizon neither materialize as due debt nor insert as the single future,
// while existing rows — a standing future on a suppressed date included —
// are never the horizon's business.
func PlanTaskTick(rule TaskRule, today time.Time, existing TaskExistence) TaskTickPlan {
	var plan TaskTickPlan

	// 1. The undated rule: its single task materializes immediately; there is
	// no schedule and no future. A raised horizon (the cleared-task marker)
	// keeps the rule dormant — nothing left to respawn until an edit resets
	// the mark.
	if rule.Undated() {
		plan.MaterializeUndated = !existing.Undated && rule.HistoryBefore == nil
		return plan
	}

	// Step 2 materializes everything due; step 3 rebuilds the single future.
	plan.Materialize = dueOccurrences(rule, today, existing)
	planSingleFuture(rule, today, existing, &plan)
	return plan
}

// dueOccurrences lists the due occurrence dates (≤ today) the tick must
// create: those with no task yet, enumerated from max(anchor, history
// horizon) — the cleared earlier dates are gone forever.
func dueOccurrences(rule TaskRule, today time.Time, existing TaskExistence) []time.Time {
	from := *rule.DueDate
	if rule.HistoryBefore != nil && rule.HistoryBefore.After(from) {
		from = *rule.HistoryBefore
	}
	var due []time.Time
	for _, day := range OccurrencesBetween(rule, from, today) {
		if _, ok := existing.Dated[day]; !ok {
			due = append(due, day)
		}
	}
	return due
}

// planSingleFuture rebuilds the plan's single future: the first occurrence
// after today without a task (to insert) or with an uncompleted one (already
// standing). Completed-ahead occurrences are skipped — the task already
// happened, look at the next. Suppressed occurrences (earlier than the
// history horizon) are skipped too, unless a standing uncompleted row
// occupies them — the horizon never unseats an existing row.
func planSingleFuture(rule TaskRule, today time.Time, existing TaskExistence, plan *TaskTickPlan) {
	horizon := nextDay(addYearsClamped(today, 5))
	for _, day := range OccurrencesBetween(rule, today, horizon) {
		if !day.After(today) {
			continue
		}
		completed, exists := existing.Dated[day]
		if rule.HistoryBefore != nil && day.Before(*rule.HistoryBefore) {
			if exists && !completed {
				plan.KeepFuture = &day
				break
			}
			continue
		}
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
}
