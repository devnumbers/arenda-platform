-- name: ListNotificationChannelPreferences :many
SELECT * FROM user_notification_channel_preferences
WHERE user_id = $1
ORDER BY event_type ASC, channel ASC;

-- name: UpsertNotificationChannelPreference :exec
INSERT INTO user_notification_channel_preferences (user_id, event_type, channel, allowed)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, event_type, channel)
DO UPDATE SET allowed = EXCLUDED.allowed;

-- name: IsNotificationChannelAllowed :one
SELECT COALESCE((
    SELECT allowed FROM user_notification_channel_preferences
    WHERE user_id = $1 AND event_type = $2 AND channel = $3
), true)::boolean AS allowed;
