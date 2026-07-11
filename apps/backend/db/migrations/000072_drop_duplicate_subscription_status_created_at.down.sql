CREATE INDEX IF NOT EXISTS idx_subscription_payments_subscription_status_created_at
ON subscription_payments (subscription_id, status, created_at DESC);
