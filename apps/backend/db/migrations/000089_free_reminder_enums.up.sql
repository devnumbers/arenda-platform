-- New target type and event type for free (user-created) reminders.
-- target_type='free' distinguishes reminders not tied to an operation or lease.
-- event_type='free_reminder' is the opt-out permission type "Свои напоминания".
ALTER TYPE notification_target_type ADD VALUE IF NOT EXISTS 'free';
ALTER TYPE notification_event_type ADD VALUE IF NOT EXISTS 'free_reminder';
