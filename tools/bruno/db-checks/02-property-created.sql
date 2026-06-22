-- Verify that a property with the given id and owner_id exists and has status 'active'.
-- Usage: psql ... -v property_id=f1049ccf-2a0d-45a0-afd8-a0f11978722c -v owner_id=0bfb52b4-a7b0-4505-b94a-852850e64bbd -f 02-property-created.sql
WITH params AS (
    SELECT :'property_id'::uuid AS property_id,
           :'owner_id'::uuid AS owner_id
)
SELECT
    (p.id IS NOT NULL AND p.status = 'active') AS ok,
    p.id IS NOT NULL AS property_exists,
    p.status AS property_status,
    p.name AS property_name
FROM params
LEFT JOIN properties p
       ON p.id = params.property_id
      AND p.owner_id = params.owner_id;
