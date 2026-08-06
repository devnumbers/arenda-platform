-- Reverts 000096 to single-row-per-reminder audit uniqueness.
-- NOTE: this migration fails with a unique violation if any reminder already
-- has audit rows for multiple recipients (the per-recipient model introduced
-- by the up migration); such rows must be removed manually before rolling back.
ALTER TABLE sent_email_reminders DROP CONSTRAINT uq_sent_email_reminders_reminder_recipient;
ALTER TABLE sent_email_reminders ADD CONSTRAINT uq_sent_email_reminders_reminder_id UNIQUE (reminder_id);
