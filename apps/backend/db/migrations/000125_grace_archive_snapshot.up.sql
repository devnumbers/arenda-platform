SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Grace v2 (ADR 0055, issue #615): the snapshot of the property ids billing
-- archived when the subscription entered its grace window — it kept one active
-- property and owes the restoration of these on the next successful payment.
-- The array is written in restoration-priority order (updated_at DESC, the
-- newest archived property first). Cleared when the debt is settled (the
-- success restored them) or dropped (downgrade to basic, service assignment).
ALTER TABLE user_subscriptions
    ADD COLUMN grace_archived_property_ids UUID[] NOT NULL DEFAULT '{}';
