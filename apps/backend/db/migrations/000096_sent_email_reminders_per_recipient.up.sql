-- Per-recipient delivery of property reminders (issue #159): reminders are
-- fanned out to the owner and all active property members, so the sent-email
-- audit must allow one row per (reminder, recipient) instead of one row per
-- reminder. The owner_id column is semantically the recipient user_id; it
-- keeps its name for compatibility.
ALTER TABLE sent_email_reminders DROP CONSTRAINT uq_sent_email_reminders_reminder_id;
ALTER TABLE sent_email_reminders ADD CONSTRAINT uq_sent_email_reminders_reminder_recipient UNIQUE (reminder_id, owner_id);
