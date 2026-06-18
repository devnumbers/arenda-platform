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

-- name: CreatePaymentMethod :one
INSERT INTO payment_methods (
    user_id,
    provider,
    provider_token,
    token_hash,
    display_mask,
    is_active
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPaymentMethodByID :one
SELECT * FROM payment_methods WHERE id = $1;

-- name: GetPaymentMethodByIDForUpdate :one
SELECT * FROM payment_methods WHERE id = $1 FOR UPDATE;

-- name: LockPaymentMethodsByUserID :many
SELECT id FROM payment_methods WHERE user_id = $1 ORDER BY id FOR UPDATE;

-- name: ListPaymentMethodsByUserID :many
SELECT * FROM payment_methods WHERE user_id = $1 ORDER BY created_at DESC;

-- name: UpdatePaymentMethodActiveByID :one
UPDATE payment_methods
SET is_active = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeactivateAllPaymentMethodsForUser :exec
UPDATE payment_methods
SET is_active = false, updated_at = now()
WHERE user_id = $1;

-- name: CountSubscriptionsByActivePaymentMethodID :one
SELECT COUNT(*) FROM user_subscriptions WHERE active_payment_method_id = $1;

-- name: DeletePaymentMethodByID :exec
DELETE FROM payment_methods WHERE id = $1;

-- name: CreateSubscriptionPayment :one
INSERT INTO subscription_payments (
    user_id,
    subscription_id,
    tariff_id,
    payment_method_id,
    period,
    amount_kopecks,
    provider,
    provider_payment_id,
    status,
    error_code
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetSubscriptionPaymentByID :one
SELECT * FROM subscription_payments WHERE id = $1;

-- name: GetSubscriptionPaymentByIDForUpdate :one
SELECT * FROM subscription_payments WHERE id = $1 FOR UPDATE;

-- name: ListSubscriptionPaymentsByUserID :many
SELECT * FROM subscription_payments WHERE user_id = $1 ORDER BY created_at DESC;

-- name: MarkSubscriptionPaymentSucceeded :one
UPDATE subscription_payments
SET status = 'succeeded', updated_at = $2
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: MarkSubscriptionPaymentFailed :one
UPDATE subscription_payments
SET status = 'failed', error_code = $2, updated_at = $3
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: UpdateSubscriptionPaymentProviderPaymentID :one
UPDATE subscription_payments
SET provider_payment_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;
