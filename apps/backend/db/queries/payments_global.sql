-- Payments context queries: the global surface of the payment rules
-- (tickets #575, #576) — the merged «Платежи» feed, the search with its
-- matched-category chips, the «Объекты» stacks and the favorites manual
-- order save. Rule CRUD lives in payments_rules.sql, operations in
-- payments_operations.sql, the tick in payments_tick.sql.
--
-- The visibility predicate is the global listings' (ticket #521): the
-- actor's own rows plus the rows of the properties they share with an
-- active membership (ADR 0028 read scope); a suspended membership grants
-- no read. The composition lives in one place — the SQL function
-- actor_can_read_property (000142, ADR 0028's derived read); the queries
-- below call it and keep the feed's own stricter archive cut beside it:
-- the rules of archived properties are out of every read (map #573) —
-- the owner included, unlike the history canon where the owner's leg is
-- unconditional. No single scope exists to resolve through the policy
-- port — the actor-scoped read carries its visibility predicate here,
-- beside the data.
--
-- Завершённые правила из всего хаба выведены (тикет #1340): терминальное
-- состояние (completed_at IS NOT NULL) не участвует в ленте, поиске, чипах
-- категорий и счётчиках; страница завершённого жива прямым чтением, личная
-- звезда читателя не ломает чтений. Будущий ридер «На оплату» наследует
-- тот же гейт.
--
-- Every per-row schedule computation (nearest date, overdue) runs against
-- the property owner's calendar date (ADR 0048): the merged feed mixes
-- owners, so the reads take the owner→today map as parallel csv lists —
-- uuids and ISO dates hold no commas — joined by owner_id (payments
-- always carry owner_id of the property's owner; there is no re-binding).

-- name: ListGlobalPaymentOwnerTodays :many
-- The distinct data owners of the actor's visible non-archived properties
-- — the keys of the owner→today map the feed and the counters join on.
-- Objects without rules contribute their owner too: the «Объекты» read
-- lists them all the same.
SELECT DISTINCT p.owner_id
FROM properties p
WHERE actor_can_read_property(p.id, sqlc.arg('actor')::uuid)
  AND p.status != 'archived';

-- name: ListGlobalPaymentRules :many
-- The actor's visible merged feed of payment rules (ticket #575): one row
-- per rule with the property's display name, the owner's today and the
-- rule's schedule aggregates — the earliest stored planned operation on or
-- after today («Ближайший»; the application layer falls back to the pure
-- projection when the row has none, the pause and the settled rule being
-- the true nulls) and the overdue aggregates (planned with the date before
-- today): the count, the oldest date — «N дней» — and the oldest
-- operation's id, the overdue card's link target (ticket #578). Cancelled
-- tombstones never exist for reads. The search ('' = no filter) is a
-- case-insensitive substring over the title and the user category's name;
-- the default catalog's label is not in the database — the application
-- layer expands the query into the matching slugs (category_slugs, '' when
-- none) and they match as a set.
--
-- The reading order is (created_at, id) and the page walks it by keyset
-- (ticket #597): the window resumes strictly after the (created_at, id)
-- the previous page ended on, so rows created, deleted or renamed between
-- loads never duplicate or drop. The property name is deliberately not a
-- sort key — a rename would move rows across the window. Both cursor args
-- travel together; NULL (no cursor) reads from the beginning.
SELECT pay.id,
       pay.owner_id,
       pay.property_id,
       pay.type,
       pay.title,
       pay.amount_kopecks,
       pay.auto_pay,
       (upf.payment_id IS NOT NULL)::bool AS is_favorite,
       upf.favorite_order,
       pay.category_slug,
       pay.user_category_id,
       pay.created_at,
       pc.name AS user_category_name,
       p.name AS property_name,
       t.today::date AS owner_today,
       agg.next_planned_date::date,
       agg.overdue_count,
       agg.oldest_overdue_date::date,
       oldest.oldest_overdue_operation_id
FROM payments pay
JOIN properties p ON p.id = pay.property_id
-- The actor's personal favorite (the cutover #1308, D12): the star and the
-- order position are the reading actor's own — a join, not the frozen
-- payments columns; absent row = not a favorite of this actor.
LEFT JOIN user_payment_favorites upf
       ON upf.payment_id = pay.id
      AND upf.user_id = sqlc.arg('actor')::uuid
JOIN (
       SELECT unnest(string_to_array(sqlc.arg('owner_ids')::text, ',')::uuid[]) AS owner_id,
              unnest(string_to_array(sqlc.arg('todays')::text, ',')::date[]) AS today
     ) AS t ON t.owner_id = pay.owner_id
LEFT JOIN payment_categories pc ON pc.id = pay.user_category_id
LEFT JOIN LATERAL (
    SELECT MIN(op.date) FILTER (WHERE op.status = 'planned' AND op.date >= t.today) AS next_planned_date,
           COUNT(*) FILTER (WHERE op.status = 'planned' AND op.date < t.today) AS overdue_count,
           MIN(op.date) FILTER (WHERE op.status = 'planned' AND op.date < t.today) AS oldest_overdue_date
    FROM operations op
    WHERE op.payment_id = pay.id
      AND op.status <> 'cancelled'
) agg ON true
LEFT JOIN LATERAL (
    SELECT op.id AS oldest_overdue_operation_id
    FROM operations op
    WHERE op.payment_id = pay.id
      AND op.status = 'planned'
      AND op.date < t.today
    ORDER BY op.date ASC, op.id ASC
    LIMIT 1
) oldest ON true
WHERE actor_can_read_property(pay.property_id, sqlc.arg('actor')::uuid)
  AND p.status != 'archived'
  AND pay.completed_at IS NULL
  AND (sqlc.arg('search')::text = ''
       OR pay.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR pc.name ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('category_slugs')::text <> ''
           AND pay.category_slug = ANY(string_to_array(sqlc.arg('category_slugs')::text, ','))))
  AND (sqlc.arg('category_filter')::text = ''
       OR pay.category_slug = sqlc.arg('category_filter')::text
       OR pay.user_category_id::text = sqlc.arg('category_filter')::text)
  AND (sqlc.arg('type_filter')::text = ''
       OR pay.type = sqlc.arg('type_filter')::text)
  AND (sqlc.narg('after_created_at')::timestamptz IS NULL
       OR (pay.created_at, pay.id) > (sqlc.narg('after_created_at'),
                                      sqlc.narg('after_id')::uuid))
ORDER BY pay.created_at, pay.id
LIMIT CASE WHEN sqlc.arg('page_limit')::int = 0 THEN NULL::bigint
           ELSE sqlc.arg('page_limit')::bigint END;

-- name: CountGlobalPaymentRules :one
-- The search's whole-scope match count (ticket #599): the list query's
-- predicate — the visibility, the search over title/user category/catalog
-- slugs, the chip's category+type filter — without the per-row schedule
-- aggregates, the ordering, the window and the keyset key. The count is the
-- scope's own, identical on every walked page; the search screen shows it
-- as «найдено N». The owner→today join travels with the predicate: the map
-- covers every visible owner (ListGlobalPaymentOwnerTodays), so its rows
-- are the list's rows.
SELECT COUNT(*)
FROM payments pay
JOIN properties p ON p.id = pay.property_id
JOIN (
       SELECT unnest(string_to_array(sqlc.arg('owner_ids')::text, ',')::uuid[]) AS owner_id,
              unnest(string_to_array(sqlc.arg('todays')::text, ',')::date[]) AS today
     ) AS t ON t.owner_id = pay.owner_id
LEFT JOIN payment_categories pc ON pc.id = pay.user_category_id
WHERE actor_can_read_property(pay.property_id, sqlc.arg('actor')::uuid)
  AND p.status != 'archived'
  AND pay.completed_at IS NULL
  AND (sqlc.arg('search')::text = ''
       OR pay.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR pc.name ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('category_slugs')::text <> ''
           AND pay.category_slug = ANY(string_to_array(sqlc.arg('category_slugs')::text, ','))))
  AND (sqlc.arg('category_filter')::text = ''
       OR pay.category_slug = sqlc.arg('category_filter')::text
       OR pay.user_category_id::text = sqlc.arg('category_filter')::text)
  AND (sqlc.arg('type_filter')::text = ''
       OR pay.type = sqlc.arg('type_filter')::text);

-- name: SumGlobalPaymentCounters :one
-- The main screen's three counters over the whole visible scope (ticket
-- #575; the due pair — ticket #1346): the actor's favorite rules — the
-- «Все избранные (N)» card, the personal set of the reading actor (the
-- cutover #1308, D12) — the overdue operations summed across the feed's
-- rules — the «Все просроченные (N)» card — and the «На оплату» window:
-- its operations and the nearest days-left, the «На оплату (N)» card's
-- number and label source. None is narrowed by a search: the counters
-- describe the scope, the search screens' rows are the feed query's.
--
-- The due aggregates ride their own lateral beside the overdue one, both
-- sargable-narrow — the overdue leg scans the planned-before-today tail, the
-- due leg the reminder window (the rule terms fold into the WHERE: a NULL
-- reminder makes the date bound NULL, the lateral comes back empty) — and
-- neither ever narrows the outer WHERE: the favorites and the overdue keep
-- counting the whole scope. No pause gate: an open pause keeps no planned
-- operations in the window (the tick removed the future ones, occurrences
-- inside the pause are not generated).
SELECT COUNT(upf.payment_id)::bigint AS favorite_count,
       COALESCE(SUM(overdue.overdue_count), 0)::bigint AS overdue_operations_count,
       COALESCE(SUM(due.due_count), 0)::bigint AS due_operations_count,
       -- -1 = empty window (the card hides the label): bare MIN(int) — CASE,
       -- array_agg and friends included — infers NOT NULL in sqlc, so the
       -- out-of-range sentinel (days-left is 0..7 by construction) travels
       -- and the adapter translates it back to null.
       COALESCE(MIN(due.nearest_days_left), -1)::int AS due_nearest_days
FROM payments pay
JOIN properties p ON p.id = pay.property_id
JOIN (
       SELECT unnest(string_to_array(sqlc.arg('owner_ids')::text, ',')::uuid[]) AS owner_id,
              unnest(string_to_array(sqlc.arg('todays')::text, ',')::date[]) AS today
     ) AS t ON t.owner_id = pay.owner_id
LEFT JOIN LATERAL (
    SELECT COUNT(*) AS overdue_count
    FROM operations op
    WHERE op.payment_id = pay.id
      AND op.status = 'planned'
      AND op.date < t.today
) overdue ON true
LEFT JOIN LATERAL (
    SELECT COUNT(*) AS due_count,
           MIN(op.date) - t.today AS nearest_days_left
    FROM operations op
    WHERE op.payment_id = pay.id
      AND op.status = 'planned'
      AND pay.reminder_offset_days IS NOT NULL
      AND pay.auto_pay = false
      AND op.date >= t.today
      AND op.date <= t.today + pay.reminder_offset_days
) due ON true
-- The personal favorite join (the cutover #1308): COUNT over the actor's
-- joined rows — a stranger's star never lands in the actor's counter.
LEFT JOIN user_payment_favorites upf
       ON upf.payment_id = pay.id
      AND upf.user_id = sqlc.arg('actor')::uuid
WHERE actor_can_read_property(pay.property_id, sqlc.arg('actor')::uuid)
  AND p.status != 'archived'
  AND pay.completed_at IS NULL;

-- name: ListPaymentDue :many
-- The «На оплату» section's rows (ticket #1346): one row per (rule,
-- operation) pair standing in the reminder window — planned operations with
-- D−N ≤ today ≤ D on the rules with the reminder armed (1/3/7) and auto-pay
-- OFF (решение владельца: напоминание у автоплатёжных стреляет, но в секции
-- их нет), the feed's own visibility predicate and archive cut, completed
-- rules cut (the #1340 gate the future reader inherits). The window joins
-- the rule's operations laterally the feed's way: the driver is
-- idx_payments_reminder_offset on the rules side, the per-rule operation
-- scan rides idx_operations_payment_date with the sargable date bounds —
-- no owner-side operation index needed at this scale.
--
-- The pause needs no gate: occurrences inside a pause are never generated
-- and the tick removed the future planned when the pause opened, so a
-- paused rule keeps no in-window operation (the same structural cut the
-- derivation rides for the timezone shifts, ADR 0048 п.5). Overdue
-- operations (D < today) are out by the lower bound — they live in «Все
-- просроченные». Days-left is the window's own math (D − today, 0..7 by
-- construction); the reading order runs the real days ascending (решение
-- владельца 13.10, #1347: the page groups the rows per real day, the chip
-- reverses the group order client-side) with the operation's id the
-- same-day tiebreak. No window: the section comes back whole — the
-- materialization keeps at most two window operations per rule (today's
-- and the next future one), the reminder is armed on a fraction of the
-- rules.
SELECT pay.id,
       op.id AS operation_id,
       pay.property_id,
       p.name AS property_name,
       pay.type,
       pay.title,
       pay.amount_kopecks,
       pay.category_slug,
       pay.user_category_id,
       pc.name AS user_category_name,
       t.today::date AS owner_today,
       op.date::date AS due_date,
       (op.date - t.today)::int AS days_left,
       pay.reminder_offset_days
FROM payments pay
JOIN properties p ON p.id = pay.property_id
JOIN (
       SELECT unnest(string_to_array(sqlc.arg('owner_ids')::text, ',')::uuid[]) AS owner_id,
              unnest(string_to_array(sqlc.arg('todays')::text, ',')::date[]) AS today
     ) AS t ON t.owner_id = pay.owner_id
JOIN operations op ON op.payment_id = pay.id
LEFT JOIN payment_categories pc ON pc.id = pay.user_category_id
WHERE actor_can_read_property(pay.property_id, sqlc.arg('actor')::uuid)
  AND p.status != 'archived'
  AND pay.completed_at IS NULL
  AND pay.reminder_offset_days IS NOT NULL
  AND pay.auto_pay = false
  AND op.status = 'planned'
  AND op.date >= t.today
  AND op.date <= t.today + pay.reminder_offset_days
ORDER BY days_left ASC, op.date DESC, op.id DESC;

-- name: SumGlobalPaymentSearchCategories :many
-- The matched categories of the payment rules search (ticket #575): one
-- row per category present among the matched rules — the search screen's
-- chips — the largest match count first (ticket #602: the count orders the
-- rows and stays out of the contract; a category's directions are one
-- chip). The identity is the rule's category reference resolved: a default
-- catalog slug or the user category. Rules without any category reference
-- cannot appear (the XOR is a durable schema invariant; no such rules exist
-- today). The search predicate is the feed query's.
SELECT pay.category_slug,
       pay.user_category_id,
       pc.name AS user_category_label
FROM payments pay
JOIN properties p ON p.id = pay.property_id
LEFT JOIN payment_categories pc ON pc.id = pay.user_category_id
WHERE actor_can_read_property(pay.property_id, sqlc.arg('actor')::uuid)
  AND p.status != 'archived'
  AND pay.completed_at IS NULL
  AND (pay.category_slug IS NOT NULL OR pay.user_category_id IS NOT NULL)
  AND (sqlc.arg('search')::text = ''
       OR pay.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR pc.name ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('category_slugs')::text <> ''
           AND pay.category_slug = ANY(string_to_array(sqlc.arg('category_slugs')::text, ','))))
GROUP BY pay.category_slug, pay.user_category_id, pc.name
ORDER BY COUNT(*) DESC, pay.category_slug NULLS LAST, pc.name NULLS LAST;

-- name: ListGlobalPaymentObjects :many
-- The actor's visible non-archived properties — the «Объекты» screen's
-- stacks (ticket #575); the rules of each stack arrive on the feed query's
-- rows and the application layer groups them. The search ('' = no filter)
-- is a case-insensitive substring over the name and the address — it
-- filters the objects, never their stacks. The order is the actor's
-- personal pin's (ticket #577, the cutover #1307): the pinned first — among
-- themselves by the pin time — then the rest by name. pinned_at travels
-- from the actor's row in user_property_pins (the LEFT JOIN) to the cards
-- for the pin mark, photo_url is the card avatar (ticket #582): with the
-- private photos (ADR 0065, #1227) one image per object, streamed by the
-- backend — the same-origin path, or NULL without a photo; type picks the
-- avatar's placeholder glyph (карта #1217, #1244).
SELECT p.id,
       p.name,
       p.address,
       p.type,
       up.pinned_at,
       -- COALESCE, not the bare column: sqlc trusts the column's NOT NULL
       -- and emits a string, while the CASE on a NULL key misses into NULL —
       -- an object without a photo would fail the scan. '' = no photo.
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/properties/' || p.id::text || '/photo'
                     END, '')::text AS photo_url
FROM properties p
LEFT JOIN user_property_pins up
       ON up.property_id = p.id
      AND up.user_id = sqlc.arg('actor')::uuid
WHERE actor_can_read_property(p.id, sqlc.arg('actor')::uuid)
  AND p.status != 'archived'
  AND (sqlc.arg('search')::text = ''
       OR p.name ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR p.address ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\')
ORDER BY up.pinned_at, p.name, p.id;

-- name: LastOperationDatesOfPayments :many
-- The newest materialized date across planned and paid per listed rule —
-- the projection cursor of the nearest-date fallback (the tick has not
-- stood the single future planned up yet; CONTEXT.md «Материализация»).
-- Cancelled tombstones are not materialized facts.
SELECT op.payment_id,
       MAX(op.date)::date AS last_date
FROM operations op
WHERE op.status <> 'cancelled'
  AND op.payment_id = ANY(string_to_array(sqlc.arg('payment_ids')::text, ',')::uuid[])
GROUP BY op.payment_id;

-- name: LockGlobalPaymentFavorites :many
-- The favorites order save's lock pass (ticket #576; the personal cutover
-- #1308): FOR UPDATE row locks on the submitted ids' rows of the ACTOR —
-- the caller passes the ids sorted, the ORDER BY keeping the lock order
-- deadlock-safe — restricted to the actor's visible non-archived rules (the
-- feed's visibility predicate). The lock runs on both the actor's
-- user_payment_favorites row (the pass's write target) and the payment row
-- (the archive/delete serialization the global mutations share). An id that
-- does not come back is unknown, foreign, invisible, archived or simply not
-- a favorite of this actor — the personal rows fold the old «visible
-- non-favorite» case into the same privacy-preserving 404 the application
-- layer answers. Every row carries the actor's role on the rule's property
-- — resolved per row beside the data (the star's right is the view one,
-- D12). Ids travel as the file's csv list; an empty list never reaches the
-- query.
SELECT pay.id,
       CASE WHEN pay.owner_id = sqlc.arg('actor') THEN 'owner' ELSE pm.role END::text AS actor_role
FROM user_payment_favorites upf
JOIN payments pay ON pay.id = upf.payment_id
JOIN properties p ON p.id = pay.property_id
-- The role's row only — the visibility is the function call in the WHERE:
-- the join's active-status condition is the membership snapshot's, not a
-- second copy of the predicate.
LEFT JOIN property_members pm ON pm.property_id = pay.property_id
     AND pm.user_id = sqlc.arg('actor')
     AND pm.status = 'active'
WHERE upf.user_id = sqlc.arg('actor')::uuid
  AND pay.id = ANY(string_to_array(sqlc.arg('ids')::text, ',')::uuid[])
  AND actor_can_read_property(pay.property_id, sqlc.arg('actor')::uuid)
  AND p.status != 'archived'
ORDER BY pay.id
FOR UPDATE OF upf, pay;

-- name: ClearGlobalPaymentFavoriteOrders :execrows
-- The favorites order save's full-replacement pass (ticket #576; the
-- personal cutover #1308): the actor's favorites OUTSIDE the submitted list
-- lose their positions — they fall to the end of the reading order («новое
-- избранное — в конец»), so duplicate positions never survive a save. The
-- pass only ever touches the actor's own rows (the personal set, D12) and
-- keeps the feed's visibility and archive cuts: a favorite on an archived
-- property keeps its position through the save — unarchiving returns the
-- rule with its order intact (D16). '' keeps the empty list working (a save
-- of nothing clears the whole visible personal order).
UPDATE user_payment_favorites upf
SET favorite_order = NULL
FROM payments pay
JOIN properties p ON p.id = pay.property_id
WHERE upf.payment_id = pay.id
  AND upf.user_id = sqlc.arg('actor')::uuid
  AND upf.favorite_order IS NOT NULL
  AND (sqlc.arg('keep_ids')::text = ''
       OR upf.payment_id <> ALL(string_to_array(sqlc.arg('keep_ids')::text, ',')::uuid[]))
  AND actor_can_read_property(pay.property_id, sqlc.arg('actor')::uuid)
  AND p.status != 'archived';

-- name: SetPaymentFavoriteOrder :execrows
-- One rule's 1-based favorite position in the actor's personal order
-- (ticket #576; the cutover #1308); the id match alone is the guard — the
-- lock pass has already proven the actor's row and its visibility in the
-- same transaction. :execrows keeps that ordering honest.
UPDATE user_payment_favorites
SET favorite_order = $3
WHERE user_id = $1 AND payment_id = $2;
