-- Composite worker-selection indexes (issue #286): the worker batch listings
-- became one parameterized query per aggregate (ListSubscriptionsBySelection,
-- ListSubscriptionPaymentsBySelection), and the planner cannot prove a
-- partial index's constant predicate — status = 'active', auto_renew_enabled,
-- provider_payment_id IS NOT NULL — from a runtime parameter, so the
-- per-selection partial indexes of migrations 000104, 000105 and 000106 would
-- silently stop matching generic plans. Status-leading composites back every
-- selection instead: the equality prefix narrows to the phase's status, the
-- second column serves the phase-clock range and the batch order.

CREATE INDEX idx_user_subscriptions_status_valid_until
    ON user_subscriptions (status, valid_until, id);

CREATE INDEX idx_user_subscriptions_status_pending_change
    ON user_subscriptions (status, pending_change_at, id);

CREATE INDEX idx_subscription_payments_status_created
    ON subscription_payments (status, created_at, id);

CREATE INDEX idx_subscription_payments_status_updated
    ON subscription_payments (status, updated_at, id);

DROP INDEX idx_user_subscriptions_up_for_renewal;
DROP INDEX idx_user_subscriptions_expired_grace;
DROP INDEX idx_user_subscriptions_expired_non_renewing;
DROP INDEX idx_user_subscriptions_expired_cancelled;
DROP INDEX idx_user_subscriptions_pending_change;
DROP INDEX idx_user_subscriptions_grace_unreminded;
DROP INDEX idx_subscription_payments_pending_created;
DROP INDEX idx_subscription_payments_refunding_updated;
