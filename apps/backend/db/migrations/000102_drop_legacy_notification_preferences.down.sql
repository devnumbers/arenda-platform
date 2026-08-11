-- Revert 000102: recreate the legacy per-event-type preference table. Data is
-- lost — it was mirrored into user_notification_channel_preferences by
-- migration 000100 and is not copied back here. The legacy shape is restored
-- only so a rollback can re-run migration 000100 cleanly.
CREATE TABLE user_notification_preferences (
    user_id    uuid                     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    event_type notification_event_type  NOT NULL,
    allowed    boolean                  NOT NULL,
    created_at timestamptz              NOT NULL DEFAULT now(),
    updated_at timestamptz              NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, event_type)
);

CREATE TRIGGER trg_user_notification_preferences_updated_at
BEFORE UPDATE ON user_notification_preferences
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
