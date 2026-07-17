-- name: CreateOperation :one
INSERT INTO operations (
    id, owner_id, property_id, lease_id, recurring_operation_id,
    type, category_id, name, amount_kopecks, operation_date, source_operation_date, comment, is_exception, status,
    reminder_offset_days
)
VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10, $11, $12, $13, $14,
    $15
)
RETURNING *;

-- name: ListOperationsByLease :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category_id, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name, reminder_offset_days, source_operation_date FROM operations
WHERE lease_id = $1
  AND deleted_at IS NULL
ORDER BY operation_date DESC;

-- name: ListOperationDatesByLease :many
SELECT COALESCE(op.source_operation_date, op.operation_date)::date AS operation_date FROM operations op
JOIN operation_categories cat ON cat.id = op.category_id
WHERE op.lease_id = $1
  AND (op.recurring_operation_id IS NOT NULL OR cat.code = 'rent')
  AND (op.deleted_at IS NULL OR op.is_exception = true);

-- name: ListOperationDatesByRecurringOperation :many
SELECT COALESCE(source_operation_date, operation_date)::date AS operation_date FROM operations
WHERE recurring_operation_id = $1
  AND (deleted_at IS NULL OR is_exception = true);

-- name: UpdateFutureGeneratedOperationReminderOffsets :execrows
UPDATE operations
SET reminder_offset_days = $1,
    updated_at = now()
WHERE recurring_operation_id = $2
  AND owner_id = $3
  AND is_exception = false
  AND operation_date >= $4
  AND deleted_at IS NULL
  AND reminder_offset_days IS DISTINCT FROM $1;

-- name: ListOperationsByRecurringOperation :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category_id, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name, reminder_offset_days, source_operation_date FROM operations
WHERE recurring_operation_id = $1
  AND deleted_at IS NULL
ORDER BY operation_date ASC;

-- name: DeleteFutureGeneratedOperations :exec
DELETE FROM operations
WHERE recurring_operation_id = $1
  AND owner_id = $2
  AND operation_date > CURRENT_DATE
  AND status IN ('pending', 'overdue')
  AND is_exception = false
  AND deleted_at IS NULL;

-- name: DeleteUneditedFutureOperationsByRecurringOperation :exec
DELETE FROM operations
WHERE recurring_operation_id = $1
  AND is_exception = false
  AND operation_date > $2
  AND deleted_at IS NULL;

-- name: DeleteUneditedOperationsByRecurringOperation :exec
DELETE FROM operations
WHERE recurring_operation_id = $1
  AND is_exception = false
  AND operation_date >= $2
  AND deleted_at IS NULL;

-- name: ListFutureOperationsByLease :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category_id, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name, reminder_offset_days, source_operation_date FROM operations
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
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category_id, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name, reminder_offset_days, source_operation_date FROM operations
WHERE owner_id = $1 AND property_id = $2
  AND deleted_at IS NULL
ORDER BY operation_date DESC;

-- name: ListOperationsByPropertyWithStatuses :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category_id, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name, reminder_offset_days, source_operation_date FROM operations
WHERE owner_id = $1 AND property_id = $2
  AND status = ANY(sqlc.arg('statuses')::text[])
  AND deleted_at IS NULL
ORDER BY operation_date DESC;

-- name: GetOperationByIDAndOwner :one
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category_id, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name, reminder_offset_days, source_operation_date FROM operations
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL;

-- name: GetOperationByIDAndOwnerForUpdate :one
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category_id, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name, reminder_offset_days, source_operation_date FROM operations
WHERE id = $1 AND owner_id = $2
  AND deleted_at IS NULL
FOR UPDATE;

-- name: UpdateOperation :one
UPDATE operations
SET type = $3,
    category_id = $4,
    name = $5,
    amount_kopecks = $6,
    operation_date = $7,
    comment = $8,
    lease_id = $9,
    status = $10,
    reminder_offset_days = $11,
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
    updated_at = now(),
    is_exception = CASE
        WHEN operations.recurring_operation_id IS NOT NULL OR (operations.lease_id IS NOT NULL AND operations.category_id = (SELECT operation_categories.id FROM operation_categories WHERE owner_id = operations.owner_id AND code = 'rent')) THEN true
        ELSE operations.is_exception
    END
WHERE operations.id = $1 AND operations.owner_id = $2
  AND operations.deleted_at IS NULL
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
        COALESCE(SUM(CASE WHEN op.type = 'income' AND op.status = 'received' THEN op.amount_kopecks ELSE 0 END), 0) -
        COALESCE(SUM(CASE WHEN op.type = 'expense' AND op.status = 'paid' THEN op.amount_kopecks ELSE 0 END), 0)
    )::bigint AS all_time_profit_kopecks,
    COALESCE(SUM(CASE WHEN op.type = 'income' AND op.status = 'received' THEN op.amount_kopecks ELSE 0 END), 0)::bigint AS all_time_income_kopecks,
    COALESCE(SUM(CASE WHEN op.type = 'expense' AND op.status = 'paid' THEN op.amount_kopecks ELSE 0 END), 0)::bigint AS all_time_expense_kopecks,
    (
        COALESCE(SUM(CASE
            WHEN op.type = 'income' AND op.status = 'received'
                AND op.operation_date >= date_trunc('month', sqlc.arg('as_of')::date)
                AND op.operation_date < date_trunc('month', sqlc.arg('as_of')::date) + interval '1 month'
            THEN op.amount_kopecks ELSE 0 END), 0) -
        COALESCE(SUM(CASE
            WHEN op.type = 'expense' AND op.status = 'paid'
                AND op.operation_date >= date_trunc('month', sqlc.arg('as_of')::date)
                AND op.operation_date < date_trunc('month', sqlc.arg('as_of')::date) + interval '1 month'
            THEN op.amount_kopecks ELSE 0 END), 0)
    )::bigint AS monthly_profit_kopecks,
    COUNT(*) FILTER (WHERE op.status = 'overdue' AND op.type = 'income' AND cat.code = 'rent') AS overdue_rent_count,
    COUNT(*) FILTER (WHERE op.status = 'overdue') AS overdue_total_count,
    (MIN(op.operation_date) FILTER (WHERE op.status IN ('pending', 'overdue') AND op.type = 'income' AND cat.code = 'rent'))::date AS next_payment_date
FROM operations op
JOIN operation_categories cat ON cat.id = op.category_id
WHERE op.owner_id = $1 AND op.property_id = $2 AND op.deleted_at IS NULL;

-- name: ListOverdueRentOperationsByOwner :many
SELECT op.lease_id, op.operation_date
FROM operations op
JOIN operation_categories cat ON cat.id = op.category_id
WHERE op.owner_id = sqlc.arg('owner_id')::uuid
  AND op.status = 'overdue'
  AND op.type = 'income'
  AND cat.code = 'rent'
  AND op.lease_id IS NOT NULL
  AND op.deleted_at IS NULL
ORDER BY op.lease_id, op.operation_date;

-- name: ListNextRentPaymentsByOwner :many
SELECT op.lease_id, MIN(op.operation_date)::date AS next_payment_date
FROM operations op
JOIN operation_categories cat ON cat.id = op.category_id
WHERE op.owner_id = sqlc.arg('owner_id')::uuid
  AND op.status = 'pending'
  AND op.operation_date >= sqlc.arg('as_of')::date
  AND op.type = 'income'
  AND cat.code = 'rent'
  AND op.lease_id IS NOT NULL
  AND op.deleted_at IS NULL
GROUP BY op.lease_id;

-- name: ListOperationsByOwner :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category_id, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name, reminder_offset_days, source_operation_date FROM operations
WHERE operations.owner_id = $1
  AND deleted_at IS NULL
  AND (COALESCE(sqlc.arg('types')::text[], '{}') = '{}'::text[] OR type = ANY(COALESCE(sqlc.arg('types')::text[], '{}')))
  AND (COALESCE(sqlc.arg('statuses')::text[], '{}') = '{}'::text[] OR status = ANY(COALESCE(sqlc.arg('statuses')::text[], '{}')))
  AND (COALESCE(sqlc.arg('category_ids')::uuid[], '{}') = '{}'::uuid[] OR category_id = ANY(COALESCE(sqlc.arg('category_ids')::uuid[], '{}')))
  AND (sqlc.arg('property_id')::uuid IS NULL OR property_id = sqlc.arg('property_id')::uuid)
  AND (sqlc.arg('from_date')::date IS NULL OR operation_date >= sqlc.arg('from_date')::date)
  AND (sqlc.arg('to_date')::date IS NULL OR operation_date <= sqlc.arg('to_date')::date)
  AND (sqlc.arg('recurring_operation_id')::uuid IS NULL OR recurring_operation_id = sqlc.arg('recurring_operation_id')::uuid)
  AND (sqlc.arg('lease_id')::uuid IS NULL OR lease_id = sqlc.arg('lease_id')::uuid)
  AND (sqlc.arg('exclude_archived_properties')::bool = false OR NOT EXISTS (SELECT 1 FROM properties p WHERE p.id = operations.property_id AND p.status = 'archived'))
ORDER BY operation_date DESC, id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: ListOperationsByOwnerAsc :many
SELECT id, owner_id, property_id, lease_id, recurring_operation_id, type, category_id, amount_kopecks, operation_date, comment, is_exception, created_at, updated_at, deleted_at, status, name, reminder_offset_days, source_operation_date FROM operations
WHERE operations.owner_id = $1
  AND deleted_at IS NULL
  AND (COALESCE(sqlc.arg('types')::text[], '{}') = '{}'::text[] OR type = ANY(COALESCE(sqlc.arg('types')::text[], '{}')))
  AND (COALESCE(sqlc.arg('statuses')::text[], '{}') = '{}'::text[] OR status = ANY(COALESCE(sqlc.arg('statuses')::text[], '{}')))
  AND (COALESCE(sqlc.arg('category_ids')::uuid[], '{}') = '{}'::uuid[] OR category_id = ANY(COALESCE(sqlc.arg('category_ids')::uuid[], '{}')))
  AND (sqlc.arg('property_id')::uuid IS NULL OR property_id = sqlc.arg('property_id')::uuid)
  AND (sqlc.arg('from_date')::date IS NULL OR operation_date >= sqlc.arg('from_date')::date)
  AND (sqlc.arg('to_date')::date IS NULL OR operation_date <= sqlc.arg('to_date')::date)
  AND (sqlc.arg('recurring_operation_id')::uuid IS NULL OR recurring_operation_id = sqlc.arg('recurring_operation_id')::uuid)
  AND (sqlc.arg('lease_id')::uuid IS NULL OR lease_id = sqlc.arg('lease_id')::uuid)
  AND (sqlc.arg('exclude_archived_properties')::bool = false OR NOT EXISTS (SELECT 1 FROM properties p WHERE p.id = operations.property_id AND p.status = 'archived'))
ORDER BY operation_date ASC, id ASC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: ListOperationsAdmin :many
SELECT op.*, cat.name AS category_name, p.name AS property_name
FROM operations op
JOIN operation_categories cat ON cat.id = op.category_id
JOIN properties p ON p.id = op.property_id
WHERE (sqlc.arg('owner_id')::uuid IS NULL OR op.owner_id = sqlc.arg('owner_id')::uuid)
  AND op.deleted_at IS NULL
  AND (sqlc.arg('status')::text = '' OR op.status = sqlc.arg('status')::text)
  AND (sqlc.arg('type')::text = '' OR op.type = sqlc.arg('type')::text)
  AND (sqlc.arg('property_id')::uuid IS NULL OR op.property_id = sqlc.arg('property_id')::uuid)
  AND (sqlc.arg('lease_id')::uuid IS NULL OR op.lease_id = sqlc.arg('lease_id')::uuid)
  AND (sqlc.arg('q')::text = '' OR op.name ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\' OR op.comment ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\')
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'operationDate' AND sqlc.arg('order')::text = 'asc' THEN op.operation_date END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'operationDate' AND sqlc.arg('order')::text = 'desc' THEN op.operation_date END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'amountKopecks' AND sqlc.arg('order')::text = 'asc' THEN op.amount_kopecks END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'amountKopecks' AND sqlc.arg('order')::text = 'desc' THEN op.amount_kopecks END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'status' AND sqlc.arg('order')::text = 'asc' THEN op.status END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'status' AND sqlc.arg('order')::text = 'desc' THEN op.status END DESC,
  CASE WHEN sqlc.arg('sort')::text = '' THEN op.operation_date END DESC,
  op.id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountOperationsAdmin :one
SELECT COUNT(*) FROM operations op
JOIN operation_categories cat ON cat.id = op.category_id
WHERE (sqlc.arg('owner_id')::uuid IS NULL OR op.owner_id = sqlc.arg('owner_id')::uuid)
  AND op.deleted_at IS NULL
  AND (sqlc.arg('status')::text = '' OR op.status = sqlc.arg('status')::text)
  AND (sqlc.arg('type')::text = '' OR op.type = sqlc.arg('type')::text)
  AND (sqlc.arg('property_id')::uuid IS NULL OR op.property_id = sqlc.arg('property_id')::uuid)
  AND (sqlc.arg('lease_id')::uuid IS NULL OR op.lease_id = sqlc.arg('lease_id')::uuid)
  AND (sqlc.arg('q')::text = '' OR op.name ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\' OR op.comment ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\');

-- name: GetOperationByIDAdmin :one
SELECT op.*, cat.name AS category_name, p.name AS property_name
FROM operations op
JOIN operation_categories cat ON cat.id = op.category_id
JOIN properties p ON p.id = op.property_id
WHERE op.id = $1;

-- name: CountOperationsTotalAdmin :one
SELECT COUNT(*) FROM operations WHERE deleted_at IS NULL;
