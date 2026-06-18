ALTER TABLE sent_sms_reminders
    ADD CONSTRAINT uq_sent_sms_reminders_reminder_id UNIQUE (reminder_id);
