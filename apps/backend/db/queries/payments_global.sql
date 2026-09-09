-- Payments context queries: the global surface of the payment rules
-- (tickets #575, #576) — the merged «Платежи» feed, the search with its
-- matched-category chips, the «Объекты» stacks and the favorites manual
-- order save. Rule CRUD lives in payments_rules.sql, operations in
-- payments_operations.sql, the tick in payments_tick.sql.
--
-- The visibility predicate is the global listings' (ticket #521): the
-- actor's own rows plus the rows of the properties they share with an
-- active membership (ADR 0028 read scope); a suspended membership grants
-- no read. The rules of archived properties are out of every read (map
-- #573). No single scope exists to resolve through the policy port — the
-- actor-scoped read carries its visibility predicate here, beside the data.
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
WHERE (
       p.owner_id = sqlc.arg('actor')
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = p.id
              AND pm.user_id = sqlc.arg('actor')
              AND pm.status = 'active'
          )
      )
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
SELECT pay.id,
       pay.owner_id,
       pay.property_id,
       pay.type,
       pay.title,
       pay.amount_kopecks,
       pay.auto_pay,
       pay.is_favorite,
       pay.favorite_order,
       pay.category_slug,
       pay.user_category_id,
       pc.name AS user_category_name,
       p.name AS property_name,
       t.today::date AS owner_today,
       agg.next_planned_date::date,
       agg.overdue_count,
       agg.oldest_overdue_date::date,
       oldest.oldest_overdue_operation_id
FROM payments pay
JOIN properties p ON p.id = pay.property_id
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
WHERE (
       pay.owner_id = sqlc.arg('actor')
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = pay.property_id
              AND pm.user_id = sqlc.arg('actor')
              AND pm.status = 'active'
          )
      )
  AND p.status != 'archived'
  AND (sqlc.arg('search')::text = ''
       OR pay.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR pc.name ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('category_slugs')::text <> ''
           AND pay.category_slug = ANY(string_to_array(sqlc.arg('category_slugs')::text, ','))))
ORDER BY p.name, pay.created_at, pay.id;

-- name: SumGlobalPaymentCounters :one
-- The main screen's two counters over the whole visible scope (ticket
-- #575): the favorite rules — the «Все избранные (N)» card — and the
-- overdue operations summed across the feed's rules — the «Все
-- просроченные (N)» card. Neither is narrowed by a search: the counters
-- describe the scope, the search screens' rows are the feed query's.
SELECT COUNT(*) FILTER (WHERE pay.is_favorite)::bigint AS favorite_count,
       COALESCE(SUM(agg.overdue_count), 0)::bigint AS overdue_operations_count
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
) agg ON true
WHERE (
       pay.owner_id = sqlc.arg('actor')
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = pay.property_id
              AND pm.user_id = sqlc.arg('actor')
              AND pm.status = 'active'
          )
      )
  AND p.status != 'archived';

-- name: SumGlobalPaymentSearchCategories :many
-- The matched categories of the payment rules search (ticket #575): one
-- row per (category, direction) present among the matched rules — the
-- search screen's chips — the largest count first. The identity is the
-- rule's category reference resolved: a default catalog slug or the user
-- category. Rules without any category reference cannot appear (the XOR
-- is a durable schema invariant; no such rules exist today). The search
-- predicate is the feed query's.
SELECT pay.category_slug,
       pay.user_category_id,
       pc.name AS user_category_label,
       pay.type,
       COUNT(*)::bigint AS rule_count
FROM payments pay
JOIN properties p ON p.id = pay.property_id
LEFT JOIN payment_categories pc ON pc.id = pay.user_category_id
WHERE (
       pay.owner_id = sqlc.arg('actor')
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = pay.property_id
              AND pm.user_id = sqlc.arg('actor')
              AND pm.status = 'active'
          )
      )
  AND p.status != 'archived'
  AND (pay.category_slug IS NOT NULL OR pay.user_category_id IS NOT NULL)
  AND (sqlc.arg('search')::text = ''
       OR pay.title ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR pc.name ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR (sqlc.arg('category_slugs')::text <> ''
           AND pay.category_slug = ANY(string_to_array(sqlc.arg('category_slugs')::text, ','))))
GROUP BY pay.category_slug, pay.user_category_id, pc.name, pay.type
ORDER BY rule_count DESC, pay.category_slug NULLS LAST, pc.name NULLS LAST, pay.type;

-- name: ListGlobalPaymentObjects :many
-- The actor's visible non-archived properties — the «Объекты» screen's
-- stacks (ticket #575); the rules of each stack arrive on the feed query's
-- rows and the application layer groups them. The search ('' = no filter)
-- is a case-insensitive substring over the name and the address — it
-- filters the objects, never their stacks. The order is the global pin's
-- (ticket #577): the pinned first — among themselves by the pin time —
-- then the rest by name. pinned_at travels to the cards for the pin mark.
SELECT p.id,
       p.name,
       p.address,
       p.pinned_at
FROM properties p
WHERE (
       p.owner_id = sqlc.arg('actor')
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = p.id
              AND pm.user_id = sqlc.arg('actor')
              AND pm.status = 'active'
          )
      )
  AND p.status != 'archived'
  AND (sqlc.arg('search')::text = ''
       OR p.name ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR p.address ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\')
ORDER BY p.pinned_at, p.name, p.id;

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
-- The favorites order save's lock pass (ticket #576): FOR UPDATE row locks
-- on the submitted rules — the caller passes the ids sorted, the ORDER BY
-- keeping the lock order deadlock-safe — restricted to the actor's visible
-- non-archived rules (the feed's visibility predicate). An id that does not
-- come back is unknown, foreign or invisible; the application layer folds
-- the three into one privacy-preserving 404. Every row carries the actor's
-- role on the rule's property — the save's write gate is the favorite
-- star's (#461, Full Access+), resolved per row beside the data. Ids
-- travel as the file's csv list; an empty list never reaches the query.
SELECT pay.id,
       pay.is_favorite,
       CASE WHEN pay.owner_id = sqlc.arg('actor') THEN 'owner' ELSE pm.role END::text AS actor_role
FROM payments pay
JOIN properties p ON p.id = pay.property_id
LEFT JOIN property_members pm ON pm.property_id = pay.property_id
     AND pm.user_id = sqlc.arg('actor')
     AND pm.status = 'active'
WHERE pay.id = ANY(string_to_array(sqlc.arg('ids')::text, ',')::uuid[])
  AND (pay.owner_id = sqlc.arg('actor') OR pm.user_id IS NOT NULL)
  AND p.status != 'archived'
ORDER BY pay.id
FOR UPDATE OF pay;

-- name: ClearGlobalPaymentFavoriteOrders :execrows
-- The favorites order save's full-replacement pass (ticket #576): the
-- visible favorites OUTSIDE the submitted list lose their positions —
-- they fall to the end of the reading order («новое избранное — в конец»),
-- so duplicate positions never survive a save. '' keeps the empty list
-- working (a save of nothing clears the whole visible order).
UPDATE payments pay
SET favorite_order = NULL
FROM properties p
WHERE pay.property_id = p.id
  AND pay.is_favorite
  AND pay.favorite_order IS NOT NULL
  AND (sqlc.arg('keep_ids')::text = ''
       OR pay.id <> ALL(string_to_array(sqlc.arg('keep_ids')::text, ',')::uuid[]))
  AND (
       pay.owner_id = sqlc.arg('actor')
       OR EXISTS (
            SELECT 1 FROM property_members pm
            WHERE pm.property_id = pay.property_id
              AND pm.user_id = sqlc.arg('actor')
              AND pm.status = 'active'
          )
      )
  AND p.status != 'archived';

-- name: SetPaymentFavoriteOrder :execrows
-- One rule's 1-based favorite position (ticket #576); the id match alone is
-- the guard — the lock pass has already proven existence and visibility in
-- the same transaction. :execrows keeps that ordering honest.
UPDATE payments SET favorite_order = $2
WHERE id = $1;
