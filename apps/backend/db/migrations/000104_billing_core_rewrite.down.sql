-- Down migration for the billing core rewrite (issue #245, ADR 0037).
-- The rewrite is destructive: the previous billing data cannot be restored.
-- Rolling back removes the new billing tables entirely; re-applying the up
-- migration re-creates them empty with the tariff seed and re-onboards
-- existing owners to the basic plan.
ALTER TABLE user_subscriptions
    DROP CONSTRAINT IF EXISTS user_subscriptions_last_applied_payment_id_fkey;

DROP TABLE IF EXISTS card_binding_sessions;
DROP TABLE IF EXISTS subscription_transitions;
DROP TABLE IF EXISTS subscription_payments;
DROP TABLE IF EXISTS user_subscriptions;
DROP TABLE IF EXISTS payment_methods;
DROP TABLE IF EXISTS tariffs;

DROP FUNCTION IF EXISTS reject_subscription_transition_mutation();
