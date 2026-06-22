-- Verify that there is at most one open lease per property.
-- "Open" means status IN ('awaiting_start', 'active', 'requires_action').
-- Usage: psql ... -v property_id=uuid -f 12-no-duplicate-open-lease.sql
WITH params AS (
    SELECT :'property_id'::uuid AS property_id
),
open_count AS (
    SELECT count(*)::int AS open_count
    FROM leases
    WHERE property_id = (SELECT property_id FROM params)
      AND status IN ('awaiting_start', 'active', 'requires_action')
)
SELECT
    (open_count <= 1) AS ok,
    open_count AS open_leases_count
FROM open_count;
