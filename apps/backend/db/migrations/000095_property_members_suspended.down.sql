-- Reverse migration 000095: drop the suspended membership state.
--
-- Restores the property_members table to its 000094 shape: a single full
-- UNIQUE constraint on (property_id, user_id) and no status/suspended_at
-- columns. Re-adding the full UNIQUE may fail if the table currently contains
-- a suspended row alongside an active row for the same (property, user) pair;
-- callers rolling back from a state that exercised the partial index should
-- reconcile those rows first.

DROP INDEX IF EXISTS idx_property_members_user_suspended;
DROP INDEX IF EXISTS property_members_active_property_user_uniq;

ALTER TABLE property_members
    ADD CONSTRAINT property_members_property_id_user_id_key UNIQUE (property_id, user_id);

ALTER TABLE property_members
    DROP COLUMN suspended_at,
    DROP COLUMN status;
