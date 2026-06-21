-- name: CreateRecurringOperation :one
INSERT INTO recurring_operations (
    owner_id, property_id, lease_id, type, category,
    amount_kopecks, start_date, payment_day, end_date,
    periodicity, status, comment
)
VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11, $12
)
RETURNING *;

-- name: GetRecurringOperationByLease :many
SELECT * FROM recurring_operations
WHERE lease_id = $1
ORDER BY created_at DESC;

-- name: GetRecurringOperationByLeaseIDAndOwner :one
SELECT * FROM recurring_operations
WHERE lease_id = $1 AND owner_id = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: ListRecurringOperationsByProperty :many
SELECT * FROM recurring_operations
WHERE owner_id = $1 AND property_id = $2
ORDER BY created_at DESC;

-- name: GetRecurringOperationByIDAndOwner :one
SELECT * FROM recurring_operations
WHERE id = $1 AND owner_id = $2;

-- name: GetRecurringOperationByIDAndOwnerForUpdate :one
SELECT * FROM recurring_operations
WHERE id = $1 AND owner_id = $2
FOR UPDATE;

-- name: UpdateRecurringOperation :one
UPDATE recurring_operations
SET type = $2,
    category = $3,
    amount_kopecks = $4,
    start_date = $5,
    payment_day = $6,
    end_date = $7,
    comment = $8
WHERE id = $1 AND owner_id = $9
RETURNING *;

-- name: UpdateRecurringOperationStatus :one
UPDATE recurring_operations
SET status = $2
WHERE id = $1 AND owner_id = $3
RETURNING *;

-- name: UpdateRecurringOperationStatusByID :one
UPDATE recurring_operations
SET status = $2
WHERE id = $1
RETURNING *;

-- name: UpdateRecurringOperationStatusByLeaseID :many
UPDATE recurring_operations
SET status = $1, updated_at = NOW()
WHERE lease_id = $2 AND owner_id = $3
RETURNING *;

-- name: UpdateRecurringOperationReminderOffset :execrows
UPDATE recurring_operations
SET reminder_offset_days = $1, updated_at = NOW()
WHERE id = $2 AND owner_id = $3;

-- name: ListRecurringOperationsByPropertyID :many
SELECT * FROM recurring_operations
WHERE property_id = $1;

-- name: UpdateRecurringOperationStatusByPropertyID :exec
UPDATE recurring_operations
SET status = $1, updated_at = NOW()
WHERE property_id = $2;

-- name: DeleteRecurringOperationByLease :exec
DELETE FROM recurring_operations
WHERE lease_id = $1;
