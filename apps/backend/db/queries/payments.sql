-- Payments context queries (ADR 0049). This file carries the materialization
-- tick's persistence (the owner-level payment listing with pauses resolved,
-- the operation dedup keys, the idempotent insert, the auto-pay day payment,
-- the future-planned rebuild), the CRUD and pause/resume queries (#457), and
-- the operations/favorites contracts of the second slice (#461).

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

-- Payment CRUD and pause/resume (ticket #457, ADR 0049 §4). Reads and writes
-- are scoped by the data owner (ADR 0028: SQL filters by scope, the policy
-- port has already resolved the actor's role); the nested path payment→property
-- is enforced in the WHERE clause.

-- name: GetPaymentByID :one
-- One rule by id within the owner's scope on the given property, with the
-- user category's current name resolved for the CategoryView.
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
       pay.is_favorite,
       pay.created_at,
       pay.updated_at,
       pc.name AS user_category_name
FROM payments pay
LEFT JOIN payment_categories pc ON pc.id = pay.user_category_id
WHERE pay.id = $1 AND pay.owner_id = $2 AND pay.property_id = $3;

-- name: ListPaymentsByProperty :many
-- The property's rules in creation order (stable for the list response).
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
       pay.is_favorite,
       pay.created_at,
       pay.updated_at,
       pc.name AS user_category_name
FROM payments pay
LEFT JOIN payment_categories pc ON pc.id = pay.user_category_id
WHERE pay.owner_id = $1 AND pay.property_id = $2
ORDER BY pay.created_at, pay.id;

-- name: InsertPayment :exec
-- ids and since are app-side (UUIDv7, the owner's today); recurrence is the
-- domain-validated jsonb; the category arrives as a default-catalog slug in
-- this slice (user_category_id stays NULL).
INSERT INTO payments (
    id, owner_id, property_id, type, title, amount_kopecks, recurrence,
    since, end_date, auto_pay, payment_form, category_slug
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: UpdatePayment :exec
-- Partial PATCH is resolved by the application layer; the statement always
-- writes the full editable set (since is never among it — server-set,
-- prototype decision №17).
UPDATE payments
SET type = $3,
    title = $4,
    amount_kopecks = $5,
    recurrence = $6,
    end_date = $7,
    auto_pay = $8,
    payment_form = $9,
    category_slug = $10
WHERE id = $1 AND owner_id = $2;

-- name: DeletePayment :execrows
-- Hard delete of the rule. Pauses cascade; operations keep their snapshots
-- with payment_id set to NULL by the FK — the "платёж удалён" mark is
-- origin='payment' AND payment_id IS NULL (ticket #446). Planned operations
-- are removed by the caller per keep_overdue before this runs.
DELETE FROM payments WHERE id = $1 AND owner_id = $2;

-- name: InsertPaymentPause :exec
-- The open-ended pause [from, ∞): exactly one may exist per rule — the
-- partial unique index (to_date IS NULL) makes a double pause a constraint
-- violation even past the application check.
INSERT INTO payment_pauses (id, payment_id, from_date, to_date)
VALUES ($1, $2, $3, NULL);

-- name: CloseActivePaymentPause :execrows
-- Resume: close the open interval with today's date (the resume day is
-- already outside the pause, [from, to)).
UPDATE payment_pauses SET to_date = $2
WHERE payment_id = $1 AND to_date IS NULL;

-- name: DeletePaymentPlannedFrom :execrows
-- Deletion of the rule's planned operations from a date on (today): future
-- planned always goes with the rule.
DELETE FROM operations
WHERE payment_id = $1 AND status = 'planned' AND date >= $2;

-- name: DeletePaymentFuturePlanned :execrows
-- The edit invalidation of the future planned (date > today, strictly): the
-- mutation deletes it and the in-transaction tick stands it again with fresh
-- snapshots (spec: "правка пересоздаёт будущее planned"). Today's occurrence
-- is not future — its snapshot is frozen.
DELETE FROM operations
WHERE payment_id = $1 AND status = 'planned' AND date > $2;

-- name: DeletePaymentPlannedBefore :execrows
-- keep_overdue=false: the overdue planned (the accumulated debt) goes too.
-- Paid operations are never touched.
DELETE FROM operations
WHERE payment_id = $1 AND status = 'planned' AND date < $2;

-- name: GetPropertyForPayment :one
-- The read side of the payments property port: the data owner of a property
-- (the SQL scope, ADR 0028) and its lifecycle state for the read gates.
SELECT id, owner_id, status FROM properties WHERE id = $1;

-- name: GetPropertyForPaymentMutation :one
-- The mutation's serialization point (ADR 0049 §3): the property row is
-- locked FOR UPDATE before the rule is read or written, so mutation-vs-tick,
-- mutation-vs-mutation and archive-vs-mutation serialize on one point.
SELECT id, owner_id, status FROM properties WHERE id = $1 FOR UPDATE;

-- Operations and favorites (ticket #461, the second contracts slice of
-- ADR 0049 §4). Reads are scoped by the data owner and by the nested path
-- property→payment→operation; "overdue" is not stored anywhere — the status
-- filter and every response item's view status are resolved against the
-- owner's today passed in by the application layer (ADR 0048).

-- name: SetPaymentFavorite :execrows
-- Atomic PUT favorite (no read-modify-write): the flag is set in one UPDATE.
-- Existence is already proven inside the same transaction under the property
-- lock; :execrows keeps the store honest independently of that ordering.
UPDATE payments SET is_favorite = $3
WHERE id = $1 AND owner_id = $2;

-- name: GetOperationByID :one
-- One operation by id within the owner's scope on the given property — the
-- pay-now use case's load step (manual operations have no payment, so the
-- payment link is not a filter).
SELECT op.id,
       op.owner_id,
       op.property_id,
       op.payment_id,
       op.origin,
       op.date,
       op.paid_date,
       op.status,
       op.type,
       op.title,
       op.amount_kopecks,
       op.payment_form,
       op.category_label,
       op.category_slug
FROM operations op
WHERE op.id = $1 AND op.owner_id = $2 AND op.property_id = $3;

-- name: PayOperationByID :execrows
-- «Оплатить сейчас» (planned → paid, paid_date = today in the owner's
-- timezone). The planned guard is belt-and-suspenders over the application's
-- loaded check: rows affected = 0 means already paid or gone.
UPDATE operations
SET status = 'paid', paid_date = $3
WHERE id = $1 AND owner_id = $2 AND status = 'planned';

-- name: ListOperationsByPayment :many
-- One payment rule's operations with pagination (limit/offset), the view
-- status filter ('' is any), an inclusive period on the operation date and
-- the sort direction. "planned" and "overdue" split the stored planned rows
-- against the owner's today — overdue is computed here from the same truth
-- the response items report (ticket #461).
SELECT op.id,
       op.owner_id,
       op.property_id,
       op.payment_id,
       op.origin,
       op.date,
       op.paid_date,
       op.status,
       op.type,
       op.title,
       op.amount_kopecks,
       op.payment_form,
       op.category_label,
       op.category_slug
FROM operations op
WHERE op.owner_id = sqlc.arg('owner')
  AND op.property_id = sqlc.arg('property')
  AND op.payment_id = sqlc.arg('payment')
  AND (
    sqlc.arg('status')::text = ''
    OR (sqlc.arg('status')::text = 'paid' AND op.status = 'paid')
    OR (sqlc.arg('status')::text = 'planned' AND op.status = 'planned'
        AND op.date >= sqlc.arg('today'))
    OR (sqlc.arg('status')::text = 'overdue' AND op.status = 'planned'
        AND op.date < sqlc.arg('today'))
  )
  AND (sqlc.narg('date_from')::date IS NULL OR op.date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL OR op.date <= sqlc.narg('date_to'))
ORDER BY
  CASE WHEN sqlc.arg('order')::text = 'asc' THEN op.date END ASC,
  CASE WHEN sqlc.arg('order')::text = 'desc' THEN op.date END DESC,
  op.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListOperationsByProperty :many
-- The property's operations across its rules with the same pagination,
-- filters and direction as the per-payment listing. Serves the debt section
-- (status='overdue') and any full operation list of the object.
SELECT op.id,
       op.owner_id,
       op.property_id,
       op.payment_id,
       op.origin,
       op.date,
       op.paid_date,
       op.status,
       op.type,
       op.title,
       op.amount_kopecks,
       op.payment_form,
       op.category_label,
       op.category_slug
FROM operations op
WHERE op.owner_id = sqlc.arg('owner')
  AND op.property_id = sqlc.arg('property')
  AND (
    sqlc.arg('status')::text = ''
    OR (sqlc.arg('status')::text = 'paid' AND op.status = 'paid')
    OR (sqlc.arg('status')::text = 'planned' AND op.status = 'planned'
        AND op.date >= sqlc.arg('today'))
    OR (sqlc.arg('status')::text = 'overdue' AND op.status = 'planned'
        AND op.date < sqlc.arg('today'))
  )
  AND (sqlc.narg('date_from')::date IS NULL OR op.date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL OR op.date <= sqlc.narg('date_to'))
ORDER BY
  CASE WHEN sqlc.arg('order')::text = 'asc' THEN op.date END ASC,
  CASE WHEN sqlc.arg('order')::text = 'desc' THEN op.date END DESC,
  op.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
