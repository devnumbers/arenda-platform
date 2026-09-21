-- name: UpsertPushSubscription :one
-- Insert a push subscription keyed by endpoint, or update its mutable fields
-- (user_id, p256dh, auth, expiration_time) when the endpoint already exists.
-- This makes re-subscribing on the same device idempotent and also re-binds an
-- endpoint that moved between accounts (rare) to the latest user. The
-- per-device settings (master enabled + the four category flags, решение
-- #738) always travel with the request: the browser keeps the desired state
-- locally and re-applies it on every subscribe, so the stored copy follows
-- the body.
INSERT INTO push_subscriptions (
    id, user_id, endpoint, p256dh, auth, expiration_time,
    enabled, category_rental, category_payments_operations, category_tasks, category_shared_access,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
ON CONFLICT (endpoint) DO UPDATE SET
    user_id          = EXCLUDED.user_id,
    p256dh           = EXCLUDED.p256dh,
    auth             = EXCLUDED.auth,
    expiration_time  = EXCLUDED.expiration_time,
    enabled          = EXCLUDED.enabled,
    category_rental              = EXCLUDED.category_rental,
    category_payments_operations = EXCLUDED.category_payments_operations,
    category_tasks               = EXCLUDED.category_tasks,
    category_shared_access       = EXCLUDED.category_shared_access,
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

-- name: GetPushSubscriptionByEndpointAndUser :one
-- The device's stored settings state (GET /push/subscriptions/preferences):
-- scoped to the user — another user's endpoint is not found.
SELECT * FROM push_subscriptions
WHERE endpoint = $1 AND user_id = $2;

-- name: UpdatePushSubscriptionPreferences :execrows
-- PUT /push/subscriptions/preferences (решение #738): the upsert of the
-- device's delivery state — master and the four category flags move, the
-- subscription's keys stay. Rows affected = 0 means the subscription does
-- not exist for this user (404).
UPDATE push_subscriptions
SET enabled                      = $3,
    category_rental              = $4,
    category_payments_operations = $5,
    category_tasks               = $6,
    category_shared_access       = $7,
    updated_at                   = now()
WHERE endpoint = $1 AND user_id = $2;
