-- Indexes and constraints for subscription lifecycle and payment tables.

-- Worker batch queries on user_subscriptions.
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_up_for_renewal
    ON user_subscriptions(valid_until, id)
    WHERE status = 'active' AND auto_renew_enabled = true;

CREATE INDEX IF NOT EXISTS idx_user_subscriptions_expired_grace
    ON user_subscriptions(valid_until, id)
    WHERE status = 'grace';

CREATE INDEX IF NOT EXISTS idx_user_subscriptions_expired_non_renewing
    ON user_subscriptions(valid_until, id)
    WHERE status = 'active' AND auto_renew_enabled = false;

CREATE INDEX IF NOT EXISTS idx_user_subscriptions_expired_cancelled
    ON user_subscriptions(valid_until, id)
    WHERE status = 'cancelled';

-- Foreign-key helper indexes.
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_tariff_id
    ON user_subscriptions(tariff_id);

CREATE INDEX IF NOT EXISTS idx_user_subscriptions_pending_tariff_id
    ON user_subscriptions(pending_tariff_id);

CREATE INDEX IF NOT EXISTS idx_subscription_payments_tariff_id
    ON subscription_payments(tariff_id);

CREATE INDEX IF NOT EXISTS idx_subscription_payments_payment_method_id
    ON subscription_payments(payment_method_id);

-- Last succeeded payment lookup and cascading deletes.
CREATE INDEX IF NOT EXISTS idx_subscription_payments_subscription_succeeded
    ON subscription_payments(subscription_id, created_at DESC)
    WHERE status = 'succeeded';

-- Pending upgrade payment deduplication.
CREATE INDEX IF NOT EXISTS idx_subscription_payments_user_pending_created
    ON subscription_payments(user_id, created_at DESC)
    WHERE status = 'pending';

-- Provider payment identifiers must be unique per provider.
CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_payments_provider_payment
    ON subscription_payments(provider, provider_payment_id);

-- Restrict provider values at the database level.
ALTER TABLE payment_methods
    ADD CONSTRAINT payment_methods_provider_check
        CHECK (provider IN ('fake', 'tkassa'));

-- Ensure scheduled downgrades do not precede the paid period.
ALTER TABLE user_subscriptions
    ADD CONSTRAINT user_subscriptions_pending_change_at_check
        CHECK (pending_change_at IS NULL OR valid_until IS NULL OR pending_change_at >= valid_until);

-- Auto-maintain updated_at on billing tables.
CREATE TRIGGER trg_payment_methods_updated_at
    BEFORE UPDATE ON payment_methods
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_subscription_payments_updated_at
    BEFORE UPDATE ON subscription_payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
