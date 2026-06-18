ALTER TABLE sent_sms_reminders
    DROP CONSTRAINT IF EXISTS uq_sent_sms_reminders_reminder_id;

CREATE UNIQUE INDEX idx_sent_sms_reminders_reminder_id
ON sent_sms_reminders(reminder_id)
WHERE reminder_id IS NOT NULL;
