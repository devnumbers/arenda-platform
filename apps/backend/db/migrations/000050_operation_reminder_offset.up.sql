ALTER TABLE operations ADD COLUMN reminder_offset_days INT CHECK (reminder_offset_days IN (1, 3, 7));
CREATE INDEX idx_operations_reminder_offset ON operations(reminder_offset_days) WHERE reminder_offset_days IS NOT NULL;
