DROP INDEX IF EXISTS idx_sent_sms_reminders_reminder_id;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uq_sent_sms_reminders_reminder_id'
          AND conrelid = 'sent_sms_reminders'::regclass
    ) THEN
        ALTER TABLE sent_sms_reminders
            ADD CONSTRAINT uq_sent_sms_reminders_reminder_id UNIQUE (reminder_id);
    END IF;
END$$;
