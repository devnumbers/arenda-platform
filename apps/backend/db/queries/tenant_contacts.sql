-- name: CreateTenantContact :one
INSERT INTO tenant_contacts (owner_id, name, surname, patronymic, phone, email, comment)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetTenantContactByIDAndOwner :one
SELECT * FROM tenant_contacts
WHERE id = $1 AND owner_id = $2;

-- name: UpdateTenantContact :one
UPDATE tenant_contacts
SET name = $2,
    surname = $3,
    patronymic = $4,
    phone = $5,
    email = $6,
    comment = $7
WHERE id = $1 AND owner_id = $8
RETURNING *;

-- name: ListTenantContactsByOwner :many
SELECT * FROM tenant_contacts
WHERE owner_id = $1
ORDER BY updated_at DESC;

-- name: ListTenantContactsByIDs :many
SELECT * FROM tenant_contacts
WHERE owner_id = $1 AND id = ANY(@ids::uuid[]);
