-- name: CreateOperationCategory :one
INSERT INTO operation_categories (owner_id, type, name, code)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateOperationCategoryIgnoreConflict :exec
INSERT INTO operation_categories (owner_id, type, name, code)
VALUES ($1, $2, $3, $4)
ON CONFLICT (owner_id, type, lower(name)) DO NOTHING;

-- name: ListOperationCategoriesByOwner :many
SELECT * FROM operation_categories
WHERE owner_id = $1
  AND (sqlc.arg('type')::text = '' OR type = sqlc.arg('type')::text)
ORDER BY name;

-- name: GetOperationCategoryByIDAndOwner :one
SELECT * FROM operation_categories
WHERE id = $1 AND owner_id = $2;

-- name: GetOperationCategoryByOwnerAndCode :one
SELECT * FROM operation_categories
WHERE owner_id = $1 AND code = $2;
