-- Payments context queries: payment rule CRUD, pause/resume and the favorite
-- flag (ADR 0049 §4; tickets #457, #461). Reads and writes are scoped by the
-- data owner (ADR 0028: SQL filters by scope, the policy port has already
-- resolved the actor's role); the nested path payment→property is enforced in
-- the WHERE clause. The materialization tick lives in payments_tick.sql,
-- operations in payments_operations.sql.

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
-- search ('' = no filter) is a case-insensitive substring match on the title;
-- the application layer escapes the ILIKE metacharacters (ESCAPE '\').
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
  AND (sqlc.arg('search')::text = ''
       OR pay.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\')
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
-- prototype decision №17). The favorite star has its own atomic UPDATE.
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

-- name: SetPaymentFavorite :execrows
-- Atomic PUT favorite (no read-modify-write): the flag is set in one UPDATE.
-- Existence is already proven inside the same transaction under the property
-- lock; :execrows keeps the store honest independently of that ordering.
UPDATE payments SET is_favorite = $3
WHERE id = $1 AND owner_id = $2;

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
