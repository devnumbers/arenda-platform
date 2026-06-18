ALTER TABLE sent_sms_reminders
    DROP CONSTRAINT IF EXISTS uq_sent_sms_reminders_reminder_id;
