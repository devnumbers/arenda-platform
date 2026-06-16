-- name: CreateRecurringOperation :one
INSERT INTO recurring_operations (
    owner_id, property_id, lease_id, type, category,
    amount_kopecks, start_date, payment_day, end_date
)
VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9
)
RETURNING *;

-- name: GetRecurringOperationByLease :many
SELECT * FROM recurring_operations
WHERE lease_id = $1
ORDER BY created_at DESC;

-- name: UpdateRecurringOperation :one
UPDATE recurring_operations
SET type = $2,
    category = $3,
    amount_kopecks = $4,
    start_date = $5,
    payment_day = $6,
    end_date = $7
WHERE id = $1
RETURNING *;

-- name: DeleteRecurringOperationByLease :exec
DELETE FROM recurring_operations
WHERE lease_id = $1;
