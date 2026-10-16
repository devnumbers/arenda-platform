-- Recreate sent_sms_reminders roughly as it existed before the drop
-- (000008 + 000012/000021 unique constraint + 000079 dropped id default).
-- Data is lost.
CREATE TABLE IF NOT EXISTS sent_sms_reminders (
    id UUID PRIMARY KEY,
    reminder_id UUID REFERENCES reminders(id) ON DELETE SET NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    phone TEXT NOT NULL,
    message TEXT NOT NULL,
    provider_response TEXT,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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
