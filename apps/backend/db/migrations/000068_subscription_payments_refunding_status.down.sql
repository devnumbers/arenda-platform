-- pre-condition guard: refunding is an internal in-flight state that must be
-- resolved before the status check constraint is narrowed back.
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM subscription_payments WHERE status = 'refunding') THEN
    RAISE EXCEPTION 'migration 000068 down aborted: subscription_payments still has status=refunding rows';
  END IF;
END $$;

ALTER TABLE subscription_payments
    DROP CONSTRAINT IF EXISTS subscription_payments_status_check;

ALTER TABLE subscription_payments
    ADD CONSTRAINT subscription_payments_status_check
        CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded', 'partial_refunded'));
