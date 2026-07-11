-- name: CreateProperty :one
INSERT INTO properties (owner_id, name, type, address, description, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPropertyByIDAndOwner :one
SELECT properties.*,
       (SELECT COUNT(*) FROM operations o
         WHERE o.property_id = properties.id
           AND o.owner_id = properties.owner_id
           AND o.status = 'overdue'
           AND o.type = 'income'
           AND o.category = 'rent'
           AND o.deleted_at IS NULL) AS overdue_rent_count
FROM properties
WHERE properties.id = $1 AND properties.owner_id = $2;

-- name: GetPropertyByIDAndOwnerForUpdate :one
SELECT * FROM properties WHERE id = $1 AND owner_id = $2
FOR UPDATE;

-- name: ListActivePropertiesByOwner :many
SELECT properties.*,
       (SELECT COUNT(*) FROM operations o
         WHERE o.property_id = properties.id
           AND o.owner_id = properties.owner_id
           AND o.status = 'overdue'
           AND o.type = 'income'
           AND o.category = 'rent'
           AND o.deleted_at IS NULL) AS overdue_rent_count
FROM properties
WHERE properties.owner_id = $1 AND properties.status IN ('active', 'maintenance')
ORDER BY properties.updated_at DESC;

-- name: ListArchivedPropertiesByOwner :many
SELECT properties.*,
       (SELECT COUNT(*) FROM operations o
         WHERE o.property_id = properties.id
           AND o.owner_id = properties.owner_id
           AND o.status = 'overdue'
           AND o.type = 'income'
           AND o.category = 'rent'
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

-- name: ListPropertiesByOwnerAdmin :many
SELECT * FROM properties
WHERE owner_id = $1
  AND (sqlc.arg('status')::text = '' OR status = sqlc.arg('status')::text)
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountPropertiesByOwnerAdmin :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1
  AND (sqlc.arg('status')::text = '' OR status = sqlc.arg('status')::text);

-- name: GetPropertyByIDAdmin :one
SELECT * FROM properties WHERE id = $1;

-- name: CountActivePropertiesByOwnerAdmin :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND status IN ('active', 'maintenance');

-- name: CountArchivedPropertiesByOwnerAdmin :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND status = 'archived';
