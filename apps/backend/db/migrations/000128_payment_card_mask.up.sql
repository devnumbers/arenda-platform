SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Card snapshot of a subscription payment (issue #619): the masked card
-- number the payment was charged with, taken at creation (the
-- merchant-initiated method being charged) or at finalization (the card the
-- provider notification reports). The snapshot is what the payment history
-- shows, so it survives payment-method deletion (payment_method_id is ON
-- DELETE SET NULL); nil for payments no card is known for.
ALTER TABLE subscription_payments
    ADD COLUMN card_mask TEXT;
