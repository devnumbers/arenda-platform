-- Roll back billing indexes and constraints.

DROP TRIGGER IF EXISTS trg_subscription_payments_updated_at ON subscription_payments;
DROP TRIGGER IF EXISTS trg_payment_methods_updated_at ON payment_methods;

ALTER TABLE user_subscriptions
    DROP CONSTRAINT IF EXISTS user_subscriptions_pending_change_at_check;

ALTER TABLE payment_methods
    DROP CONSTRAINT IF EXISTS payment_methods_provider_check;

DROP INDEX IF EXISTS idx_subscription_payments_provider_payment;
DROP INDEX IF EXISTS idx_subscription_payments_user_pending_created;
DROP INDEX IF EXISTS idx_subscription_payments_subscription_succeeded;
DROP INDEX IF EXISTS idx_subscription_payments_payment_method_id;
DROP INDEX IF EXISTS idx_subscription_payments_tariff_id;
DROP INDEX IF EXISTS idx_user_subscriptions_pending_tariff_id;
DROP INDEX IF EXISTS idx_user_subscriptions_tariff_id;
DROP INDEX IF EXISTS idx_user_subscriptions_expired_cancelled;
DROP INDEX IF EXISTS idx_user_subscriptions_expired_non_renewing;
DROP INDEX IF EXISTS idx_user_subscriptions_expired_grace;
DROP INDEX IF EXISTS idx_user_subscriptions_up_for_renewal;
