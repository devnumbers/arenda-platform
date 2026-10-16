-- Partial index for the stale-refund reconciliation worker (issue #254): the
-- refund saga leaves a payment in the internal refunding reservation while its
-- outcome is unresolved; the worker selects refunding payments whose
-- updated_at crossed the reconciliation staleness.
CREATE INDEX idx_subscription_payments_refunding_updated
    ON subscription_payments(updated_at, id)
    WHERE status = 'refunding' AND provider_payment_id IS NOT NULL;
