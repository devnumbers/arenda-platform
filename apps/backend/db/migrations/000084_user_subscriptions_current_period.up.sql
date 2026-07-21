-- current_period stores the billing period the subscription is currently paid
-- up for. It is subscription state, set whenever a tariff change, renewal or
-- scheduled downgrade is applied, so renewal resolution does not depend on the
-- last succeeded payment (which is wrong right after a scheduled downgrade:
-- the payment still references the old tariff's period).
ALTER TABLE user_subscriptions
    ADD COLUMN current_period TEXT CHECK (current_period IN ('month', 'year'));

UPDATE user_subscriptions us SET current_period = last.period
FROM (
    SELECT DISTINCT ON (subscription_id) subscription_id, period
    FROM subscription_payments
    WHERE status = 'succeeded'
    ORDER BY subscription_id, created_at DESC
) AS last
WHERE us.id = last.subscription_id;
