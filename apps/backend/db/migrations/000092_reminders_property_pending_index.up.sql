-- Partial index supporting the "upcoming free reminders by property" endpoint,
-- which lists pending free-target reminders ordered by scheduled_at within a
-- property. The existing idx_reminders_owner already covers owner-scoped scans,
-- but this index gives a tight plan for the property page block.
CREATE INDEX IF NOT EXISTS idx_reminders_property_pending
ON reminders (property_id, scheduled_at)
WHERE target_type = 'free' AND status = 'pending';
