-- Payments context queries: the materialization tick's persistence
-- (ADR 0049 §3, ticket #458). The owner-level payment listing with pauses
-- resolved, the operation dedup keys, the idempotent insert, the auto-pay day
-- payment and the future-planned rebuild. Rule CRUD lives in
-- payments_rules.sql, operations/favorites in payments_operations.sql.

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
-- Future-planned rebuild in one statement: remove every future planned
-- operation of the rule except the single allowed one. A NULL keep removes
-- them all — the rule is paused or ended, no future planned may remain
-- (ADR 0049 §2).
DELETE FROM operations
WHERE payment_id = $1
  AND status = 'planned'
  AND date > $2
  AND ($3::date IS NULL OR date <> $3);

-- name: GetOwnerTimezone :one
-- The data owner's IANA timezone (ADR 0048): the tick's "today" is the
-- calendar date in the property owner's timezone. NOT NULL with the
-- 'Europe/Moscow' default (migration 000088); IANA-validated on write.
SELECT timezone FROM users WHERE id = $1;

-- name: ListTickZones :many
-- The hourly zone sweep of the tick worker (ADR 0048 p.3): the distinct owner
-- timezones having payment rules on active/maintenance properties, with the
-- data owners of each zone. One "today" is computed per zone in Go; owners
-- without rules on such properties are not sweep targets. Stateless — every
-- run re-lists, no per-zone or per-owner tick state is kept.
SELECT DISTINCT u.timezone, pay.owner_id
FROM payments pay
JOIN properties pr ON pr.id = pay.property_id
JOIN users u ON u.id = pay.owner_id
WHERE pr.status IN ('active', 'maintenance')
ORDER BY u.timezone, pay.owner_id;
