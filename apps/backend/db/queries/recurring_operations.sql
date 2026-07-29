-- name: CreateRecurringOperation :one
INSERT INTO recurring_operations (
    id, owner_id, property_id, lease_id, type, category_id, name,
    amount_kopecks, start_date, payment_day, end_date,
    periodicity, status, comment, reminder_offset_days
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9, $10, $11,
    $12, $13, $14, $15
)
RETURNING *;

-- name: GetRecurringOperationByLease :many
SELECT * FROM recurring_operations
WHERE lease_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetRecurringOperationByLeaseIDAndOwner :one
SELECT * FROM recurring_operations
WHERE lease_id = $1 AND owner_id = $2
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: ListRecurringOperationsByProperty :many
SELECT * FROM recurring_operations
WHERE owner_id = $1 AND property_id = $2
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: GetRecurringOperationByIDAndOwner :one
SELECT * FROM recurring_operations
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL;

-- name: GetRecurringOperationByIDAndOwnerForUpdate :one
SELECT * FROM recurring_operations
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL
FOR UPDATE;

-- name: UpdateRecurringOperation :one
UPDATE recurring_operations
SET type = $2,
    category_id = $3,
    name = $4,
    amount_kopecks = $5,
    start_date = $6,
    payment_day = $7,
    end_date = $8,
    periodicity = $9,
    comment = $10,
    reminder_offset_days = $11
WHERE id = $1 AND owner_id = $12
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateRecurringOperationStatus :one
UPDATE recurring_operations
SET status = $2
WHERE id = $1 AND owner_id = $3
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateRecurringOperationStatusByID :one
UPDATE recurring_operations
SET status = $2
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateRecurringOperationStatusByLeaseID :many
UPDATE recurring_operations
SET status = $1, updated_at = now()
WHERE lease_id = $2 AND owner_id = $3
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateRecurringOperationReminderOffset :execrows
UPDATE recurring_operations
SET reminder_offset_days = $1, updated_at = now()
WHERE id = $2 AND owner_id = $3
  AND deleted_at IS NULL;

-- name: ListRecurringOperationsByPropertyID :many
SELECT * FROM recurring_operations
WHERE property_id = $1
  AND deleted_at IS NULL;

-- name: UpdateRecurringOperationStatusByPropertyID :exec
UPDATE recurring_operations
SET status = $1, updated_at = now()
WHERE property_id = $2
  AND deleted_at IS NULL;

-- name: DeleteRecurringOperationByLease :exec
DELETE FROM recurring_operations
WHERE lease_id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteRecurringOperation :one
UPDATE recurring_operations
SET deleted_at = now(),
    updated_at = now()
WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: ListRecurringOperationsByOwner :many
SELECT * FROM recurring_operations
WHERE owner_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: DeleteRecurringOperationsByProperty :exec
DELETE FROM recurring_operations
WHERE owner_id = $1 AND property_id = $2;
