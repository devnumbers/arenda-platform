DROP INDEX IF EXISTS idx_user_subscriptions_active_payment_method_id;

ALTER TABLE user_subscriptions
    DROP CONSTRAINT IF EXISTS user_subscriptions_active_payment_method_id_fkey;

DROP INDEX IF EXISTS idx_subscription_payments_provider_payment_id;

DROP INDEX IF EXISTS idx_subscription_payments_user_created;
DROP INDEX IF EXISTS idx_subscription_payments_one_pending_upgrade;
DROP TABLE IF EXISTS subscription_payments;

DROP INDEX IF EXISTS idx_payment_methods_one_active_per_user;
DROP INDEX IF EXISTS idx_payment_methods_user_active;
DROP TABLE IF EXISTS payment_methods;
