-- name: InsertAuditLog :one
INSERT INTO audit_log (id, created_at, actor_id, actor_role, action, entity_type, entity_id, context, request_id, ip)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id;

-- name: ListAuditLogsAdmin :many
SELECT id, created_at, actor_id, actor_role, action, entity_type, entity_id, context, request_id, ip
FROM audit_log
WHERE (sqlc.arg('actor_id')::uuid IS NULL OR actor_id = sqlc.arg('actor_id')::uuid)
  AND (sqlc.arg('action')::text = '' OR action = sqlc.arg('action')::text)
  AND (sqlc.arg('entity_type')::text = '' OR entity_type = sqlc.arg('entity_type')::text)
  AND (sqlc.arg('date_from')::timestamptz IS NULL OR created_at >= sqlc.arg('date_from')::timestamptz)
  AND (sqlc.arg('date_to')::timestamptz IS NULL OR created_at < sqlc.arg('date_to')::timestamptz)
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'createdAt' AND sqlc.arg('order')::text = 'asc' THEN created_at END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'createdAt' AND sqlc.arg('order')::text = 'desc' THEN created_at END DESC,
  CASE WHEN sqlc.arg('sort')::text = '' THEN created_at END DESC,
  id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountAuditLogsAdmin :one
SELECT COUNT(*)
FROM audit_log
WHERE (sqlc.arg('actor_id')::uuid IS NULL OR actor_id = sqlc.arg('actor_id')::uuid)
  AND (sqlc.arg('action')::text = '' OR action = sqlc.arg('action')::text)
  AND (sqlc.arg('entity_type')::text = '' OR entity_type = sqlc.arg('entity_type')::text)
  AND (sqlc.arg('date_from')::timestamptz IS NULL OR created_at >= sqlc.arg('date_from')::timestamptz)
  AND (sqlc.arg('date_to')::timestamptz IS NULL OR created_at < sqlc.arg('date_to')::timestamptz);

-- name: GetAuditLogByIDAdmin :one
SELECT id, created_at, actor_id, actor_role, action, entity_type, entity_id, context, request_id, ip
FROM audit_log
WHERE id = $1;
