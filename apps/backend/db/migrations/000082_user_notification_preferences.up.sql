-- Per-user permissions to send reminders of each event type (opt-out model):
-- the absence of a row means the event type is allowed, so there is no
-- backfill. Permissions are bound to the event type, not to a channel.
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
