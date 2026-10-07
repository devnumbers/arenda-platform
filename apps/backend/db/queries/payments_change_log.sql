-- The payment change log queries (ADR 0065, ticket #1188): the append-only
-- story of one rule's user edits. The insert runs inside the mutation
-- conveyor's transaction; the reads serve the payment screen's «изменения»
-- page — the bidirectional keyset over (created_at, id) DESC, scoped by the
-- data owner and the payment→property path (ADR 0028).

-- name: InsertPaymentChangeLog :exec
-- One append-only row: the id is minted app-side (UUIDv7, ADR 0019),
-- created_at is the database's. Empty changes (pause/resume) travel as the
-- literal [] — the application layer normalizes nil.
INSERT INTO payment_change_log (
    id, payment_id, owner_id, property_id, actor_id, action, changes
)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.arg('changes')::jsonb);

-- name: ListPaymentChanges :many
-- The first page, no cursor: newest first — the feed's one order.
SELECT id, payment_id, actor_id, action, changes, created_at
FROM payment_change_log
WHERE payment_id = $1 AND owner_id = $2 AND property_id = $3
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit');

-- name: ListPaymentChangesBefore :many
-- The next-older page: rows strictly before the (created_at, id) key, newest
-- first — the keyset continuation the index
-- (payment_id, created_at DESC, id DESC) serves.
SELECT id, payment_id, actor_id, action, changes, created_at
FROM payment_change_log
WHERE payment_id = $1 AND owner_id = $2 AND property_id = $3
  AND (created_at, id) < (sqlc.arg('before_created_at'), sqlc.arg('before_id')::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit');

-- name: ListPaymentChangesAfter :many
-- The after-leg of the bidirectional keyset (the prepend of newer rows): an
-- ASC walk strictly after the anchor, so a burst wider than the page is
-- carried from the anchor toward the fresh edge without holes; the adapter
-- restores the feed's DESC order.
SELECT id, payment_id, actor_id, action, changes, created_at
FROM payment_change_log
WHERE payment_id = $1 AND owner_id = $2 AND property_id = $3
  AND (created_at, id) > (sqlc.arg('after_created_at'), sqlc.arg('after_id')::uuid)
ORDER BY created_at ASC, id ASC
LIMIT sqlc.arg('page_limit');
