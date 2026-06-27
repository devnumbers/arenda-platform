-- name: CreateOperation :one
INSERT INTO operations (
    owner_id, property_id, lease_id, recurring_operation_id,
    type, category, name, amount_kopecks, operation_date, comment, is_exception, status
)
VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: ListOperationsByLease :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name FROM operations
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
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name FROM operations
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
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name FROM operations
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
  AND operation_date >= $2
  AND deleted_at IS NULL;

-- name: ListOperationsByProperty :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name FROM operations
WHERE owner_id = $1 AND property_id = $2
  AND deleted_at IS NULL
ORDER BY operation_date DESC;

-- name: ListOperationsByPropertyWithStatuses :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name FROM operations
WHERE owner_id = $1 AND property_id = $2
  AND status = ANY(sqlc.arg('statuses')::text[])
  AND deleted_at IS NULL
ORDER BY operation_date DESC;

-- name: GetOperationByIDAndOwner :one
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name FROM operations
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL;

-- name: GetOperationByIDAndOwnerForUpdate :one
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name FROM operations
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL
FOR UPDATE;

-- name: UpdateOperation :one
UPDATE operations
SET type = $3,
    category = $4,
    name = $5,
    amount_kopecks = $6,
    operation_date = $7,
    comment = $8,
    lease_id = $9,
    status = $10,
    is_exception = true
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL
RETURNING *;

-- name: MarkOperationOverdue :one
UPDATE operations
SET status = 'overdue'
WHERE id = $1
  AND owner_id = $2
  AND status = 'pending'
  AND operation_date < sqlc.arg('as_of')::date
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

-- name: ListPendingOperationsWithPastDate :many
SELECT * FROM operations
WHERE owner_id = $1
  AND status = 'pending'
  AND operation_date < sqlc.arg('as_of')::date
  AND deleted_at IS NULL
ORDER BY operation_date ASC
LIMIT sqlc.arg('limit')::int;

-- name: ListAllPendingOperationsWithPastDate :many
SELECT * FROM operations
WHERE status = 'pending'
  AND operation_date < sqlc.arg('as_of')::date
  AND deleted_at IS NULL
ORDER BY operation_date ASC
LIMIT sqlc.arg('limit')::int;

-- name: GetPropertyOperationsSummary :one
SELECT
    (
        COALESCE(SUM(CASE WHEN type = 'income' AND status = 'received' THEN amount_kopecks ELSE 0 END), 0) -
        COALESCE(SUM(CASE WHEN type = 'expense' AND status = 'paid' THEN amount_kopecks ELSE 0 END), 0)
    )::bigint AS all_time_profit_kopecks,
    (
        COALESCE(SUM(CASE
            WHEN type = 'income' AND status = 'received'
                AND operation_date >= date_trunc('month', sqlc.arg('as_of')::date)
                AND operation_date < date_trunc('month', sqlc.arg('as_of')::date) + interval '1 month'
            THEN amount_kopecks ELSE 0 END), 0) -
        COALESCE(SUM(CASE
            WHEN type = 'expense' AND status = 'paid'
                AND operation_date >= date_trunc('month', sqlc.arg('as_of')::date)
                AND operation_date < date_trunc('month', sqlc.arg('as_of')::date) + interval '1 month'
            THEN amount_kopecks ELSE 0 END), 0)
    )::bigint AS monthly_profit_kopecks,
    COUNT(*) FILTER (WHERE status = 'overdue' AND type = 'income' AND category = 'rent') AS overdue_rent_count,
    COUNT(*) FILTER (WHERE status = 'overdue') AS overdue_total_count,
    (MIN(operation_date) FILTER (WHERE status IN ('pending', 'overdue') AND type = 'income' AND category = 'rent'))::date AS next_payment_date
FROM operations
WHERE owner_id = $1 AND property_id = $2 AND deleted_at IS NULL;

-- name: HasDepositReturnForLease :one
SELECT EXISTS(
    SELECT 1 FROM operations
    WHERE lease_id = $1
      AND type = 'expense'
      AND category = 'deposit_return'
      AND deleted_at IS NULL
) AS has_deposit_return;

-- name: ListOperationsByOwner :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name FROM operations
WHERE owner_id = $1
  AND deleted_at IS NULL
  AND (sqlc.arg('types')::text[] = '{}'::text[] OR type = ANY(sqlc.arg('types')::text[]))
  AND (sqlc.arg('statuses')::text[] = '{}'::text[] OR status = ANY(sqlc.arg('statuses')::text[]))
  AND (sqlc.arg('categories')::text[] = '{}'::text[] OR category = ANY(sqlc.arg('categories')::text[]))
  AND (sqlc.arg('property_id')::uuid IS NULL OR property_id = sqlc.arg('property_id')::uuid)
  AND (sqlc.arg('from_date')::date IS NULL OR operation_date >= sqlc.arg('from_date')::date)
  AND (sqlc.arg('to_date')::date IS NULL OR operation_date <= sqlc.arg('to_date')::date)
  AND (sqlc.arg('recurring_operation_id')::uuid IS NULL OR recurring_operation_id = sqlc.arg('recurring_operation_id')::uuid)
ORDER BY operation_date DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;
