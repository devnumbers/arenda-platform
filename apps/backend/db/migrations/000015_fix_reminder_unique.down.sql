-- NOTE: this down migration can fail if duplicate active reminders were created
-- while the partial unique index was in place. Deduplicate manually before rolling back.
DROP INDEX IF EXISTS idx_reminders_active_unique;
ALTER TABLE reminders ADD CONSTRAINT one_reminder_per_target_event
    UNIQUE (owner_id, target_type, operation_id, recurring_operation_id, lease_id, event_type);
