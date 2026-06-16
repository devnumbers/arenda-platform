-- name: CreateOperation :one
INSERT INTO operations (
    owner_id, property_id, lease_id, recurring_operation_id,
    type, category, amount_kopecks, operation_date, comment, is_exception
)
VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: ListOperationsByLease :many
SELECT * FROM operations
WHERE lease_id = $1
ORDER BY operation_date DESC;

-- name: ListFutureOperationsByLease :many
SELECT * FROM operations
WHERE lease_id = $1 AND operation_date > $2
ORDER BY operation_date ASC;

-- name: DeleteFutureOperationsByLease :exec
DELETE FROM operations
WHERE lease_id = $1 AND operation_date > $2;

-- name: DeleteUneditedFutureOperationsByLease :exec
DELETE FROM operations
WHERE lease_id = $1
  AND is_exception = false
  AND operation_date > $2;
