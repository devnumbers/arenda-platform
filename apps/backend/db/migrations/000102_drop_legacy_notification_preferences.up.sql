-- Contract phase (ADR 0030): the per-event-type preference table is replaced by
-- user_notification_channel_preferences (migration 000100). The API moved to
-- per-channel in #184; production code reads only the new table. Drop the
-- legacy table and its trigger. Data is lost — it was already mirrored into the
-- new table by migration 000100.
DROP TRIGGER IF EXISTS trg_user_notification_preferences_updated_at ON user_notification_preferences;
DROP TABLE IF EXISTS user_notification_preferences;
