-- name: CreatePropertyContact :one
INSERT INTO property_contacts (id, property_id, owner_id, name, phone)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListPropertyContactsByProperty :many
SELECT * FROM property_contacts
WHERE property_id = $1 AND owner_id = $2
ORDER BY created_at ASC;
