-- name: ListSeenPopups :many
SELECT popup_key FROM user_popup_views
WHERE user_id = $1
ORDER BY popup_key ASC;

-- name: MarkPopupSeen :exec
INSERT INTO user_popup_views (user_id, popup_key)
VALUES ($1, $2)
ON CONFLICT (user_id, popup_key) DO NOTHING;
