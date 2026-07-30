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
WHERE owner_id = $1
ORDER BY updated_at DESC;

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

-- name: ListOpenLeasePropertyIDsByOwner :many
SELECT DISTINCT property_id FROM leases
WHERE owner_id = $1
  AND status IN ('awaiting_start', 'active', 'requires_action');

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

-- name: ListLeasesAdmin :many
SELECT l.*, p.name AS property_name,
       tc.id AS tc_id, tc.owner_id AS tc_owner_id, tc.name AS tc_name,
       tc.surname AS tc_surname, tc.patronymic AS tc_patronymic,
       tc.phone AS tc_phone, tc.email AS tc_email, tc.comment AS tc_comment,
       tc.created_at AS tc_created_at, tc.updated_at AS tc_updated_at
FROM leases l
LEFT JOIN properties p ON p.id = l.property_id
LEFT JOIN tenant_contacts tc ON tc.id = l.tenant_contact_id
WHERE (sqlc.arg('owner_id')::uuid IS NULL OR l.owner_id = sqlc.arg('owner_id')::uuid)
  AND (sqlc.arg('property_id')::uuid IS NULL OR l.property_id = sqlc.arg('property_id')::uuid)
  AND (sqlc.arg('status')::text = '' OR l.status = sqlc.arg('status')::text)
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'startDate' AND sqlc.arg('order')::text = 'asc' THEN l.start_date END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'startDate' AND sqlc.arg('order')::text = 'desc' THEN l.start_date END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'updatedAt' AND sqlc.arg('order')::text = 'asc' THEN l.updated_at END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'updatedAt' AND sqlc.arg('order')::text = 'desc' THEN l.updated_at END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'status' AND sqlc.arg('order')::text = 'asc' THEN l.status END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'status' AND sqlc.arg('order')::text = 'desc' THEN l.status END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'rentAmountKopecks' AND sqlc.arg('order')::text = 'asc' THEN l.rent_amount_kopecks END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'rentAmountKopecks' AND sqlc.arg('order')::text = 'desc' THEN l.rent_amount_kopecks END DESC,
  CASE WHEN sqlc.arg('sort')::text = '' THEN l.updated_at END DESC,
  l.id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountLeasesAdmin :one
SELECT COUNT(*) FROM leases l
WHERE (sqlc.arg('owner_id')::uuid IS NULL OR l.owner_id = sqlc.arg('owner_id')::uuid)
  AND (sqlc.arg('property_id')::uuid IS NULL OR l.property_id = sqlc.arg('property_id')::uuid)
  AND (sqlc.arg('status')::text = '' OR l.status = sqlc.arg('status')::text);

-- name: GetLeaseByIDAdmin :one
SELECT l.*, p.name AS property_name,
       tc.id AS tc_id, tc.owner_id AS tc_owner_id, tc.name AS tc_name,
       tc.surname AS tc_surname, tc.patronymic AS tc_patronymic,
       tc.phone AS tc_phone, tc.email AS tc_email, tc.comment AS tc_comment,
       tc.created_at AS tc_created_at, tc.updated_at AS tc_updated_at
FROM leases l
LEFT JOIN properties p ON p.id = l.property_id
LEFT JOIN tenant_contacts tc ON tc.id = l.tenant_contact_id
WHERE l.id = $1;

-- name: CountLeasesTotalAdmin :one
SELECT COUNT(*) FROM leases;

-- name: DeleteLeasesByProperty :exec
DELETE FROM leases
WHERE owner_id = $1 AND property_id = $2;
