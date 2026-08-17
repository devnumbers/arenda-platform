-- The partial index depends on grace_reminded_at, so it goes first: dropping
-- the column alone would cascade-drop the index and the statement below
-- would then fail.
DROP INDEX idx_user_subscriptions_grace_unreminded;
ALTER TABLE user_subscriptions DROP COLUMN grace_reminded_at;

-- PostgreSQL cannot remove a value from an enum type; the 'subscription_grace'
-- value stays after rollback (same trade-off as migration 000089).
