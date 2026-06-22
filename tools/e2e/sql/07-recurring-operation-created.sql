-- Verify that a recurring operation exists with the expected payment_day, amount and status.
-- Usage: psql ... -v recurring_operation_id=uuid -v payment_day=1 -v amount_kopecks=100000 -v status=active -f 07-recurring-operation-created.sql
WITH params AS (
    SELECT :'recurring_operation_id'::uuid AS recurring_operation_id,
           :payment_day::int AS expected_payment_day,
           :amount_kopecks::bigint AS expected_amount_kopecks,
           :'status' AS expected_status
)
SELECT
    (ro.id IS NOT NULL
     AND ro.payment_day = params.expected_payment_day
     AND ro.amount_kopecks = params.expected_amount_kopecks
     AND ro.status = params.expected_status) AS ok,
    ro.id IS NOT NULL AS recurring_operation_exists,
    ro.payment_day,
    ro.amount_kopecks,
    ro.status
FROM params
LEFT JOIN recurring_operations ro ON ro.id = params.recurring_operation_id;
