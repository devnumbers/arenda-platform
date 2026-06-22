-- Verify that the recurring operation with the given id has status 'paused'.
-- Usage: psql ... -v recurring_operation_id=uuid -f 08-recurring-operation-paused.sql
WITH params AS (
    SELECT :'recurring_operation_id'::uuid AS recurring_operation_id
)
SELECT
    (ro.id IS NOT NULL AND ro.status = 'paused') AS ok,
    ro.id IS NOT NULL AS recurring_operation_exists,
    ro.status AS recurring_operation_status
FROM params
LEFT JOIN recurring_operations ro ON ro.id = params.recurring_operation_id;
