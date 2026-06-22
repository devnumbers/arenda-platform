-- Verify that the active property count for the user does not exceed their tariff limit.
-- A limit of -1 means unlimited.
-- Usage: psql ... -v user_id=uuid -f 14-tariff-limit-not-exceeded.sql
WITH params AS (
    SELECT :'user_id'::uuid AS user_id
),
active_properties AS (
    SELECT count(*)::int AS active_property_count
    FROM properties
    WHERE owner_id = (SELECT user_id FROM params)
      AND status = 'active'
),
current_tariff AS (
    SELECT t.active_property_limit
    FROM user_subscriptions us
    JOIN tariffs t ON t.id = us.tariff_id
    WHERE us.user_id = (SELECT user_id FROM params)
    ORDER BY us.created_at DESC
    LIMIT 1
)
SELECT
    (ct.active_property_limit IS NOT NULL
     AND (ap.active_property_count <= ct.active_property_limit
          OR ct.active_property_limit = -1)) AS ok,
    ap.active_property_count,
    ct.active_property_limit
FROM active_properties ap
LEFT JOIN current_tariff ct ON true;
