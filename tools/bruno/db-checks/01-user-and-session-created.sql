-- Verify that after verify-code a user and at least one session exist for the given phone.
-- Usage: psql ... -v phone=+79150000000 -f 01-user-and-session-created.sql
WITH params AS (
    SELECT :'phone' AS phone
)
SELECT
    (u.id IS NOT NULL AND s.cnt > 0) AS ok,
    u.id IS NOT NULL AS user_exists,
    COALESCE(s.cnt, 0) AS session_count,
    u.id AS user_id
FROM params
LEFT JOIN users u ON u.phone = params.phone
LEFT JOIN LATERAL (
    SELECT count(*)::int AS cnt
    FROM sessions
    WHERE user_id = u.id
) s ON true;
