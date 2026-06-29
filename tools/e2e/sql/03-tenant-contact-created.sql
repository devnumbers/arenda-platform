-- Verify that a tenant contact exists for the given owner.
-- Usage: psql ... -v owner_id=uuid -f 03-tenant-contact-created.sql
WITH params AS (
    SELECT :'owner_id'::uuid AS owner_id
),
latest AS (
    SELECT tc.id, tc.name, tc.surname
    FROM tenant_contacts tc
    WHERE tc.owner_id = (SELECT owner_id FROM params)
    ORDER BY tc.created_at DESC
    LIMIT 1
)
SELECT
    (l.id IS NOT NULL) AS ok,
    l.id IS NOT NULL AS contact_exists,
    l.name AS contact_name,
    l.surname AS contact_surname
FROM params
LEFT JOIN latest l ON true;
