-- name: UpsertPushSubscription :one
-- Insert a push subscription keyed by endpoint, or update its mutable fields
-- (user_id, p256dh, auth, expiration_time) when the endpoint already exists.
-- This makes re-subscribing on the same device idempotent and also re-binds an
-- endpoint that moved between accounts (rare) to the latest user.
INSERT INTO push_subscriptions (
    id, user_id, endpoint, p256dh, auth, expiration_time, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (endpoint) DO UPDATE SET
    user_id          = EXCLUDED.user_id,
    p256dh           = EXCLUDED.p256dh,
    auth             = EXCLUDED.auth,
    expiration_time  = EXCLUDED.expiration_time,
    updated_at       = EXCLUDED.updated_at
RETURNING *;

-- name: DeletePushSubscriptionByEndpointAndUser :execrows
-- Delete a push subscription by endpoint scoped to a user. Returns 0 rows when
-- the subscription does not exist or belongs to another user (404 in the API).
DELETE FROM push_subscriptions
WHERE endpoint = $1 AND user_id = $2;

-- name: ListPushSubscriptionsByUser :many
SELECT * FROM push_subscriptions
WHERE user_id = $1
ORDER BY created_at ASC;
