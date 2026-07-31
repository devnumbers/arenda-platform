-- Reverting this migration requires that no detached (property_id IS NULL)
-- rows exist, because the columns were originally NOT NULL. Resolve or delete
-- such rows before running the down migration.
ALTER TABLE leases ALTER COLUMN property_id SET NOT NULL;
ALTER TABLE operations ALTER COLUMN property_id SET NOT NULL;
ALTER TABLE recurring_operations ALTER COLUMN property_id SET NOT NULL;

ALTER TABLE reminders
    DROP CONSTRAINT IF EXISTS reminders_property_id_fkey,
    ADD CONSTRAINT reminders_property_id_fkey
        FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE CASCADE;

ALTER TABLE recurring_operations
    DROP CONSTRAINT IF EXISTS recurring_operations_property_id_fkey,
    ADD CONSTRAINT recurring_operations_property_id_fkey
        FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE CASCADE;

ALTER TABLE operations
    DROP CONSTRAINT IF EXISTS operations_property_id_fkey,
    ADD CONSTRAINT operations_property_id_fkey
        FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE CASCADE;

ALTER TABLE leases
    DROP CONSTRAINT IF EXISTS leases_property_id_fkey,
    ADD CONSTRAINT leases_property_id_fkey
        FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE CASCADE;
