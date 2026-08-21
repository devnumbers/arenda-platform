-- SMS channel removal (ADR 0044, supersedes ADR 0006): the platform delivers
-- reminders via email and Web Push only. sent_sms_reminders has been write-free
-- since the reminder worker moved to email/push dispatch; the audit trail for
-- live channels lives in sent_email_reminders / sent_push_reminders.
DROP TABLE IF EXISTS sent_sms_reminders;
