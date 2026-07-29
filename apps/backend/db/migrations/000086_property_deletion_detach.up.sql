-- Step 1 of property deletion: allow child records to survive property deletion
-- by changing FK actions to SET NULL for business tables. Photos remain tied
-- to the property and will still cascade.
ALTER TABLE leases
    DROP CONSTRAINT IF EXISTS leases_property_id_fkey,
    ADD CONSTRAINT leases_property_id_fkey
        FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE SET NULL;

ALTER TABLE operations
    DROP CONSTRAINT IF EXISTS operations_property_id_fkey,
    ADD CONSTRAINT operations_property_id_fkey
        FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE SET NULL;

ALTER TABLE recurring_operations
    DROP CONSTRAINT IF EXISTS recurring_operations_property_id_fkey,
    ADD CONSTRAINT recurring_operations_property_id_fkey
        FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE SET NULL;

ALTER TABLE reminders
    DROP CONSTRAINT IF EXISTS reminders_property_id_fkey,
    ADD CONSTRAINT reminders_property_id_fkey
        FOREIGN KEY (property_id) REFERENCES properties(id) ON DELETE SET NULL;

ALTER TABLE leases ALTER COLUMN property_id DROP NOT NULL;
ALTER TABLE operations ALTER COLUMN property_id DROP NOT NULL;
ALTER TABLE recurring_operations ALTER COLUMN property_id DROP NOT NULL;
