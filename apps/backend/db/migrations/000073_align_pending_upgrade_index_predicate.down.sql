DROP INDEX IF EXISTS idx_subscription_payments_pending_created;

CREATE INDEX IF NOT EXISTS idx_subscription_payments_pending_created
ON subscription_payments (created_at)
WHERE status = 'pending' AND provider_payment_id IS NOT NULL AND provider_payment_id <> '';
