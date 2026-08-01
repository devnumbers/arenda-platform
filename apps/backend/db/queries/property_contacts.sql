-- name: CreatePropertyContact :one
INSERT INTO property_contacts (id, property_id, owner_id, name, phone)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListPropertyContactsByProperty :many
SELECT * FROM property_contacts
WHERE property_id = $1 AND owner_id = $2
ORDER BY created_at ASC;

-- name: GetPropertyContact :one
SELECT * FROM property_contacts
WHERE id = $1 AND owner_id = $2;

-- name: UpdatePropertyContact :one
UPDATE property_contacts SET name = $3, phone = $4
WHERE id = $1 AND owner_id = $2
RETURNING *;

-- name: DeletePropertyContact :exec
DELETE FROM property_contacts
WHERE id = $1 AND owner_id = $2;

-- name: ListPropertyContactsAdmin :many
SELECT *
FROM property_contacts
WHERE property_id = sqlc.arg('property_id')::uuid
ORDER BY created_at ASC, id ASC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountPropertyContactsAdmin :one
SELECT COUNT(*) FROM property_contacts
WHERE property_id = sqlc.arg('property_id')::uuid;
