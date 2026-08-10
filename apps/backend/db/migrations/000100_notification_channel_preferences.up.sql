-- Per-user, per-event-type, per-channel permission to send reminders
-- (ADR 0030). Expand-phase companion to user_notification_preferences (ADR
-- 0022): the new table carries independent email/push flags; the legacy table
-- is frozen and removed in a later contract-phase migration.
CREATE TYPE notification_channel AS ENUM ('email', 'push');

CREATE TABLE user_notification_channel_preferences (
    user_id    uuid                    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    event_type notification_event_type NOT NULL,
    channel    notification_channel    NOT NULL,
    allowed    boolean                 NOT NULL,
    created_at timestamptz             NOT NULL DEFAULT now(),
    updated_at timestamptz             NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, event_type, channel)
);

CREATE TRIGGER trg_user_notification_channel_preferences_updated_at
BEFORE UPDATE ON user_notification_channel_preferences
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- Data migration (#172, point 7): for every legacy per-event-type row, create
-- an email row with the same value and a push row mirroring it, so existing
-- users keep their current email behaviour and gain push matched to it.
INSERT INTO user_notification_channel_preferences (user_id, event_type, channel, allowed, created_at, updated_at)
SELECT user_id, event_type, 'email', allowed, created_at, updated_at
FROM user_notification_preferences
UNION ALL
SELECT user_id, event_type, 'push', allowed, created_at, updated_at
FROM user_notification_preferences;
