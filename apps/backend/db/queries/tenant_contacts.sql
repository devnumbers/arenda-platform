-- name: CreateTenantContact :one
INSERT INTO tenant_contacts (owner_id, name, surname, patronymic, phone, email, comment)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetTenantContactByIDAndOwner :one
SELECT * FROM tenant_contacts
WHERE id = $1 AND owner_id = $2;

-- name: ListTenantContactsByOwner :many
SELECT * FROM tenant_contacts
WHERE owner_id = $1
ORDER BY updated_at DESC;
