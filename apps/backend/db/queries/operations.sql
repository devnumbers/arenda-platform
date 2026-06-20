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
  AND deleted_at IS NULL
ORDER BY operation_date DESC;

-- name: ListOperationDatesByLease :many
SELECT operation_date FROM operations
WHERE lease_id = $1
  AND deleted_at IS NULL;

-- name: ListOperationDatesByRecurringOperation :many
SELECT operation_date FROM operations
WHERE recurring_operation_id = $1
  AND deleted_at IS NULL;

-- name: ListOperationsByRecurringOperation :many
SELECT * FROM operations
WHERE recurring_operation_id = $1
  AND deleted_at IS NULL
ORDER BY operation_date ASC;

-- name: DeleteUneditedFutureOperationsByRecurringOperation :exec
DELETE FROM operations
WHERE recurring_operation_id = $1
  AND is_exception = false
  AND operation_date > $2
  AND deleted_at IS NULL;

-- name: ListFutureOperationsByLease :many
SELECT * FROM operations
WHERE lease_id = $1 AND operation_date > $2
  AND deleted_at IS NULL
ORDER BY operation_date ASC;

-- name: DeleteFutureOperationsByLease :exec
DELETE FROM operations
WHERE lease_id = $1
  AND operation_date > $2
  AND deleted_at IS NULL;

-- name: DeleteUneditedFutureOperationsByLease :exec
DELETE FROM operations
WHERE lease_id = $1
  AND is_exception = false
  AND operation_date > $2
  AND deleted_at IS NULL;

-- name: DeleteOperationsOutsideLeaseRange :exec
DELETE FROM operations
WHERE lease_id = $1
  AND deleted_at IS NULL
  AND (
      operation_date < $2
      OR ($3::date IS NOT NULL AND operation_date > $3::date)
  );

-- name: DeleteUneditedOperationsByLease :exec
DELETE FROM operations
WHERE lease_id = $1
  AND is_exception = false
  AND deleted_at IS NULL;

-- name: ListOperationsByProperty :many
SELECT * FROM operations
WHERE owner_id = $1 AND property_id = $2
  AND deleted_at IS NULL
ORDER BY operation_date DESC;

-- name: GetOperationByIDAndOwner :one
SELECT * FROM operations
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL;

-- name: GetOperationByIDAndOwnerForUpdate :one
SELECT * FROM operations
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL
FOR UPDATE;

-- name: UpdateOperation :one
UPDATE operations
SET type = $3,
    category = $4,
    amount_kopecks = $5,
    operation_date = $6,
    comment = $7,
    lease_id = $8,
    is_exception = true
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteOperation :one
UPDATE operations
SET deleted_at = now(),
    updated_at = now()
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteFutureUneditedOperationsByProperty :exec
DELETE FROM operations
WHERE property_id = $1
  AND is_exception = false
  AND operation_date > $2
  AND deleted_at IS NULL;
