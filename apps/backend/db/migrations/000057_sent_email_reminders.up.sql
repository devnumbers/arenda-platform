CREATE TABLE sent_email_reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reminder_id UUID NOT NULL REFERENCES reminders(id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    subject TEXT NOT NULL,
    plain_body TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_sent_email_reminders_reminder_id UNIQUE (reminder_id)
);

CREATE INDEX idx_sent_email_reminders_owner_id ON sent_email_reminders(owner_id);
CREATE INDEX idx_sent_email_reminders_sent_at ON sent_email_reminders(sent_at);
