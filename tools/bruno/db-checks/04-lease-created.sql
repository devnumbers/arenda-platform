-- Verify that a lease exists with the expected property_id, status, start_date and end_date.
-- Usage: psql ... -v lease_id=uuid -v property_id=uuid -v status=active -v start_date=2026-01-01 -v end_date=2026-12-31 -f 04-lease-created.sql
WITH params AS (
    SELECT :'lease_id'::uuid AS lease_id,
           :'property_id'::uuid AS property_id,
           :'status' AS expected_status,
           :'start_date'::date AS expected_start_date,
           :'end_date'::date AS expected_end_date
)
SELECT
    (l.id IS NOT NULL
     AND l.property_id = params.property_id
     AND l.status = params.expected_status
     AND l.start_date = params.expected_start_date
     AND l.end_date = params.expected_end_date) AS ok,
    l.id IS NOT NULL AS lease_exists,
    l.status AS lease_status,
    l.start_date,
    l.end_date,
    l.property_id
FROM params
LEFT JOIN leases l ON l.id = params.lease_id;
