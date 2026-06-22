-- Verify that the user has exactly one active payment method.
-- Usage: psql ... -v user_id=uuid -f 10-payment-method-active.sql
WITH params AS (
    SELECT :'user_id'::uuid AS user_id
),
active_count AS (
    SELECT count(*)::int AS active_count
    FROM payment_methods
    WHERE user_id = (SELECT user_id FROM params)
      AND is_active = true
)
SELECT
    (active_count = 1) AS ok,
    active_count AS active_payment_methods_count
FROM active_count;
