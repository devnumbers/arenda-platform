-- Existing rows will have NULL event_date. If the table already contains data,
-- reschedule or backfill those rows so that event_date is populated as needed.
-- In the MVP deployment the reminders table is empty, so no backfill is required.
ALTER TABLE reminders ADD COLUMN event_date DATE;
