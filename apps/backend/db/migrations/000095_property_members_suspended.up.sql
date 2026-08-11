-- Add the suspended membership state (T4, issue #158).
--
-- A membership is the lifecycle state of a recipient on an object, not a state
-- of the object itself: when a recipient's tariff slot limit is exceeded, the
-- membership becomes 'suspended' and the object is hidden from that recipient
-- (no slot occupied, no access), rather than being revoked outright. Suspended
-- memberships are recovered FIFO when a slot frees up. See PRD #153.
--
-- Schema change over the property_members table from migration 000094:
--   * Replace the inline full UNIQUE (property_id, user_id) with a partial
--     unique index constrained to active rows, so a suspended membership does
--     not block a future active grant for the same (property, user) pair.
--   * Index suspended memberships per user for FIFO recovery lookup.
--
-- No backfill is needed: status defaults to 'active', so every existing row
-- keeps occupying its slot exactly as before. See ADR 0028 (object data access
-- model).

ALTER TABLE property_members
    ADD COLUMN status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'suspended')),
    ADD COLUMN suspended_at TIMESTAMPTZ;

ALTER TABLE property_members
    DROP CONSTRAINT property_members_property_id_user_id_key;

CREATE UNIQUE INDEX property_members_active_property_user_uniq
    ON property_members(property_id, user_id)
    WHERE status = 'active';

CREATE INDEX idx_property_members_user_suspended
    ON property_members(user_id)
    WHERE status = 'suspended';
