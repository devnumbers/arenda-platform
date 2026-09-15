-- Cancel keep choice (issue #617): the property the owner chose to keep when
-- cancelling — the expiry worker keeps it alive when the cancelled
-- subscription falls to the basic tariff and archives the excess ones. Nil
-- for every other path; consumed (cleared) by the first lifecycle move that
-- makes the choice moot.
-- Deliberately no FK: the choice is a validated hint, not a durable
-- invariant — a stale or dangling id (the property archived or deleted after
-- the cancel) must not block the property's own lifecycle; the expiry
-- enforcement falls back to the default survivor rule.
ALTER TABLE user_subscriptions
    ADD COLUMN keep_property_id UUID;
