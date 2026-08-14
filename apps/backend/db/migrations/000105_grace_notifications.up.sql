-- Grace notifications (issue #253). The notifications context gains a sixth
-- event type covering both grace moments (payment failed → grace entered, and
-- the grace-expiry reminder): one preference row pair governs the billing
-- lifecycle notices (ADR 0030 opt-out model).
ALTER TYPE notification_event_type ADD VALUE IF NOT EXISTS 'subscription_grace';

-- The grace-expiry reminder worker marks each grace window as reminded exactly
-- once; entering a new grace window clears the flag (the domain's EnterGrace).
-- Nullable: NULL = the window has not been reminded yet.
ALTER TABLE user_subscriptions ADD COLUMN grace_reminded_at timestamptz;

-- Backs the reminder-window listing: grace subscriptions inside the reminder
-- window that were not reminded yet (partial index, mirrors migration 000104).
CREATE INDEX idx_user_subscriptions_grace_unreminded
    ON user_subscriptions (valid_until)
    WHERE status = 'grace' AND grace_reminded_at IS NULL;
