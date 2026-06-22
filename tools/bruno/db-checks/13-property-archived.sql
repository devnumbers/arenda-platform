-- Verify that the property with the given id has status 'archived'.
-- Usage: psql ... -v property_id=uuid -f 13-property-archived.sql
WITH params AS (
    SELECT :'property_id'::uuid AS property_id
)
SELECT
    (p.id IS NOT NULL AND p.status = 'archived') AS ok,
    p.id IS NOT NULL AS property_exists,
    p.status AS property_status
FROM params
LEFT JOIN properties p ON p.id = params.property_id;
