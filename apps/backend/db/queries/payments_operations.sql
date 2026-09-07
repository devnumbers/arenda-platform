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
-- direction, a search filter ('' = no filter; the application layer escapes
-- the ILIKE metacharacters, ESCAPE '\'): a case-insensitive substring over
-- the title and the category snapshot — and, when the query reads as an
-- amount, its digits inside the amount's decimal digits in kopecks (the
-- display amount without separators; ticket #476), the direction filter (''
-- is any) and the comma-separated category slugs filter ('' is any; rows
-- without a category snapshot never match a slug).
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
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR op.category_label ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('search_digits')::text <> ''
           AND CAST(op.amount_kopecks AS text) LIKE '%' || sqlc.arg('search_digits')::text || '%'))
  AND (sqlc.arg('type')::text = '' OR op.type = sqlc.arg('type')::text)
  AND (sqlc.arg('categories')::text = ''
       OR op.category_slug = ANY(string_to_array(sqlc.arg('categories')::text, ',')))
ORDER BY
  CASE WHEN sqlc.arg('order')::text = 'asc' THEN op.date END ASC,
  CASE WHEN sqlc.arg('order')::text = 'desc' THEN op.date END DESC,
  op.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: SumOperationTotals :many
-- The period totals of one property's operations by direction (ticket #473):
-- the same status/period predicate as ListOperations, aggregated in SQL so
-- the summary cards never re-add a paginated listing client-side. The
-- direction filter deliberately does not apply here — the totals always
-- report both directions (the contract: the type filter narrows only the
-- category breakdown). Types absent from the scope simply miss from the
-- result — the adapter reports them as zero. Cancelled tombstones never
-- count. The search filter (ticket #476) is the listing's predicate — the
-- summary of the searched scope stays consistent with its list.
SELECT op.type,
       SUM(op.amount_kopecks)::bigint AS total_kopecks
FROM operations op
WHERE op.owner_id = sqlc.arg('owner')
  AND op.property_id = sqlc.arg('property')
  AND op.status <> 'cancelled'
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
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR op.category_label ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('search_digits')::text <> ''
           AND CAST(op.amount_kopecks AS text) LIKE '%' || sqlc.arg('search_digits')::text || '%'))
GROUP BY op.type;

-- name: SumOperationsByCategory :many
-- The per-category breakdown behind the category chips and the summary
-- cards' bar (ticket #473): one row per category snapshot present in the
-- scope, largest total first; rows without a category snapshot are skipped
-- (no chip identity — their amounts still count in the totals). The search
-- filter (ticket #476) is the listing's predicate: the breakdown over the
-- searched scope is the search screen's matched-category chips.
SELECT op.category_slug,
       op.category_label,
       op.type,
       SUM(op.amount_kopecks)::bigint AS total_kopecks
FROM operations op
WHERE op.owner_id = sqlc.arg('owner')
  AND op.property_id = sqlc.arg('property')
  AND op.status <> 'cancelled'
  AND op.category_slug IS NOT NULL
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
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR op.category_label ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('search_digits')::text <> ''
           AND CAST(op.amount_kopecks AS text) LIKE '%' || sqlc.arg('search_digits')::text || '%'))
  AND (sqlc.arg('type')::text = '' OR op.type = sqlc.arg('type')::text)
GROUP BY op.category_slug, op.category_label, op.type
ORDER BY total_kopecks DESC, op.category_slug;

-- name: CountPaidOperationsByPayment :one
-- The paid-operations count of one rule (ADR 0053 §2: the rentals progress'
-- paidMonths — «N из M месяцев» counts the managed payment's paid facts).
-- Cancelled tombstones never count; the nested payment→property path is
-- enforced in the WHERE clause.
SELECT COUNT(*)::bigint
FROM operations
WHERE owner_id = sqlc.arg('owner')
  AND property_id = sqlc.arg('property')
  AND payment_id = sqlc.arg('payment')
  AND status = 'paid';

-- The global read side (ticket #540): the actor-scoped cross-property read
-- over the paid facts of their own book plus the properties they can view.
-- The visibility predicate is this SQL's (the tasks global feed precedent,
-- ticket #521): no single scope exists to resolve through the
-- policy port, and an active membership grants the read right here; a
-- suspended one does not. The feed is paid-only — planned/overdue are the
-- property screens' vocabulary — and the archived properties are out of it
-- by default: the property join doubles as the archive cut, lifted only by
-- the include_archived opt-in (ticket #549) under the same visibility
-- predicate.
-- The propertyIds filter ('' is any) is validated through the view gate by
-- the application layer before this SQL runs — the uuid[] cast never sees a
-- foreign id (its row would be invisible anyway) or a non-uuid.

-- name: ListPaidOperationsGlobal :many
-- One page of the actor's visible merged feed, the property listing's
-- ordering (op.date, id tiebreak) and filter vocabulary minus the status
-- filter: paid is the feed's only stored status. property_name is the row's
-- property label — the global screen's row label.
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
       op.category_slug,
       p.name AS property_name
FROM operations op
JOIN properties p ON p.id = op.property_id
WHERE op.status = 'paid'
  AND (
       op.owner_id = sqlc.arg('actor')
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = op.property_id
              AND pm.user_id = sqlc.arg('actor')
              AND pm.status = 'active'
          )
      )
  AND (sqlc.arg('include_archived')::bool OR p.status != 'archived')
  AND (sqlc.arg('property_ids')::text = ''
       OR op.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[])
       OR (sqlc.arg('include_archived')::bool AND p.status = 'archived'))
  AND (sqlc.narg('date_from')::date IS NULL OR op.date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL OR op.date <= sqlc.narg('date_to'))
  AND (sqlc.arg('search')::text = ''
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR op.category_label ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('search_digits')::text <> ''
           AND CAST(op.amount_kopecks AS text) LIKE '%' || sqlc.arg('search_digits')::text || '%'))
  AND (sqlc.arg('type')::text = '' OR op.type = sqlc.arg('type')::text)
  AND (sqlc.arg('categories')::text = ''
       OR op.category_slug = ANY(string_to_array(sqlc.arg('categories')::text, ',')))
ORDER BY
  CASE WHEN sqlc.arg('order')::text = 'asc' THEN op.date END ASC,
  CASE WHEN sqlc.arg('order')::text = 'desc' THEN op.date END DESC,
  op.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: SumPaidOperationTotalsGlobal :many
-- The period totals by direction of the actor's visible merged feed (ticket
-- #540): the propertyIds filter and the period narrow the totals, the type
-- and category filters deliberately do not — the totals always report both
-- directions whatever the breakdown is narrowed to. Types absent from the
-- scope miss from the result — the adapter reports them as zero.
SELECT op.type,
       SUM(op.amount_kopecks)::bigint AS total_kopecks
FROM operations op
JOIN properties p ON p.id = op.property_id
WHERE op.status = 'paid'
  AND (
       op.owner_id = sqlc.arg('actor')
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = op.property_id
              AND pm.user_id = sqlc.arg('actor')
              AND pm.status = 'active'
          )
      )
  AND (sqlc.arg('include_archived')::bool OR p.status != 'archived')
  AND (sqlc.arg('property_ids')::text = ''
       OR op.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[])
       OR (sqlc.arg('include_archived')::bool AND p.status = 'archived'))
  AND (sqlc.narg('date_from')::date IS NULL OR op.date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL OR op.date <= sqlc.narg('date_to'))
  AND (sqlc.arg('search')::text = ''
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR op.category_label ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('search_digits')::text <> ''
           AND CAST(op.amount_kopecks AS text) LIKE '%' || sqlc.arg('search_digits')::text || '%'))
GROUP BY op.type;

-- name: SumPaidOperationsByCategoryGlobal :many
-- The per-category breakdown of the actor's visible merged feed (ticket
-- #540), largest total first; rows without a category snapshot are skipped
-- (no chip identity — their amounts still count in the totals). Both the
-- type and the category filters narrow this read only.
SELECT op.category_slug,
       op.category_label,
       op.type,
       SUM(op.amount_kopecks)::bigint AS total_kopecks
FROM operations op
JOIN properties p ON p.id = op.property_id
WHERE op.status = 'paid'
  AND (
       op.owner_id = sqlc.arg('actor')
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = op.property_id
              AND pm.user_id = sqlc.arg('actor')
              AND pm.status = 'active'
          )
      )
  AND (sqlc.arg('include_archived')::bool OR p.status != 'archived')
  AND op.category_slug IS NOT NULL
  AND (sqlc.arg('property_ids')::text = ''
       OR op.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[])
       OR (sqlc.arg('include_archived')::bool AND p.status = 'archived'))
  AND (sqlc.narg('date_from')::date IS NULL OR op.date >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL OR op.date <= sqlc.narg('date_to'))
  AND (sqlc.arg('search')::text = ''
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR op.category_label ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('search_digits')::text <> ''
           AND CAST(op.amount_kopecks AS text) LIKE '%' || sqlc.arg('search_digits')::text || '%'))
  AND (sqlc.arg('type')::text = '' OR op.type = sqlc.arg('type')::text)
  AND (sqlc.arg('categories')::text = ''
       OR op.category_slug = ANY(string_to_array(sqlc.arg('categories')::text, ',')))
GROUP BY op.category_slug, op.category_label, op.type
ORDER BY total_kopecks DESC, op.category_slug;
