-- name: GetTariffByName :one
SELECT id, name, active_property_limit, monthly_price_kopecks, yearly_price_kopecks, created_at
FROM tariffs
WHERE name = $1;

-- name: GetTariffByID :one
SELECT id, name, active_property_limit, monthly_price_kopecks, yearly_price_kopecks, created_at
FROM tariffs
WHERE id = $1;

-- name: ListTariffs :many
SELECT id, name, active_property_limit, monthly_price_kopecks, yearly_price_kopecks, created_at
FROM tariffs ORDER BY monthly_price_kopecks, id;

-- name: CreateSubscription :one
INSERT INTO user_subscriptions (
    user_id,
    tariff_id,
    source,
    status,
    valid_until,
    auto_renew_enabled,
    pending_tariff_id,
    pending_change_at,
    active_payment_method_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (user_id) DO NOTHING
RETURNING *;

-- name: GetSubscriptionByUserID :one
SELECT * FROM user_subscriptions WHERE user_id = $1;

-- name: UpdateSubscription :one
UPDATE user_subscriptions
SET
    tariff_id = $2,
    source = $3,
    status = $4,
    valid_until = $5,
    auto_renew_enabled = $6,
    pending_tariff_id = $7,
    pending_change_at = $8,
    active_payment_method_id = $9
WHERE id = $1
RETURNING *;
