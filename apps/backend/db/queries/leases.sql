-- name: CreateLease :one
INSERT INTO leases (
    owner_id, property_id, tenant_contact_id, status,
    start_date, end_date, rent_amount_kopecks, deposit_amount_kopecks,
    payment_day, comment
)
VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8,
    $9, $10
)
RETURNING *;

-- name: GetLeaseByIDAndOwner :one
SELECT * FROM leases
WHERE id = $1 AND owner_id = $2;

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
WHERE property_id = $1
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
  AND end_date < sqlc.arg('as_of')::date;
