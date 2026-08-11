-- Audit + deduplication table for delivered push reminders. Mirrors
-- sent_email_reminders but uses a clearly named recipient_id column (the email
-- table carries an owner_id column that is semantically the recipient — naming
-- debt from issue #159, not repeated here). One row per (reminder, recipient)
-- enables per-recipient deduplication across dispatch retries.
CREATE TABLE sent_push_reminders (
    id           uuid        NOT NULL,
    reminder_id  uuid        NOT NULL REFERENCES reminders (id) ON DELETE CASCADE,
    recipient_id uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    sent_at      timestamptz NOT NULL DEFAULT now (),
    PRIMARY KEY (id)
);

CREATE UNIQUE INDEX uq_sent_push_reminders_reminder_recipient
    ON sent_push_reminders (reminder_id, recipient_id);
CREATE INDEX idx_sent_push_reminders_recipient ON sent_push_reminders (recipient_id);
CREATE INDEX idx_sent_push_reminders_sent_at ON sent_push_reminders (sent_at);
