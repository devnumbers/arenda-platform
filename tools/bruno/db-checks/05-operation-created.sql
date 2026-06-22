-- Verify that an operation exists with the expected type, category, amount and date.
-- Usage: psql ... -v operation_id=uuid -v type=income -v category=rent -v amount_kopecks=100000 -v operation_date=2026-06-01 -f 05-operation-created.sql
WITH params AS (
    SELECT :'operation_id'::uuid AS operation_id,
           :'type' AS expected_type,
           :'category' AS expected_category,
           :amount_kopecks::bigint AS expected_amount_kopecks,
           :'operation_date'::date AS expected_operation_date
)
SELECT
    (o.id IS NOT NULL
     AND o.type = params.expected_type
     AND o.category = params.expected_category
     AND o.amount_kopecks = params.expected_amount_kopecks
     AND o.operation_date = params.expected_operation_date) AS ok,
    o.id IS NOT NULL AS operation_exists,
    o.type,
    o.category,
    o.amount_kopecks,
    o.operation_date,
    o.deleted_at
FROM params
LEFT JOIN operations o ON o.id = params.operation_id;
