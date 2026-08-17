-- Restore the per-selection partial indexes of migrations 000104, 000105 and
-- 000106 and drop the status-leading composites of 000107.

DROP INDEX idx_subscription_payments_status_updated;
DROP INDEX idx_subscription_payments_status_created;
DROP INDEX idx_user_subscriptions_status_pending_change;
DROP INDEX idx_user_subscriptions_status_valid_until;

CREATE INDEX idx_user_subscriptions_up_for_renewal
    ON user_subscriptions(valid_until, id)
    WHERE status = 'active' AND auto_renew_enabled = true;

CREATE INDEX idx_user_subscriptions_expired_grace
    ON user_subscriptions(valid_until, id)
    WHERE status = 'grace';

CREATE INDEX idx_user_subscriptions_expired_non_renewing
    ON user_subscriptions(valid_until, id)
    WHERE status = 'active' AND auto_renew_enabled = false;

CREATE INDEX idx_user_subscriptions_expired_cancelled
    ON user_subscriptions(valid_until, id)
    WHERE status = 'cancelled';

CREATE INDEX idx_user_subscriptions_pending_change
    ON user_subscriptions(pending_change_at)
    WHERE status = 'active' AND pending_tariff_id IS NOT NULL;

CREATE INDEX idx_user_subscriptions_grace_unreminded
    ON user_subscriptions (valid_until)
    WHERE status = 'grace' AND grace_reminded_at IS NULL;

CREATE INDEX idx_subscription_payments_pending_created
    ON subscription_payments(created_at)
    WHERE status = 'pending' AND provider_payment_id IS NOT NULL;

CREATE INDEX idx_subscription_payments_refunding_updated
    ON subscription_payments(updated_at, id)
    WHERE status = 'refunding' AND provider_payment_id IS NOT NULL;
