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
       op.category_label,
       op.category_slug,
       op.updated_at
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
SET status = 'paid', paid_date = $3, paid_source = 'manual'
WHERE id = $1 AND owner_id = $2 AND status = 'planned';

-- name: CancelOperationByID :execrows
-- «Удалить операцию» (решение владельца): tombstone-статус cancelled —
-- строка остаётся с ключом (payment_id, date) и не воскресает на тике,
-- paid_date очищается вместе с фактом оплаты (штамп источника — вместе с
-- ним, #1169). Только planned и paid;
-- прочие строки (включая уже отменённые) не трогаются — use case рапортует
-- not-found: отменённая операция для всех чтений больше не существует.
UPDATE operations
SET status = 'cancelled', paid_date = NULL, paid_source = NULL
WHERE id = $1 AND owner_id = $2 AND property_id = $3
  AND status IN ('planned', 'paid');

-- name: CreateManualOperation :exec
-- Ручная разовая операция (тикет #569): рождается оплаченной в «сегодня»
-- владельца — приложение передаёт одну дату, обе колонки берут её. Правила
-- за фактом нет: origin='manual', payment_id NULL (partial unique
-- (payment_id, date) накрывает только платёжные строки и не применяется).
-- Штамп источника — 'manual' (семантика колонки едина, #1169; событие
-- «автоплатёж исполнен» платёжные факты без правила не читает).
INSERT INTO operations (
    id, owner_id, property_id, payment_id, origin, date, paid_date, paid_source,
    status, type, title, amount_kopecks, category_label, category_slug
)
VALUES ($1, $2, $3, NULL, 'manual', $4, $4, 'manual', 'paid', $5, $6, $7, $8, $9);

-- name: ListOperations :many
-- The operations of one scope with pagination (limit/offset), the view status
-- filter ('' is any), an inclusive period on the listing's date key, the
-- sort key (date = the planned operation date — the payment history's
-- default; paid_date = the actual payment fact, ticket #992), the sort
-- direction, a search filter ('' = no filter; the application layer escapes
-- the ILIKE metacharacters, ESCAPE '\'): a case-insensitive substring over
-- the title and the category snapshot — and, when the query reads as an
-- amount, its digits inside the amount's decimal digits in kopecks (the
-- display amount without separators; ticket #476), the direction filter (''
-- is any) and the comma-separated category slugs filter ('' is any; rows
-- without a category snapshot never match a slug).
-- The period bounds follow the sort key: at paid_date a planned row (no
-- fact) falls out of every window, and sorts below the facts in both
-- directions (NULLS LAST) — the operational feed reads payment facts.
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
       op.category_label,
       op.category_slug,
       op.updated_at
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
  AND (sqlc.narg('date_from')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             <= sqlc.narg('date_to'))
  AND (sqlc.arg('search')::text = ''
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR op.category_label ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('search_digits')::text <> ''
           AND CAST(op.amount_kopecks AS text) LIKE '%' || sqlc.arg('search_digits')::text || '%'))
  AND (sqlc.arg('type')::text = '' OR op.type = sqlc.arg('type')::text)
  AND (sqlc.arg('categories')::text = ''
       OR op.category_slug = ANY(string_to_array(sqlc.arg('categories')::text, ',')))
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'paid_date'
        AND sqlc.arg('order')::text = 'asc' THEN op.paid_date END ASC NULLS LAST,
  CASE WHEN sqlc.arg('sort')::text = 'paid_date'
        AND sqlc.arg('order')::text = 'desc' THEN op.paid_date END DESC NULLS LAST,
  -- Внутри одного дня факта — момент оплаты (#1195): updated_at платёжной
  -- строки двигает только сама оплата (после оплаты строка не меняется),
  -- так что tiebreak читает порядок оплат, а не порядок создания строк —
  -- синтетические и заранее материализованные тиком id его не хранят.
  CASE WHEN sqlc.arg('sort')::text = 'paid_date'
        AND sqlc.arg('order')::text = 'asc' THEN op.updated_at END ASC NULLS LAST,
  CASE WHEN sqlc.arg('sort')::text = 'paid_date'
        AND sqlc.arg('order')::text = 'desc' THEN op.updated_at END DESC NULLS LAST,
  CASE WHEN sqlc.arg('sort')::text <> 'paid_date'
        AND sqlc.arg('order')::text = 'asc' THEN op.date END ASC,
  CASE WHEN sqlc.arg('sort')::text <> 'paid_date'
        AND sqlc.arg('order')::text = 'desc' THEN op.date END DESC,
  op.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: SumOperationTotals :many
-- The period totals of one property's operations by direction (ticket #473):
-- the same status/period predicate as ListOperations, aggregated in SQL so
-- the summary cards never re-add a paginated listing client-side. The
-- direction filter deliberately does not apply here — the totals always
-- report both directions (the contract: the type filter narrows only the
-- category breakdown). The category filter narrows the totals (the card
-- mirrors the filtered list, решение владельца 01.10). Cancelled
-- tombstones never count. The search filter (ticket #476) is the listing's
-- predicate — the summary of the searched scope stays consistent with its
-- list.
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
  AND (sqlc.narg('date_from')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             <= sqlc.narg('date_to'))
  AND (sqlc.arg('search')::text = ''
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR op.category_label ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('search_digits')::text <> ''
           AND CAST(op.amount_kopecks AS text) LIKE '%' || sqlc.arg('search_digits')::text || '%'))
  AND (sqlc.arg('categories')::text = ''
       OR op.category_slug = ANY(string_to_array(sqlc.arg('categories')::text, ',')))
GROUP BY op.type;

-- name: SumOperationsByCategory :many
-- The per-category breakdown behind the category chips and the summary
-- cards' bar (ticket #473): one row per category snapshot present in the
-- scope, largest total first; rows without a category snapshot are skipped
-- (no chip identity — their amounts still count in the totals). The search
-- filter (ticket #476) is the listing's predicate: the breakdown over the
-- searched scope is the search screen's matched-category chips. The
-- category filter narrows the breakdown to the selected slugs (the chips
-- multi-select); the period follows the listing's date key (ticket #992).
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
  AND (sqlc.narg('date_from')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             <= sqlc.narg('date_to'))
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

-- name: CountOverdueOperationsByPayment :one
-- The overdue-occurrences count of one rule (#817: the rentals progress'
-- overdueMonths — «Просрочено N месяцев» counts the managed payment's
-- planned rows dated before the owner's today). The same computed truth the
-- listings report (domain.OperationView mirrors the predicate; ticket
-- #461); paid facts never read as overdue, cancelled tombstones never count,
-- the nested payment→property path is enforced in the WHERE clause.
SELECT COUNT(*)::bigint
FROM operations
WHERE owner_id = sqlc.arg('owner')
  AND property_id = sqlc.arg('property')
  AND payment_id = sqlc.arg('payment')
  AND status = 'planned'
  AND date < sqlc.arg('today');

-- name: CountPaidAndOverdueByPaymentIDs :many
-- The progress counters of the listed rules in one batched read (ticket
-- #845): the rentals list's «N из M» / «Просрочено N месяцев» — the same
-- paid and overdue predicates the per-payment counts encode, split by
-- COUNT FILTER over one GROUP BY instead of a query pair per rule. A rule
-- with no matching operations yields no row — its zeros default in the
-- application. Cancelled tombstones never count; the nested
-- payment→property path is enforced in the WHERE clause. An empty id list
-- never reaches the query (the ListRentalManagedPaymentIDs precedent).
SELECT payment_id,
       COUNT(*) FILTER (WHERE status = 'paid') AS paid_count,
       COUNT(*) FILTER (WHERE status = 'planned' AND date < sqlc.arg('today')) AS overdue_count
FROM operations
WHERE owner_id = sqlc.arg('owner')
  AND property_id = sqlc.arg('property')
  AND payment_id = ANY(@payment_ids::uuid[])
  AND status IN ('paid', 'planned')
GROUP BY payment_id;

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
-- ordering (the sort key, id tiebreak) and filter vocabulary minus the status
-- filter: paid is the feed's only stored status. property_name is the row's
-- property label — the global screen's row label.
--
-- The sort key (ticket #992): date — the planned operation date (the
-- default), paid_date — the actual payment fact the operational feeds read;
-- the feed is paid-only, so the fact is never NULL (CHECK
-- paid ⟺ paid_date NOT NULL). The period bounds follow the same key.
--
-- The page walks the feed's own order by keyset (ticket #597): the window
-- resumes strictly after the (sort key, id) the previous page ended on, so
-- rows created, deleted or renamed between loads never duplicate or drop.
-- The id tiebreak runs DESC in both directions, so the continuation is
-- (date ahead of the cursor) or (same date, id below it); the direction
-- only flips the date comparison. Both cursor args travel together; NULL
-- (no cursor) reads from the beginning.
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
       op.category_label,
       op.category_slug,
       op.updated_at,
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
  AND (sqlc.narg('date_from')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             <= sqlc.narg('date_to'))
  AND (sqlc.arg('search')::text = ''
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR op.category_label ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('search_digits')::text <> ''
           AND CAST(op.amount_kopecks AS text) LIKE '%' || sqlc.arg('search_digits')::text || '%'))
  AND (sqlc.arg('type')::text = '' OR op.type = sqlc.arg('type')::text)
  AND (sqlc.arg('categories')::text = ''
       OR op.category_slug = ANY(string_to_array(sqlc.arg('categories')::text, ',')))
  AND (sqlc.narg('after_id')::uuid IS NULL
       OR (sqlc.arg('order')::text = 'asc'
           AND (sqlc.arg('sort')::text = 'paid_date'
                AND (op.paid_date > sqlc.narg('after_date')::date
                     OR (op.paid_date = sqlc.narg('after_date')
                         AND op.id < sqlc.narg('after_id')::uuid))
             OR sqlc.arg('sort')::text <> 'paid_date'
                AND (op.date > sqlc.narg('after_date')::date
                     OR (op.date = sqlc.narg('after_date')
                         AND op.id < sqlc.narg('after_id')::uuid))))
       OR (sqlc.arg('order')::text <> 'asc'
           AND (sqlc.arg('sort')::text = 'paid_date'
                AND (op.paid_date < sqlc.narg('after_date')::date
                     OR (op.paid_date = sqlc.narg('after_date')
                         AND op.id < sqlc.narg('after_id')::uuid))
             OR sqlc.arg('sort')::text <> 'paid_date'
                AND (op.date < sqlc.narg('after_date')::date
                     OR (op.date = sqlc.narg('after_date')
                         AND op.id < sqlc.narg('after_id')::uuid)))))
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'paid_date'
        AND sqlc.arg('order')::text = 'asc' THEN op.paid_date END ASC NULLS LAST,
  CASE WHEN sqlc.arg('sort')::text = 'paid_date'
        AND sqlc.arg('order')::text = 'desc' THEN op.paid_date END DESC NULLS LAST,
  CASE WHEN sqlc.arg('sort')::text <> 'paid_date'
        AND sqlc.arg('order')::text = 'asc' THEN op.date END ASC,
  CASE WHEN sqlc.arg('sort')::text <> 'paid_date'
        AND sqlc.arg('order')::text = 'desc' THEN op.date END DESC,
  op.id DESC
LIMIT sqlc.arg('limit');

-- name: CountPaidOperationsGlobal :one
-- The global feed's whole-scope count (ticket #599): the list query's
-- predicate — paid, the visibility, the archive cut, the propertyIds
-- multi-select, the period on the listing's date key, the search over
-- title/category label/amount digits, the direction and category filters —
-- without the keyset key, the ordering and the window. The count is the
-- scope's own, identical on every walked page; the search screen shows it
-- as «найдено N».
SELECT COUNT(*)
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
  AND (sqlc.narg('date_from')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             <= sqlc.narg('date_to'))
  AND (sqlc.arg('search')::text = ''
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR op.category_label ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('search_digits')::text <> ''
           AND CAST(op.amount_kopecks AS text) LIKE '%' || sqlc.arg('search_digits')::text || '%'))
  AND (sqlc.arg('type')::text = '' OR op.type = sqlc.arg('type')::text)
  AND (sqlc.arg('categories')::text = ''
       OR op.category_slug = ANY(string_to_array(sqlc.arg('categories')::text, ',')));

-- name: SumPaidOperationTotalsGlobal :many
-- The period totals by direction of the actor's visible merged feed (ticket
-- #540): the propertyIds filter, the period and the category filter narrow
-- the totals (the card mirrors the filtered list, решение владельца
-- 01.10); the type filter deliberately does not — the totals always report
-- both directions whatever the breakdown is narrowed to. Types absent from
-- the scope miss from the result — the adapter reports them as zero.
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
  AND (sqlc.narg('date_from')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             <= sqlc.narg('date_to'))
  AND (sqlc.arg('search')::text = ''
       OR op.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR op.category_label ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('search_digits')::text <> ''
           AND CAST(op.amount_kopecks AS text) LIKE '%' || sqlc.arg('search_digits')::text || '%'))
  AND (sqlc.arg('categories')::text = ''
       OR op.category_slug = ANY(string_to_array(sqlc.arg('categories')::text, ',')))
GROUP BY op.type;

-- name: SumPaidOperationsByCategoryGlobal :many
-- The per-category breakdown of the actor's visible merged feed (ticket
-- #540), largest total first; rows without a category snapshot are skipped
-- (no chip identity — their amounts still count in the totals). Both the
-- type and the category filters narrow this read only. The period follows
-- the listing's date key (ticket #992).
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
  AND (sqlc.narg('date_from')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to')::date IS NULL
       OR (CASE WHEN sqlc.arg('sort')::text = 'paid_date' THEN op.paid_date ELSE op.date END)
             <= sqlc.narg('date_to'))
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

-- name: ListPropertyIDsWithOverdueOperations :many
-- The payments half of the list red dot (ticket #585, резолюция #584): the
-- subset of the given properties holding at least one overdue planned
-- operation — a stored planned row dated before the owner's today, the same
-- predicate ListOperations resolves as the overdue view status in exactly
-- one place. One batched read per data owner; cancelled tombstones are not
-- planned rows and never match.
SELECT DISTINCT op.property_id
FROM operations op
WHERE op.owner_id = sqlc.arg('owner')
  AND op.status = 'planned'
  AND op.date < sqlc.arg('today')
  AND op.property_id = ANY(sqlc.arg('property_ids')::uuid[]);

-- name: NearestDateInputsOfPayments :many
-- The stored aggregates the next payment date resolution consumes (ticket
-- #991) in one batched read: the earliest planned operation on or after
-- today (the stored half — a paid prepaid nearest is not «следующая») and
-- the newest materialized date across planned and paid (the projection
-- cursor — the IsCompleted rule; the tick has not stood the single future
-- planned up yet). Cancelled tombstones are not materialized facts. A rule
-- with no matching operations yields no row — the consumer resolves from
-- nils. The nested payment→property path is enforced in the WHERE clause;
-- an empty id list never reaches the query.
SELECT payment_id,
       MIN(date) FILTER (WHERE status = 'planned' AND date >= sqlc.arg('today'))::date AS next_planned_date,
       MAX(date) FILTER (WHERE status <> 'cancelled')::date AS last_materialized_date
FROM operations
WHERE owner_id = sqlc.arg('owner')
  AND property_id = sqlc.arg('property')
  AND payment_id = ANY(@payment_ids::uuid[])
GROUP BY payment_id;
