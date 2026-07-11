ALTER TABLE user_subscriptions DROP CONSTRAINT user_subscriptions_status_check;
ALTER TABLE user_subscriptions ADD CONSTRAINT user_subscriptions_status_check CHECK (status IN ('active','grace','blocked','cancelled'));
