-- name: CreateTenantContact :one
INSERT INTO tenant_contacts (id, owner_id, name, surname, patronymic, phone, email, comment)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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

-- name: ListTenantContactsAdmin :many
SELECT tc.*, u.phone AS owner_phone, u.phone_encrypted AS owner_phone_encrypted
FROM tenant_contacts tc
JOIN users u ON tc.owner_id = u.id
WHERE (sqlc.arg('owner_id')::uuid IS NULL OR tc.owner_id = sqlc.arg('owner_id')::uuid)
  AND (sqlc.arg('q')::text = '' OR tc.name ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\'
       OR tc.surname ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\'
       OR tc.phone ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\')
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('order')::text = 'asc' THEN tc.name END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('order')::text = 'desc' THEN tc.name END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'updatedAt' AND sqlc.arg('order')::text = 'asc' THEN tc.updated_at END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'updatedAt' AND sqlc.arg('order')::text = 'desc' THEN tc.updated_at END DESC,
  CASE WHEN sqlc.arg('sort')::text = '' THEN tc.updated_at END DESC,
  tc.id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountTenantContactsAdmin :one
SELECT COUNT(*) FROM tenant_contacts
WHERE (sqlc.arg('owner_id')::uuid IS NULL OR owner_id = sqlc.arg('owner_id')::uuid)
  AND (sqlc.arg('q')::text = '' OR name ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\'
       OR surname ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\'
       OR phone ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\');

-- name: GetTenantContactByIDAdmin :one
SELECT tc.*, u.phone AS owner_phone, u.phone_encrypted AS owner_phone_encrypted
FROM tenant_contacts tc
JOIN users u ON tc.owner_id = u.id
WHERE tc.id = $1;
