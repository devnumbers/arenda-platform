SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Pending-payment form TTL (issue #616): the absolute instant a
-- customer-initiated payment's payer form closes — the module's PaymentFormTTL
-- computed at initiation and passed to the provider as the same RedirectDueDate.
-- Nil on merchant-initiated charges (no payer form). The TTL worker expires
-- still-pending payments past this instant, which unlocks the tariff choice;
-- a late provider success is reconciled against the provider afterwards.
ALTER TABLE subscription_payments
    ADD COLUMN expires_at TIMESTAMPTZ;

-- The expiry batch of the TTL worker: still-pending payments past their
-- deadline, oldest deadline first.
CREATE INDEX idx_subscription_payments_expired_pending
    ON subscription_payments(expires_at)
    WHERE status = 'pending' AND expires_at IS NOT NULL;
