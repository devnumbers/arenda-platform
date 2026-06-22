-- Verify that a tenant contact exists for the given owner and phone.
-- Usage: psql ... -v owner_id=uuid -v phone=+79150000000 -f 03-tenant-contact-created.sql
WITH params AS (
    SELECT :'owner_id'::uuid AS owner_id,
           :'phone' AS phone
)
SELECT
    (tc.id IS NOT NULL) AS ok,
    tc.id IS NOT NULL AS contact_exists,
    tc.name AS contact_name,
    tc.surname AS contact_surname
FROM params
LEFT JOIN tenant_contacts tc
       ON tc.owner_id = params.owner_id
      AND tc.phone = params.phone;
