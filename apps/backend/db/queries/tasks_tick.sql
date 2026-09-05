-- Tasks context queries: the materialization tick's persistence (ADR 0051,
-- mirroring ADR 0049 §3). The owner-level rule listing, the task dedup keys,
-- the idempotent inserts and the future rebuild. Rule CRUD lives in
-- tasks_rules.sql, task reads/completions in tasks_tasks.sql.

-- name: LockTaskOwnerProperties :many
-- Serialization point of the tick: the run locks the owner's
-- active/maintenance property rows, ordered by id, before reading or writing
-- anything. Context mutations take the same ordered set at their
-- transaction's front (#546 — the deadlock-free global order), so
-- update-vs-tick, archive-vs-tick, delete-vs-tick and mutation-vs-mutation
-- serialize on one lock order. Archived properties are skipped by the tick
-- entirely.
SELECT id FROM properties
WHERE owner_id = $1 AND status IN ('active', 'maintenance')
ORDER BY id
FOR UPDATE;

-- name: LockTaskOwner :one
-- The owner row is the serialization anchor of the property-less cut (ADR
-- 0052): owner-scope mutations lock it before reading or writing a rule, and
-- the full tick locks it after the property rows, so owner-mutation-vs-tick
-- serializes on one point. A bound-rule mutation never takes this lock — the
-- two slices' rules are disjoint row sets, and crossing the lock orders
-- would invite deadlocks.
SELECT id FROM users WHERE id = $1 FOR UPDATE;

-- name: ListTickTaskRulesByOwner :many
-- The owner's tick read side: rules without a property plus rules on
-- non-archived properties (ADR 0052). The LEFT JOIN keeps the property-less
-- cut while the status predicate still skips archived ones.
SELECT r.id, r.owner_id, r.property_id, r.title, r.comment, r.due_date, r.due_time, r.repeat
FROM task_rules r
LEFT JOIN properties pr ON pr.id = r.property_id
WHERE r.owner_id = $1
  AND (r.property_id IS NULL OR pr.status IN ('active', 'maintenance'));

-- name: ListTickTaskKeys :many
-- Dedup keys of the listed rules' tasks: existence by (rule_id, due_date)
-- for dated rows and by rule_id alone for the undated one; the completed
-- flag steers the single-future walk (completed-ahead rows are skipped).
SELECT rule_id, due_date, (completed_date IS NOT NULL)::boolean AS completed
FROM tasks
WHERE rule_id = ANY($1::uuid[]);

-- name: InsertMaterializedTask :exec
-- Idempotent by the partial unique (rule_id, due_date): a concurrent or
-- repeated run inserts nothing. Always uncompleted — the completion is a
-- manual act only.
INSERT INTO tasks (
    id, owner_id, property_id, rule_id, due_date, due_time, title, comment
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (rule_id, due_date) WHERE rule_id IS NOT NULL AND due_date IS NOT NULL DO NOTHING;

-- name: InsertMaterializedUndatedTask :exec
-- The undated task of an undated rule («Без срока»): idempotent by the
-- partial unique (rule_id) over the undated rows.
INSERT INTO tasks (
    id, owner_id, property_id, rule_id, due_date, due_time, title, comment
)
VALUES ($1, $2, $3, $4, NULL, NULL, $5, $6)
ON CONFLICT (rule_id) WHERE rule_id IS NOT NULL AND due_date IS NULL DO NOTHING;

-- name: DeleteFutureTasksExcept :execrows
-- Future rebuild in one statement: remove every uncompleted strictly future
-- task of the rule except the single allowed one. A NULL keep removes them
-- all — an undated rule keeps no dated future (stale rows from rule edits).
DELETE FROM tasks
WHERE rule_id = $1
  AND completed_date IS NULL
  AND due_date > $2
  AND ($3::date IS NULL OR due_date <> $3);

-- name: GetTaskOwnerTimezone :one
-- The data owner's IANA timezone (ADR 0048): the tick's "today" and the
-- listings' computed buckets are resolved in the property owner's timezone.
-- NOT NULL with the 'Europe/Moscow' default (migration 000088);
-- IANA-validated on write.
SELECT timezone FROM users WHERE id = $1;

-- name: ListTaskTickZones :many
-- The hourly zone sweep of the tasks tick worker (ADR 0048 p.3): the
-- distinct owner timezones having task rules — without a property (ADR 0052)
-- or on active/maintenance properties — with the data owners of each zone.
-- One "today" is computed per zone in Go; owners without tickable rules are
-- not sweep targets. Stateless — every run re-lists, no per-zone or
-- per-owner tick state is kept.
SELECT DISTINCT u.timezone, r.owner_id
FROM task_rules r
JOIN users u ON u.id = r.owner_id
LEFT JOIN properties pr ON pr.id = r.property_id
WHERE r.property_id IS NULL OR pr.status IN ('active', 'maintenance')
ORDER BY u.timezone, r.owner_id;
