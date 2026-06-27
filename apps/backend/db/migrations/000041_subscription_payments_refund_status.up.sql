ALTER TABLE subscription_payments
    DROP CONSTRAINT IF EXISTS subscription_payments_status_check;

ALTER TABLE subscription_payments
    ADD CONSTRAINT subscription_payments_status_check
        CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded', 'partial_refunded'));
