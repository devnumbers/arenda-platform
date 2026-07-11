-- name: GetFinanceReportTotals :one
SELECT
  COALESCE(SUM(CASE WHEN type = 'income' THEN amount_kopecks ELSE 0 END), 0)::bigint AS income_kopecks,
  COALESCE(SUM(CASE WHEN type = 'expense' THEN amount_kopecks ELSE 0 END), 0)::bigint AS expense_kopecks
FROM operations
WHERE owner_id = sqlc.arg('owner_id')::uuid
  AND status IN ('paid', 'received')
  AND (sqlc.arg('from_date')::date IS NULL OR operation_date >= sqlc.arg('from_date')::date)
  AND (sqlc.arg('to_date')::date IS NULL OR operation_date <= sqlc.arg('to_date')::date)
  AND deleted_at IS NULL;

-- name: GetFinanceReportByProperty :many
SELECT
  property_id,
  COALESCE(SUM(CASE WHEN type = 'income' THEN amount_kopecks ELSE 0 END), 0)::bigint AS income_kopecks,
  COALESCE(SUM(CASE WHEN type = 'expense' THEN amount_kopecks ELSE 0 END), 0)::bigint AS expense_kopecks
FROM operations
WHERE owner_id = sqlc.arg('owner_id')::uuid
  AND status IN ('paid', 'received')
  AND (sqlc.arg('from_date')::date IS NULL OR operation_date >= sqlc.arg('from_date')::date)
  AND (sqlc.arg('to_date')::date IS NULL OR operation_date <= sqlc.arg('to_date')::date)
  AND deleted_at IS NULL
GROUP BY property_id
ORDER BY (
  COALESCE(SUM(CASE WHEN type = 'income' THEN amount_kopecks ELSE 0 END), 0) -
  COALESCE(SUM(CASE WHEN type = 'expense' THEN amount_kopecks ELSE 0 END), 0)
) DESC;

-- name: GetFinanceReportByCategory :many
SELECT
  type,
  category,
  COALESCE(SUM(amount_kopecks), 0)::bigint AS total_kopecks
FROM operations
WHERE owner_id = sqlc.arg('owner_id')::uuid
  AND status IN ('paid', 'received')
  AND (sqlc.arg('from_date')::date IS NULL OR operation_date >= sqlc.arg('from_date')::date)
  AND (sqlc.arg('to_date')::date IS NULL OR operation_date <= sqlc.arg('to_date')::date)
  AND deleted_at IS NULL
GROUP BY type, category
ORDER BY total_kopecks DESC;

-- name: GetFinanceReportByMonth :many
SELECT
  date_trunc('month', operation_date)::date AS month,
  COALESCE(SUM(CASE WHEN type = 'income' THEN amount_kopecks ELSE 0 END), 0)::bigint AS income_kopecks,
  COALESCE(SUM(CASE WHEN type = 'expense' THEN amount_kopecks ELSE 0 END), 0)::bigint AS expense_kopecks
FROM operations
WHERE owner_id = sqlc.arg('owner_id')::uuid
  AND status IN ('paid', 'received')
  AND (sqlc.arg('from_date')::date IS NULL OR operation_date >= sqlc.arg('from_date')::date)
  AND (sqlc.arg('to_date')::date IS NULL OR operation_date <= sqlc.arg('to_date')::date)
  AND deleted_at IS NULL
GROUP BY month
ORDER BY month;
