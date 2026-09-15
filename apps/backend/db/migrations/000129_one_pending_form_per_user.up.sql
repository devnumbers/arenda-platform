-- One pending payment form per user (issue #690, the #681 backstop): at most
-- one pending payment with a payer form per user, whatever the tariff and
-- period — the durable schema truth behind the application-level
-- one-pending-payment rule (issue #616). The same-target index below stays:
-- it remains the only schema guard for merchant-initiated same-target
-- duplicates, whose rows carry no deadline and never enter this index
-- (issue #616 invariant).
--
-- The sweep first: long-lived databases can hold dead pending forms (the TTL
-- worker has not reached them yet), and two of them for one user would break
-- the unique build. They are expired exactly the way the worker does it —
-- the server-side expiry is the truth (issue #616) — so the index builds
-- clean. Live forms and deadline-less rows are untouched; two live forms for
-- one user would fail the build loudly, and rightly: no flow can produce
-- that state.
UPDATE subscription_payments
SET status = 'failed', error_code = 'form_expired', updated_at = now()
WHERE status = 'pending'
  AND expires_at IS NOT NULL
  AND expires_at <= now();

CREATE UNIQUE INDEX idx_subscription_payments_one_pending_form
    ON subscription_payments(user_id)
    WHERE status = 'pending' AND expires_at IS NOT NULL;
