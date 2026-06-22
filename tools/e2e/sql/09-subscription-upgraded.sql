-- Verify that the user subscription has the expected tariff/status and a matching payment exists.
-- Usage: psql ... -v user_id=uuid -v tariff_id=uuid -v subscription_status=active -v payment_status=succeeded -f 09-subscription-upgraded.sql
WITH params AS (
    SELECT :'user_id'::uuid AS user_id,
           :'tariff_id'::uuid AS expected_tariff_id,
           :'subscription_status' AS expected_subscription_status,
           :'payment_status' AS expected_payment_status
)
SELECT
    (EXISTS (
         SELECT 1
         FROM user_subscriptions us
         WHERE us.user_id = params.user_id
           AND us.tariff_id = params.expected_tariff_id
           AND us.status = params.expected_subscription_status
     )
     AND EXISTS (
         SELECT 1
         FROM subscription_payments sp
         WHERE sp.user_id = params.user_id
           AND sp.tariff_id = params.expected_tariff_id
           AND sp.status = params.expected_payment_status
     )) AS ok,
    (SELECT count(*)::int
     FROM user_subscriptions us
     WHERE us.user_id = params.user_id
       AND us.tariff_id = params.expected_tariff_id
       AND us.status = params.expected_subscription_status) AS matching_subscriptions_count,
    (SELECT count(*)::int
     FROM subscription_payments sp
     WHERE sp.user_id = params.user_id
       AND sp.tariff_id = params.expected_tariff_id
       AND sp.status = params.expected_payment_status) AS matching_payments_count
FROM params;
