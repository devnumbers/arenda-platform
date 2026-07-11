ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS last_applied_payment_id UUID;
