-- name: CreateProperty :one
INSERT INTO properties (owner_id, name, type, address, description, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPropertyByIDAndOwner :one
SELECT * FROM properties WHERE id = $1 AND owner_id = $2;

-- name: GetPropertyByIDAndOwnerForUpdate :one
SELECT * FROM properties WHERE id = $1 AND owner_id = $2
FOR UPDATE;

-- name: ListActivePropertiesByOwner :many
SELECT * FROM properties
WHERE owner_id = $1 AND status = 'active'
ORDER BY updated_at DESC;

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
WHERE owner_id = $1 AND status = 'active';
