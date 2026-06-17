DROP INDEX IF EXISTS idx_reminders_active_unique;
ALTER TABLE reminders ADD CONSTRAINT one_reminder_per_target_event
    UNIQUE (owner_id, target_type, operation_id, recurring_operation_id, lease_id, event_type);
