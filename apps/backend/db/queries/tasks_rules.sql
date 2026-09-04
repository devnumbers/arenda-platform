-- Tasks context queries: the task rule CRUD with the property serialization
-- lock and the rule's task invalidations (ADR 0051). The property lock
-- mirrors the payments precedent (ADR 0049 §3); the tick's persistence lives
-- in tasks_tick.sql, the task reads/completions in tasks_tasks.sql.

-- name: GetPropertyForTask :one
-- The tasks-scoped read of the property (ADR 0028): the data owner, the
-- archived flag and the display name (the global listing's row projection,
-- ticket #521), no lock.
SELECT id, owner_id, status, name FROM properties WHERE id = $1;

-- name: GetPropertyForTaskMutation :one
-- The mutation's serialization point: the property row locked inside the
-- caller's transaction, so mutation-vs-tick, mutation-vs-mutation and
-- archive-vs-mutation serialize on one point (ADR 0049 §3).
SELECT id, owner_id, status FROM properties WHERE id = $1 FOR UPDATE;

-- name: CreateTaskRule :exec
INSERT INTO task_rules (
    id, owner_id, property_id, title, comment, due_date, due_time, repeat
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: UpdateTaskRule :exec
-- The editable fields of the rule (the anchor included: edit invalidates the
-- not-yet-due uncompleted tasks, the in-transaction tick stands the single
-- future again); id/owner_id/property_id never move.
UPDATE task_rules
SET title = $2,
    comment = $3,
    due_date = $4,
    due_time = $5,
    repeat = $6
WHERE id = $1 AND owner_id = $7;

-- name: DeleteTaskRule :exec
-- The hard rule deletion (resolution #496): the completed journal keeps its
-- rows with rule_id set to NULL by the FK; the uncompleted tasks are
-- physically removed by DeleteRuleUncompleted before this runs.
DELETE FROM task_rules WHERE id = $1 AND owner_id = $2;

-- name: GetTaskRule :one
-- The nested path rule→property is part of the key: a foreign or re-hung
-- row is the privacy 404.
SELECT id, owner_id, property_id, title, comment, due_date, due_time, repeat, created_at, updated_at
FROM task_rules
WHERE id = $1 AND owner_id = $2 AND property_id = $3;

-- name: GetTaskRuleWithoutProperty :one
-- The property-less cut of the rule read (ADR 0052: the slices never mix —
-- the predicate over property_id picks the slice explicitly). The id-scoped
-- owner key is the privacy 404; a bound rule is invisible here by design.
SELECT id, owner_id, property_id, title, comment, due_date, due_time, repeat, created_at, updated_at
FROM task_rules
WHERE id = $1 AND owner_id = $2 AND property_id IS NULL;

-- name: DeleteRuleNotDueUncompleted :exec
-- The edit invalidation (resolution #496): the rule's uncompleted tasks that
-- have not fallen due yet — the undated one and the strictly future ones —
-- are removed; the in-transaction tick stands the single future again with
-- fresh snapshots. Already due and completed tasks keep their frozen
-- snapshots.
DELETE FROM tasks
WHERE rule_id = $1
  AND completed_date IS NULL
  AND (due_date IS NULL OR due_date > $2);

-- name: DeleteRuleUncompleted :exec
-- The rule deletion semantics (resolution #496): every uncompleted task of
-- the rule — planned, overdue, today's and the single future — is physically
-- removed; completed rows stay in the journal.
DELETE FROM tasks
WHERE rule_id = $1 AND completed_date IS NULL;
