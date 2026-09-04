-- Tasks context queries: the task reads, the completion toggle and the
-- «Удалить все выполненные» journal clear (ADR 0051, resolutions #496/#497).
-- Rule CRUD lives in tasks_rules.sql, the tick's persistence in
-- tasks_tick.sql.

-- name: GetTask :one
-- The nested path task→property is part of the key: a foreign or re-hung
-- row is the privacy 404. rule_repeat is the live rule's repeat read through the
-- LEFT JOIN (null once the rule is deleted) — the wire's ↻ mark; the task
-- row itself carries no repeat snapshot.
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
WHERE t.id = $1 AND t.owner_id = $2 AND t.property_id = $3;

-- name: GetTaskWithoutProperty :one
-- The property-less cut of the task read (ADR 0052: the slices never mix).
-- rule_repeat is the same live-rule read projection as in GetTask.
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
WHERE t.id = $1 AND t.owner_id = $2 AND t.property_id IS NULL;

-- name: CountTasksByProperty :one
-- The total count of one bucket — the «Выполненные N» counter (false =
-- active tasks, true = the completed journal).
SELECT count(*) FROM tasks
WHERE owner_id = $1 AND property_id = $2
  AND (completed_date IS NOT NULL) = $3::boolean;

-- name: ListActiveTasksByProperty :many
-- The active tasks (uncompleted) of the property: the screen's main
-- sections. Due order with the undated last — the client buckets sections
-- against the owner's today delivered by the response.
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
WHERE t.owner_id = $1 AND t.property_id = $2 AND t.completed_date IS NULL
ORDER BY t.due_date ASC NULLS LAST, t.due_time ASC NULLS FIRST, t.created_at ASC, t.id ASC
LIMIT $3 OFFSET $4;

-- name: ListCompletedTasksByProperty :many
-- The completed journal of the property, newest completions first.
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
WHERE t.owner_id = $1 AND t.property_id = $2 AND t.completed_date IS NOT NULL
ORDER BY t.completed_date DESC, t.created_at DESC, t.id ASC
LIMIT $3 OFFSET $4;

-- name: CompleteTask :execrows
-- «Выполнить»: the completion fact stamped on the still-active task; rows
-- affected = 0 surfaces as ErrAlreadyCompleted (the use case has already
-- proven existence under the property lock).
UPDATE tasks
SET completed_date = $2
WHERE id = $1 AND owner_id = $3 AND completed_date IS NULL;

-- name: UncompleteTask :execrows
-- «Отменить выполнение»: the completion fact cleared; rows affected = 0
-- surfaces as ErrNotCompleted.
UPDATE tasks
SET completed_date = NULL
WHERE id = $1 AND owner_id = $2 AND completed_date IS NOT NULL;

-- name: DeleteCompletedJournal :execrows
-- «Удалить все выполненные» (resolution #497): the completed tasks of the
-- property's deleted rules (rule_id IS NULL) are removed forever. The
-- completed tasks of live rules stay — they hold the tick's dedup keys, and
-- clearing them would re-materialize the rule's whole past (ADR 0051).
DELETE FROM tasks
WHERE owner_id = $1 AND property_id = $2
  AND completed_date IS NOT NULL
  AND rule_id IS NULL;
