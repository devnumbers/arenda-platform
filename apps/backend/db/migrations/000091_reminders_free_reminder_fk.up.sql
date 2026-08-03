-- Link concrete reminder rows to their free-reminder template and allow a
-- 'free' target in the exactly-one-target constraint. ON DELETE CASCADE means
-- deleting a free-reminder template removes its concrete reminders.
ALTER TABLE reminders
    ADD COLUMN IF NOT EXISTS free_reminder_id uuid REFERENCES free_reminders (id) ON DELETE CASCADE;

ALTER TABLE reminders DROP CONSTRAINT IF EXISTS exactly_one_target;
ALTER TABLE reminders ADD CONSTRAINT exactly_one_target CHECK (
    (target_type = 'operation' AND operation_id IS NOT NULL AND lease_id IS NULL) OR
    (target_type = 'recurring_operation' AND recurring_operation_id IS NOT NULL AND operation_id IS NULL AND lease_id IS NULL) OR
    (target_type = 'lease' AND lease_id IS NOT NULL AND operation_id IS NULL) OR
    (target_type = 'free' AND free_reminder_id IS NOT NULL AND operation_id IS NULL AND recurring_operation_id IS NULL AND lease_id IS NULL)
);

CREATE INDEX IF NOT EXISTS idx_reminders_free_reminder ON reminders (free_reminder_id);
