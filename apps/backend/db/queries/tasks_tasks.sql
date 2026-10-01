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
-- The bound rows' verdict is the derived-read canon — the SQL function
-- actor_can_read_property (000142, ADR 0028): the owner plus the active
-- members of a non-archived object, a suspended membership grants no read
-- (the SQL-level status filter, #158 T4). The owner term stays beside the
-- function call: the property-less rows are the owner's alone — the
-- function needs a property row to say yes.
--
-- The bound task rows always carry owner_id of the property's owner (there
-- is no re-binding), so the owner branch covers the actor's own properties.

-- name: CountTasksGlobal :one
-- The total of one bucket of the actor's visible merged feed. The tasks of
-- archived properties are not in the global feed (карта #518 решение 9,
-- тикет #522): non-archived properties plus the property-less cut.
SELECT count(*) FROM tasks t
LEFT JOIN properties p ON p.id = t.property_id
WHERE (t.owner_id = $1 OR actor_can_read_property(t.property_id, $1))
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
WHERE (t.owner_id = $1 OR actor_can_read_property(t.property_id, $1))
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
WHERE (t.owner_id = $1 OR actor_can_read_property(t.property_id, $1))
  AND (t.property_id IS NULL OR p.status != 'archived')
  AND t.completed_date IS NOT NULL
ORDER BY t.completed_date DESC, t.created_at DESC, t.id ASC
LIMIT $2 OFFSET $3;

-- The listed-properties cut of the global listing (ticket #547): the merged
-- feed restricted to the picker's selection — the visibility predicate is
-- the feed's, the list arrives comma-separated (uuids hold no commas).
-- Archived properties contribute nothing (карта #518, решение 9); the
-- without_property flag unions the actor's own property-less rows in — the
-- feed filter's «Общие задачи» + objects (решение владельца 2026-09-07).

-- name: CountTasksGlobalOfProperties :one
SELECT count(*) FROM tasks t
LEFT JOIN properties p ON p.id = t.property_id
WHERE (t.owner_id = $1 OR actor_can_read_property(t.property_id, $1))
  AND (
       t.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[])
       OR (sqlc.arg('without_property')::bool AND t.property_id IS NULL)
      )
  AND (t.property_id IS NULL OR p.status != 'archived')
  AND (t.completed_date IS NOT NULL) = $2::boolean;

-- name: ListActiveTasksGlobalOfProperties :many
-- The active tasks of the listed properties, the merged feed's visibility
-- and due order (ticket #547); without_property unions the actor's own
-- property-less rows in (решение владельца 2026-09-07).
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat,
       p.name AS property_name
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
LEFT JOIN properties p ON p.id = t.property_id
WHERE (t.owner_id = $1 OR actor_can_read_property(t.property_id, $1))
  AND (
       t.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[])
       OR (sqlc.arg('without_property')::bool AND t.property_id IS NULL)
      )
  AND (t.property_id IS NULL OR p.status != 'archived')
  AND t.completed_date IS NULL
ORDER BY t.due_date ASC NULLS LAST, t.due_time ASC NULLS FIRST, t.created_at ASC, t.id ASC
LIMIT $2 OFFSET $3;

-- name: ListCompletedTasksGlobalOfProperties :many
-- The completed journal of the listed properties, newest completions first
-- (ticket #547); without_property unions the actor's own property-less
-- journal in (решение владельца 2026-09-07).
SELECT t.id, t.owner_id, t.property_id, t.rule_id, t.due_date, t.due_time, t.title, t.comment, t.completed_date, t.created_at, t.updated_at,
       r.repeat AS rule_repeat,
       p.name AS property_name
FROM tasks t
LEFT JOIN task_rules r ON r.id = t.rule_id
LEFT JOIN properties p ON p.id = t.property_id
WHERE (t.owner_id = $1 OR actor_can_read_property(t.property_id, $1))
  AND (
       t.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[])
       OR (sqlc.arg('without_property')::bool AND t.property_id IS NULL)
      )
  AND (t.property_id IS NULL OR p.status != 'archived')
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

-- name: RaiseTaskHistoryHorizons :exec
-- The horizon leg of «Удалить все выполненные» (ADR 0051 as amended
-- 2026-10-01): every live rule of the property with completed rows about to
-- be cleared gets its history horizon raised to max(deleted due)+1 — the
-- cleared occurrences never materialize again. For an undated rule MAX(due)
-- is NULL: the CASE stamps the dormancy marker (the owner's today) whose
-- date part is meaningless. GREATEST keeps an earlier-raised horizon — the
-- deletion is forever. Runs inside the clear's transaction, before
-- DeleteCompletedJournal.
UPDATE task_rules r
SET history_before = CASE
      WHEN x.horizon IS NULL THEN COALESCE(r.history_before, sqlc.arg('today')::date)
      ELSE GREATEST(COALESCE(r.history_before, x.horizon), x.horizon)
    END
FROM (
  SELECT t.rule_id, MAX(t.due_date) + 1 AS horizon
  FROM tasks t
  WHERE t.owner_id = sqlc.arg('owner_id') AND t.property_id = sqlc.arg('property_id')
    AND t.completed_date IS NOT NULL
    AND t.rule_id IS NOT NULL
  GROUP BY t.rule_id
) x
WHERE r.id = x.rule_id AND r.owner_id = sqlc.arg('owner_id');

-- name: DeleteCompletedJournal :execrows
-- «Удалить все выполненные» (resolution #497, ADR 0051 as amended
-- 2026-10-01): every completed task of the property is removed forever — the
-- deleted-rule journal and the live rules' rows alike. The live rules'
-- horizons were raised by RaiseTaskHistoryHorizons in the same transaction:
-- the cleared dates never materialize again, the standing rows stand.
DELETE FROM tasks
WHERE owner_id = $1 AND property_id = $2
  AND completed_date IS NOT NULL;

-- name: RaiseTaskHistoryHorizonsOwnerBook :exec
-- The book-wide horizon leg: the owner's live rules with completed rows
-- about to be cleared — on non-archived properties and without one (ADR
-- 0052). The same max(deleted due)+1 raise; archived properties' rules stay
-- outside (their rows are not cleared — ADR 0025).
UPDATE task_rules r
SET history_before = CASE
      WHEN x.horizon IS NULL THEN COALESCE(r.history_before, sqlc.arg('today')::date)
      ELSE GREATEST(COALESCE(r.history_before, x.horizon), x.horizon)
    END
FROM (
  SELECT t.rule_id, MAX(t.due_date) + 1 AS horizon
  FROM tasks t
  WHERE t.owner_id = sqlc.arg('owner_id')
    AND t.completed_date IS NOT NULL
    AND t.rule_id IS NOT NULL
    AND (t.property_id IS NULL OR EXISTS (
          SELECT 1 FROM properties p
          WHERE p.id = t.property_id AND p.status != 'archived'
        ))
  GROUP BY t.rule_id
) x
WHERE r.id = x.rule_id AND r.owner_id = sqlc.arg('owner_id')
  AND (r.property_id IS NULL OR EXISTS (
        SELECT 1 FROM properties p
        WHERE p.id = r.property_id AND p.status != 'archived'
      ));

-- name: DeleteCompletedJournalOwnerBook :many
-- «Удалить все выполненные» across the owner's whole book (ticket #536):
-- every completed task of the book in one query — the bound rows and the
-- property-less ones (nullable property_id, ADR 0052), the deleted-rule
-- journals and the live rules' rows alike (the horizons were raised by
-- RaiseTaskHistoryHorizonsOwnerBook in the same transaction, ADR 0051 as
-- amended). Archived properties stay frozen (ADR 0025) and the shared-to
-- properties' journals are other owners' books (owner-scope, ADR 0028).
-- Returns the removed rows' property anchors, one per removed row (NULL is
-- the property-less leg) — the use case groups them into the per-object
-- journal rows (ADR 0061 §3).
DELETE FROM tasks t
WHERE t.owner_id = $1
  AND t.completed_date IS NOT NULL
  AND (t.property_id IS NULL OR EXISTS (
        SELECT 1 FROM properties p
        WHERE p.id = t.property_id AND p.status != 'archived'
      ))
RETURNING t.property_id;

-- name: ListRuleUncompletedTaskIDs :many
-- The scheduling seam's in-transaction handover (issue #775): the rule's
-- standing uncompleted tasks' ids, read after the materialization tick has
-- settled the rule's rows. The freshly materialized and the kept standing
-- tasks travel alike — notifications re-resolves each task's liveness and
-- term itself, so stale or kept ids are safe to hand over.
SELECT id
FROM tasks
WHERE rule_id = $1
  AND completed_date IS NULL
ORDER BY id;
