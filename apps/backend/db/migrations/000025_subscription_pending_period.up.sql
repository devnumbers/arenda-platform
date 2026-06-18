ALTER TABLE user_subscriptions ADD COLUMN pending_period TEXT CHECK (pending_period IN ('month', 'year'));
