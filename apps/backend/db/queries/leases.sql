-- name: CreateLease :one
INSERT INTO leases (
    id, owner_id, property_id, tenant_contact_id, status,
    start_date, end_date, rent_amount_kopecks, deposit_amount_kopecks,
    payment_day, comment
)
VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11
)
RETURNING *;

-- name: GetLeaseByIDAndOwner :one
SELECT * FROM leases
WHERE id = $1 AND owner_id = $2;

-- name: GetLeaseByIDAndOwnerForUpdate :one
SELECT * FROM leases
WHERE id = $1 AND owner_id = $2
FOR UPDATE;

-- name: GetLeaseByID :one
SELECT * FROM leases
WHERE id = $1;

-- name: GetLeaseByIDForUpdate :one
SELECT * FROM leases WHERE id = $1 FOR UPDATE;

-- name: ListLeasesByOwner :many
SELECT * FROM leases
WHERE (leases.owner_id = $1
       OR (leases.property_id = ANY(sqlc.arg('accessible_property_ids')::uuid[])
           AND NOT EXISTS (SELECT 1 FROM properties p WHERE p.id = leases.property_id AND p.status = 'archived')))
ORDER BY leases.updated_at DESC;

-- name: UpdateLease :one
UPDATE leases
SET property_id = $3,
    tenant_contact_id = $4,
    status = $5,
    start_date = $6,
    end_date = $7,
    rent_amount_kopecks = $8,
    deposit_amount_kopecks = $9,
    payment_day = $10,
    comment = $11
WHERE id = $1 AND owner_id = $2
RETURNING *;

-- name: CompleteLease :one
UPDATE leases
SET status = 'completed'
WHERE id = $1 AND owner_id = $2
RETURNING *;

-- name: CountOpenLeasesByProperty :one
SELECT COUNT(*) FROM leases
WHERE property_id = $1
  AND status IN ('awaiting_start', 'active', 'requires_action');

-- name: GetOpenLeaseByProperty :one
SELECT * FROM leases
WHERE owner_id = $1
  AND property_id = $2
  AND status IN ('awaiting_start', 'active', 'requires_action')
ORDER BY updated_at DESC
LIMIT 1;

-- name: ListOpenLeasesWithPastEndDate :many
SELECT * FROM leases
WHERE status IN ('awaiting_start', 'active')
  AND end_date IS NOT NULL
  AND end_date < sqlc.arg('as_of')::date
ORDER BY id
LIMIT sqlc.arg('limit')::int;

-- name: ListLeasesByProperty :many
SELECT * FROM leases
WHERE property_id = $1 AND owner_id = $2
ORDER BY updated_at DESC;

-- name: ListLeasesWithTenantForExport :many
SELECT l.id, l.status, l.start_date, l.end_date,
       l.rent_amount_kopecks, l.deposit_amount_kopecks,
       l.payment_day, l.comment,
       l.tenant_contact_id,
       tc.surname AS tenant_surname,
       tc.name   AS tenant_name,
       tc.patronymic AS tenant_patronymic,
       tc.phone  AS tenant_phone,
       tc.email  AS tenant_email,
       tc.comment AS tenant_comment
FROM leases l
LEFT JOIN tenant_contacts tc ON tc.id = l.tenant_contact_id
WHERE l.property_id = $1 AND l.owner_id = $2
ORDER BY l.start_date DESC, l.id DESC;
