ALTER TABLE reminders DROP CONSTRAINT IF EXISTS one_reminder_per_target_event;
DROP INDEX IF EXISTS idx_reminders_active_unique;
CREATE UNIQUE INDEX idx_reminders_active_unique
ON reminders (owner_id, target_type, operation_id, recurring_operation_id, lease_id, event_type)
WHERE status IN ('pending', 'sending');
