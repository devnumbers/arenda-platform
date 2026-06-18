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
INSERT INTO user_subscriptions (user_id, tariff_id, source, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
RETURNING *;

-- name: GetSubscriptionByUserID :one
SELECT * FROM user_subscriptions WHERE user_id = $1;
