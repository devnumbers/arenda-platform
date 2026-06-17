CREATE UNIQUE INDEX idx_sent_sms_reminders_reminder_id
ON sent_sms_reminders(reminder_id)
WHERE reminder_id IS NOT NULL;
