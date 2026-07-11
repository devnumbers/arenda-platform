CREATE INDEX IF NOT EXISTS idx_user_subscriptions_pending_change
ON user_subscriptions (pending_change_at)
WHERE status = 'active' AND pending_tariff_id IS NOT NULL;
