-- name: CreateProperty :one
INSERT INTO properties (id, owner_id, name, type, address, description, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetPropertyByIDAndOwner :one
SELECT properties.*,
       (SELECT COUNT(*) FROM operations o
         JOIN operation_categories cat ON cat.id = o.category_id
         WHERE o.property_id = properties.id
           AND o.owner_id = properties.owner_id
           AND o.status = 'overdue'
           AND o.type = 'income'
           AND cat.code = 'rent'
           AND o.deleted_at IS NULL) AS overdue_rent_count
FROM properties
WHERE properties.id = $1 AND properties.owner_id = $2;

-- name: GetPropertyByIDAndOwnerForUpdate :one
SELECT * FROM properties WHERE id = $1 AND owner_id = $2
FOR UPDATE;

-- name: GetPropertyStatusByOwner :one
SELECT status FROM properties
WHERE id = $1 AND owner_id = $2;

-- name: ListActivePropertiesByOwner :many
SELECT properties.*,
       (SELECT COUNT(*) FROM operations o
         JOIN operation_categories cat ON cat.id = o.category_id
         WHERE o.property_id = properties.id
           AND o.owner_id = properties.owner_id
           AND o.status = 'overdue'
           AND o.type = 'income'
           AND cat.code = 'rent'
           AND o.deleted_at IS NULL) AS overdue_rent_count
FROM properties
WHERE properties.owner_id = $1 AND properties.status IN ('active', 'maintenance')
ORDER BY properties.updated_at DESC;

-- name: ListArchivedPropertiesByOwner :many
SELECT properties.*,
       (SELECT COUNT(*) FROM operations o
         JOIN operation_categories cat ON cat.id = o.category_id
         WHERE o.property_id = properties.id
           AND o.owner_id = properties.owner_id
           AND o.status = 'overdue'
           AND o.type = 'income'
           AND cat.code = 'rent'
           AND o.deleted_at IS NULL) AS overdue_rent_count
FROM properties
WHERE properties.owner_id = $1 AND properties.status = 'archived'
ORDER BY properties.updated_at DESC;

-- name: UpdateProperty :one
UPDATE properties
SET name = $3, type = $4, address = $5, description = $6, status = $7
WHERE id = $1 AND owner_id = $2
RETURNING *;

-- name: ArchiveProperty :one
UPDATE properties SET status = 'archived'
WHERE id = $1 AND owner_id = $2
RETURNING *;

-- name: UnarchiveProperty :one
UPDATE properties SET status = 'active'
WHERE id = $1 AND owner_id = $2
RETURNING *;

-- name: CountActivePropertiesByOwner :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND status IN ('active', 'maintenance');

-- name: ListPropertiesAdmin :many
SELECT p.*, u.phone AS owner_phone, u.phone_encrypted AS owner_phone_encrypted
FROM properties p
JOIN users u ON p.owner_id = u.id
WHERE (sqlc.arg('owner_id')::uuid IS NULL OR p.owner_id = sqlc.arg('owner_id')::uuid)
  AND (sqlc.arg('status')::text = '' OR p.status = sqlc.arg('status')::text)
  AND (sqlc.arg('q')::text = '' OR p.name ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\' OR p.address ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\')
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('order')::text = 'asc' THEN p.name END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('order')::text = 'desc' THEN p.name END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'createdAt' AND sqlc.arg('order')::text = 'asc' THEN p.created_at END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'createdAt' AND sqlc.arg('order')::text = 'desc' THEN p.created_at END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'updatedAt' AND sqlc.arg('order')::text = 'asc' THEN p.updated_at END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'updatedAt' AND sqlc.arg('order')::text = 'desc' THEN p.updated_at END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'status' AND sqlc.arg('order')::text = 'asc' THEN p.status END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'status' AND sqlc.arg('order')::text = 'desc' THEN p.status END DESC,
  CASE WHEN sqlc.arg('sort')::text = '' THEN p.updated_at END DESC,
  p.id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountPropertiesAdmin :one
SELECT COUNT(*) FROM properties
WHERE (sqlc.arg('owner_id')::uuid IS NULL OR owner_id = sqlc.arg('owner_id')::uuid)
  AND (sqlc.arg('status')::text = '' OR status = sqlc.arg('status')::text)
  AND (sqlc.arg('q')::text = '' OR name ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\' OR address ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\');

-- name: GetPropertyByIDAdmin :one
SELECT p.*, u.phone AS owner_phone, u.phone_encrypted AS owner_phone_encrypted
FROM properties p
JOIN users u ON p.owner_id = u.id
WHERE p.id = $1;

-- name: CountActivePropertiesByOwnerAdmin :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND status IN ('active', 'maintenance');

-- name: CountArchivedPropertiesByOwnerAdmin :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND status = 'archived';

-- name: GetPropertiesStatsAdmin :one
-- "active" mirrors CountActivePropertiesByOwnerAdmin: active plus maintenance.
SELECT
  COUNT(*) FILTER (WHERE status IN ('active', 'maintenance')) AS active_count,
  COUNT(*) FILTER (WHERE status = 'archived') AS archived_count
FROM properties;
