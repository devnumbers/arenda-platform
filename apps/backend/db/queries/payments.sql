-- Payments context queries (ADR 0049, ticket #454).
--
-- This file carries the materialization tick's persistence: the owner-level
-- payment listing with pauses resolved, the operation dedup keys, the
-- idempotent insert, the auto-pay day payment and the future-planned rebuild.
-- CRUD and listing queries arrive with their tickets (#457, #461).

-- name: LockOwnerTickProperties :many
-- Serialization point of the tick (ADR 0049 §3): the run locks the owner's
-- active/maintenance property rows, ordered by id, before reading or writing
-- anything. Context mutations take the same lock per property through
-- GetPropertyByIDAndOwnerForUpdate, so update-vs-tick, archive-vs-tick and
-- delete-vs-tick serialize on one point. Archived properties are skipped by
-- the tick entirely.
SELECT id FROM properties
WHERE owner_id = $1 AND status IN ('active', 'maintenance')
ORDER BY id
FOR UPDATE;

-- name: ListTickPaymentsByOwner :many
-- The owner's payment rules on non-archived properties, with the user
-- category's current name resolved for the materialization snapshot. Pauses
-- are listed separately (ListTickPausesByPaymentIDs).
SELECT pay.id,
       pay.owner_id,
       pay.property_id,
       pay.type,
       pay.title,
       pay.amount_kopecks,
       pay.recurrence,
       pay.since,
       pay.end_date,
       pay.auto_pay,
       pay.payment_form,
       pay.category_slug,
       pay.user_category_id,
       pc.name AS user_category_name
FROM payments pay
JOIN properties pr ON pr.id = pay.property_id
LEFT JOIN payment_categories pc ON pc.id = pay.user_category_id
WHERE pay.owner_id = $1
  AND pr.status IN ('active', 'maintenance');

-- name: ListTickPausesByPaymentIDs :many
-- Pause intervals of the listed payments (ADR 0002 of the prototype: the
-- holes are reproducible from the rule plus intervals).
SELECT id, payment_id, from_date, to_date
FROM payment_pauses
WHERE payment_id = ANY($1::uuid[])
ORDER BY payment_id, from_date;

-- name: ListTickOperationStatuses :many
-- Dedup keys and statuses of the listed payments' operations: existence by
-- (payment_id, date), status for the future-planned rebuild.
SELECT payment_id, date, status
FROM operations
WHERE payment_id = ANY($1::uuid[]);

-- name: InsertMaterializedOperation :exec
-- Idempotent by the partial unique (payment_id, date): a concurrent or
-- repeated run inserts nothing. Always planned — the auto-pay closes today's
-- occurrence separately, strictly on its day (ADR 0049 §2).
INSERT INTO operations (
    id, owner_id, property_id, payment_id, origin, date, paid_date, status,
    type, title, amount_kopecks, payment_form, category_label, category_slug
)
VALUES ($1, $2, $3, $4, 'payment', $5, NULL, 'planned', $6, $7, $8, $9, $10, $11)
ON CONFLICT (payment_id, date) WHERE payment_id IS NOT NULL DO NOTHING;

-- name: PayOperationDueToday :execrows
-- The auto-pay day payment (ADR 0049 §2): planned with date = today becomes
-- paid, paid_date = today. Strictly today — never backdated; the active
-- pause is excluded by the caller (the domain plan), and occurrences inside
-- a pause are not generated at all.
UPDATE operations
SET status = 'paid', paid_date = $2
WHERE payment_id = $1 AND status = 'planned' AND date = $2;

-- name: DeleteFuturePlannedExcept :execrows
-- Future-planned rebuild: remove every future planned operation of the rule
-- except the single allowed one (stale rows left by rule edits).
DELETE FROM operations
WHERE payment_id = $1 AND status = 'planned' AND date > $2 AND date <> $3;

-- name: DeleteFuturePlannedAll :execrows
-- Future-planned rebuild, no-survivor variant: the rule is paused or ended,
-- no future planned may remain.
DELETE FROM operations
WHERE payment_id = $1 AND status = 'planned' AND date > $2;

-- name: GetOwnerTimezone :one
-- The data owner's IANA timezone (ADR 0048): the tick's "today" is the
-- calendar date in the property owner's timezone. NOT NULL with the
-- 'Europe/Moscow' default (migration 000088); IANA-validated on write.
SELECT timezone FROM users WHERE id = $1;
