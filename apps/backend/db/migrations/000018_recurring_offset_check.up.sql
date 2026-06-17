ALTER TABLE recurring_operations ADD CONSTRAINT chk_recurring_offset_nonnegative CHECK (reminder_offset_days >= 0);
