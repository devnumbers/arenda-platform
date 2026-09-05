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

-- The global listings' visibility predicate (ticket #521): the actor's own
-- rows — bound and property-less — plus the bound rows of the properties
-- they share with an active membership (ADR 0028 read scope with the merged
-- visibility of ADR 0052 decision 3). The policy port maps an actor to a
-- role per property; the merged feed has no single scope to resolve, so the
-- actor-scoped read carries its visibility predicate here, beside the data.
-- A suspended membership grants no read (the SQL-level status filter, the
-- property_members precedent); the property-less rows are the owner's alone
-- — the member EXISTS needs a property to match.
--
-- The bound task rows always carry owner_id of the property's owner (there
-- is no re-binding), so the owner branch covers the actor's own properties.

-- name: CountTasksGlobal :one
-- The total of one bucket of the actor's visible merged feed. The tasks of
-- archived properties are not in the global feed (карта #518 решение 9,
-- тикет #522): non-archived properties plus the property-less cut.
SELECT count(*) FROM tasks t
LEFT JOIN properties p ON p.id = t.property_id
WHERE (
       t.owner_id = $1
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = t.property_id
              AND pm.user_id = $1
              AND pm.status = 'active'
          )
      )
  AND (t.property_id IS NULL OR p.status != 'archived')
  AND (t.completed_date IS NOT NULL) = $2::boolean;

-- name: ListActiveTasksGlobal :many
-- The active tasks of the actor's visible merged feed, due order with the
-- undated last — the global screen's sections (ticket #521). property_name
-- is the row's property label; the actor's today travels in the response.
-- Archived properties are out of the feed (карта #518 решение 9): the
-- sections stay contiguous for the client's pagination and bucketing.
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat,
       p.name AS property_name
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
LEFT JOIN properties p ON p.id = t.property_id
WHERE (
       t.owner_id = $1
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = t.property_id
              AND pm.user_id = $1
              AND pm.status = 'active'
          )
      )
  AND (t.property_id IS NULL OR p.status != 'archived')
  AND t.completed_date IS NULL
ORDER BY t.due_date ASC NULLS LAST, t.due_time ASC NULLS FIRST, t.created_at ASC, t.id ASC
LIMIT $2 OFFSET $3;

-- name: ListCompletedTasksGlobal :many
-- The completed journal of the actor's visible merged feed, newest
-- completions first (ticket #521); archived properties are out (решение 9).
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat,
       p.name AS property_name
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
LEFT JOIN properties p ON p.id = t.property_id
WHERE (
       t.owner_id = $1
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = t.property_id
              AND pm.user_id = $1
              AND pm.status = 'active'
          )
      )
  AND (t.property_id IS NULL OR p.status != 'archived')
  AND t.completed_date IS NOT NULL
ORDER BY t.completed_date DESC, t.created_at DESC, t.id ASC
LIMIT $2 OFFSET $3;

-- The listed-properties cut of the global listing (ticket #547): the merged
-- feed restricted to the picker's selection — the visibility predicate is
-- the feed's, the list arrives comma-separated (uuids hold no commas).
-- Archived properties contribute nothing (карта #518, решение 9); the
-- property-less slice is not reachable through this filter.

-- name: CountTasksGlobalOfProperties :one
SELECT count(*) FROM tasks t
LEFT JOIN properties p ON p.id = t.property_id
WHERE (
       t.owner_id = $1
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = t.property_id
              AND pm.user_id = $1
              AND pm.status = 'active'
          )
      )
  AND t.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[])
  AND p.status != 'archived'
  AND (t.completed_date IS NOT NULL) = $2::boolean;

-- name: ListActiveTasksGlobalOfProperties :many
-- The active tasks of the listed properties, the merged feed's visibility
-- and due order (ticket #547).
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat,
       p.name AS property_name
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
LEFT JOIN properties p ON p.id = t.property_id
WHERE (
       t.owner_id = $1
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = t.property_id
              AND pm.user_id = $1
              AND pm.status = 'active'
          )
      )
  AND t.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[])
  AND p.status != 'archived'
  AND t.completed_date IS NULL
ORDER BY t.due_date ASC NULLS LAST, t.due_time ASC NULLS FIRST, t.created_at ASC, t.id ASC
LIMIT $2 OFFSET $3;

-- name: ListCompletedTasksGlobalOfProperties :many
-- The completed journal of the listed properties, newest completions first
-- (ticket #547).
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat,
       p.name AS property_name
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
LEFT JOIN properties p ON p.id = t.property_id
WHERE (
       t.owner_id = $1
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = t.property_id
              AND pm.user_id = $1
              AND pm.status = 'active'
          )
      )
  AND t.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[])
  AND p.status != 'archived'
  AND t.completed_date IS NOT NULL
ORDER BY t.completed_date DESC, t.created_at DESC, t.id ASC
LIMIT $2 OFFSET $3;

-- The property-less cut of the global listing (ADR 0052: the actor's own
-- book only). No property join — the label is always absent there.

-- name: CountTasksGlobalWithoutProperty :one
SELECT count(*) FROM tasks
WHERE owner_id = $1 AND property_id IS NULL
  AND (completed_date IS NOT NULL) = $2::boolean;

-- name: ListActiveTasksGlobalWithoutProperty :many
-- The active property-less tasks of the actor's book, the same due order as
-- the property listings (ticket #521).
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
WHERE t.owner_id = $1 AND t.property_id IS NULL AND t.completed_date IS NULL
ORDER BY t.due_date ASC NULLS LAST, t.due_time ASC NULLS FIRST, t.created_at ASC, t.id ASC
LIMIT $2 OFFSET $3;

-- name: ListCompletedTasksGlobalWithoutProperty :many
-- The completed journal of the actor's property-less tasks, newest
-- completions first (ticket #521).
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
WHERE t.owner_id = $1 AND t.property_id IS NULL AND t.completed_date IS NOT NULL
ORDER BY t.completed_date DESC, t.created_at DESC, t.id ASC
LIMIT $2 OFFSET $3;

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

-- name: DeleteCompletedJournalOwnerBook :execrows
-- «Удалить все выполненные» across the owner's whole book (ticket #536):
-- the completed tasks of the deleted rules (rule_id IS NULL) in one query —
-- the bound rows and the property-less ones (nullable property_id, ADR 0052).
-- Archived properties stay frozen (ADR 0025) and the shared-to properties'
-- journals are other owners' books (owner-scope, ADR 0028). The completed
-- tasks of live rules stay — the tick's dedup keys (ADR 0051).
DELETE FROM tasks t
WHERE t.owner_id = $1
  AND t.completed_date IS NOT NULL
  AND t.rule_id IS NULL
  AND (t.property_id IS NULL OR EXISTS (
        SELECT 1 FROM properties p
        WHERE p.id = t.property_id AND p.status != 'archived'
      ));
