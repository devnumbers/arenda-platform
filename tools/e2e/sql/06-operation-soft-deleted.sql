-- Verify that the operation with the given id has deleted_at IS NOT NULL.
-- Usage: psql ... -v operation_id=uuid -f 06-operation-soft-deleted.sql
WITH params AS (
    SELECT :'operation_id'::uuid AS operation_id
)
SELECT
    (o.id IS NOT NULL AND o.deleted_at IS NOT NULL) AS ok,
    o.id IS NOT NULL AS operation_exists,
    o.deleted_at IS NOT NULL AS is_deleted,
    o.deleted_at
FROM params
LEFT JOIN operations o ON o.id = params.operation_id;
