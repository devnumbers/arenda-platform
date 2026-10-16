DROP INDEX IF EXISTS idx_subscription_payments_expired_pending;
ALTER TABLE subscription_payments
    DROP COLUMN expires_at;
