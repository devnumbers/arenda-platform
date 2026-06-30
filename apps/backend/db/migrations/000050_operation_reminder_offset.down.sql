DROP INDEX IF EXISTS idx_operations_reminder_offset;
ALTER TABLE operations DROP COLUMN IF EXISTS reminder_offset_days;
