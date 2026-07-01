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

-- name: ListTenantContactsWithLeaseStatus :many
SELECT
    tc.id,
    tc.owner_id,
    tc.name,
    tc.surname,
    tc.patronymic,
    tc.phone,
    tc.email,
    tc.comment,
    tc.created_at,
    tc.updated_at,
    l.id AS lease_id,
    l.owner_id AS lease_owner_id,
    l.property_id AS lease_property_id,
    l.status AS lease_status,
    l.start_date AS lease_start_date,
    l.end_date AS lease_end_date,
    l.rent_amount_kopecks AS lease_rent_amount_kopecks,
    l.deposit_amount_kopecks AS lease_deposit_amount_kopecks,
    l.payment_day AS lease_payment_day,
    l.comment AS lease_comment,
    l.created_at AS lease_created_at,
    l.updated_at AS lease_updated_at
FROM tenant_contacts tc
LEFT JOIN leases l ON l.tenant_contact_id = tc.id AND l.owner_id = tc.owner_id
WHERE tc.owner_id = $1
ORDER BY
    CASE WHEN l.status IN ('awaiting_start', 'active', 'requires_action') THEN 0 ELSE 1 END,
    l.updated_at DESC;

-- name: ListTenantContactsByOwnerAdmin :many
SELECT * FROM tenant_contacts
WHERE owner_id = $1
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountTenantContactsByOwnerAdmin :one
SELECT COUNT(*) FROM tenant_contacts
WHERE owner_id = $1;

-- name: GetTenantContactByIDAdmin :one
SELECT * FROM tenant_contacts WHERE id = $1;
