-- Verify that at least one reminder was created for the given owner.
-- Reminders may be cancelled by subsequent cleanup, so we check any status.
-- Usage: psql ... -v user_id=uuid -f 11-reminder-created.sql
WITH params AS (
    SELECT :'user_id'::uuid AS user_id
)
SELECT
    EXISTS (
        SELECT 1
        FROM reminders r
        WHERE r.owner_id = params.user_id
    ) AS ok,
    (SELECT count(*)::int
     FROM reminders
     WHERE owner_id = params.user_id) AS reminders_count,
    (SELECT count(*)::int
     FROM reminders
     WHERE owner_id = params.user_id
       AND status::text = 'pending') AS pending_count
FROM params;
