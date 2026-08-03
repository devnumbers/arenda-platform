-- Revert 000091: drop the free_reminder_id column and restore the original
-- exactly_one_target constraint without the 'free' branch.
ALTER TABLE reminders DROP CONSTRAINT IF EXISTS exactly_one_target;
ALTER TABLE reminders ADD CONSTRAINT exactly_one_target CHECK (
    (target_type = 'operation' AND operation_id IS NOT NULL AND lease_id IS NULL) OR
    (target_type = 'recurring_operation' AND recurring_operation_id IS NOT NULL AND operation_id IS NULL AND lease_id IS NULL) OR
    (target_type = 'lease' AND lease_id IS NOT NULL AND operation_id IS NULL)
);

DROP INDEX IF EXISTS idx_reminders_free_reminder;

ALTER TABLE reminders DROP COLUMN IF EXISTS free_reminder_id;
