-- Payments context queries: operations and favorites-facing operation reads
-- (ticket #461, the second contracts slice of ADR 0049 §4). Reads are scoped
-- by the data owner and by the nested path property→payment→operation;
-- "overdue" is not stored anywhere — the status filter and every response
-- item's view status resolve against the owner's today passed in by the
-- application layer (ADR 0048). Rule CRUD lives in payments_rules.sql.

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
WHERE op.id = $1 AND op.owner_id = $2 AND op.property_id = $3
  -- Отменённая операция исчезает для всех чтений (решение владельца):
  -- прямой GET по ней — тот же приватный 404, что у чужой строки.
  AND op.status <> 'cancelled';

-- name: PayOperationByID :execrows
-- «Оплатить сейчас» (planned → paid, paid_date = today in the owner's
-- timezone). The planned guard is belt-and-suspenders over the application's
-- loaded check: rows affected = 0 means already paid or gone.
UPDATE operations
SET status = 'paid', paid_date = $3
WHERE id = $1 AND owner_id = $2 AND status = 'planned';

-- name: CancelOperationByID :execrows
-- «Удалить операцию» (решение владельца): tombstone-статус cancelled —
-- строка остаётся с ключом (payment_id, date) и не воскресает на тике,
-- paid_date очищается вместе с фактом оплаты. Только planned и paid;
-- прочие строки (включая уже отменённые) не трогаются — use case рапортует
-- not-found: отменённая операция для всех чтений больше не существует.
UPDATE operations
SET status = 'cancelled', paid_date = NULL
WHERE id = $1 AND owner_id = $2 AND property_id = $3
  AND status IN ('planned', 'paid');

-- name: ListOperations :many
-- The operations of one scope with pagination (limit/offset), the view status
-- filter ('' is any), an inclusive period on the operation date, the sort
-- direction and a case-insensitive substring search by title ('' = no filter;
-- the application layer escapes the ILIKE metacharacters, ESCAPE '\').
-- A NULL payment widens the scope from one rule to every rule of
-- the property: "planned" and "overdue" split the stored planned rows against
-- the owner's today — overdue is computed here from the same truth the
-- response items report, in exactly one place (domain.OperationView mirrors
-- this predicate for already-loaded rows; ticket #461).
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
  -- Отменённые операции (надгробия) исчезают из всех выборок: не долг
  -- (не planned), не факт (не paid); выдача по умолчанию их тоже не показывает.
  AND op.status <> 'cancelled'
  AND (sqlc.narg('payment')::uuid IS NULL OR op.payment_id = sqlc.narg('payment'))
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
  AND (sqlc.arg('search')::text = ''
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\')
ORDER BY
  CASE WHEN sqlc.arg('order')::text = 'asc' THEN op.date END ASC,
  CASE WHEN sqlc.arg('order')::text = 'desc' THEN op.date END DESC,
  op.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
