-- Terminal status for reminders whose event type the owner has revoked
-- permission for: the worker skips dispatch without retries.
ALTER TYPE notification_status ADD VALUE 'skipped';
