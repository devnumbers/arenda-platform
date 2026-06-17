ALTER TABLE reminders DROP CONSTRAINT IF EXISTS exactly_one_target;
ALTER TABLE reminders ADD CONSTRAINT exactly_one_target CHECK (
    (target_type = 'operation' AND operation_id IS NOT NULL AND recurring_operation_id IS NULL AND lease_id IS NULL) OR
    (target_type = 'recurring_operation' AND recurring_operation_id IS NOT NULL AND operation_id IS NULL AND lease_id IS NULL) OR
    (target_type = 'lease' AND lease_id IS NOT NULL AND operation_id IS NULL AND recurring_operation_id IS NULL)
);
