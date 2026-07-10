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
    pending_period,
    active_payment_method_id,
    last_applied_payment_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (user_id) DO NOTHING
RETURNING *;

-- name: GetSubscriptionByID :one
SELECT * FROM user_subscriptions WHERE id = $1;

-- name: GetSubscriptionByIDForUpdate :one
SELECT * FROM user_subscriptions WHERE id = $1 FOR UPDATE;

-- name: GetSubscriptionByUserID :one
SELECT * FROM user_subscriptions WHERE user_id = $1;

-- name: GetSubscriptionByUserIDForUpdate :one
SELECT * FROM user_subscriptions WHERE user_id = $1 FOR UPDATE;

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
    pending_period = $9,
    active_payment_method_id = $10,
    last_applied_payment_id = $11
WHERE id = $1
RETURNING *;

-- name: CreatePaymentMethod :one
INSERT INTO payment_methods (
    user_id,
    provider,
    provider_token,
    token_hash,
    display_mask,
    provider_card_id,
    exp_date,
    is_active
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpsertPaymentMethodByTokenHash :one
INSERT INTO payment_methods (
    user_id,
    provider,
    provider_token,
    token_hash,
    display_mask,
    provider_card_id,
    exp_date,
    is_active
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (user_id, token_hash)
DO UPDATE SET
    provider_token = EXCLUDED.provider_token,
    provider_card_id = EXCLUDED.provider_card_id,
    display_mask = EXCLUDED.display_mask,
    exp_date = EXCLUDED.exp_date,
    updated_at = now()
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
    payment_url,
    status,
    error_code
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetSubscriptionPaymentByID :one
SELECT * FROM subscription_payments WHERE id = $1;

-- name: GetSubscriptionPaymentByIDAdmin :one
SELECT sp.*, u.phone AS user_phone
FROM subscription_payments sp
JOIN users u ON sp.user_id = u.id
WHERE sp.id = $1;

-- name: GetSubscriptionPaymentByIDForUpdate :one
SELECT * FROM subscription_payments WHERE id = $1 FOR UPDATE;

-- name: ListSubscriptionPaymentsByUserID :many
SELECT * FROM subscription_payments WHERE user_id = $1 ORDER BY created_at DESC;

-- name: ListPendingSubscriptionPaymentsByUserID :many
SELECT * FROM subscription_payments WHERE user_id = $1 AND status = 'pending' ORDER BY created_at DESC;

-- name: MarkSubscriptionPaymentSucceeded :one
UPDATE subscription_payments
SET status = 'succeeded', updated_at = $2, succeeded_at = $2
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: MarkSubscriptionPaymentFailed :one
UPDATE subscription_payments
SET status = 'failed', error_code = $2, updated_at = $3
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: MarkSubscriptionPaymentRefunded :one
UPDATE subscription_payments
SET status = $2, refunded_amount_kopecks = $3, updated_at = $4
WHERE id = $1 AND status IN ('succeeded', 'pending', 'refunding')
RETURNING *;

-- name: MarkSubscriptionPaymentReconciledSucceeded :one
-- Transition a failed payment to succeeded after an explicit provider-side
-- status check. This handles out-of-order webhooks where the provider reports
-- success after the system has already marked the payment as failed.
UPDATE subscription_payments
SET status = 'succeeded', updated_at = $2, succeeded_at = $2, error_code = NULL
WHERE id = $1 AND status = 'failed'
RETURNING *;

-- name: MarkSubscriptionPaymentReconciledRefunded :one
-- Transition a failed payment to refunded after an explicit provider-side
-- status check. This handles out-of-order webhooks where the provider reports
-- a refund after the system has already marked the payment as failed.
UPDATE subscription_payments
SET status = 'refunded', refunded_amount_kopecks = $2, updated_at = $3
WHERE id = $1 AND status = 'failed'
RETURNING *;

-- name: BeginSubscriptionPaymentRefund :execresult
-- Atomically reserve a payment for an in-flight refund. updated_at is maintained
-- by the trg_subscription_payments_updated_at trigger, so it is not set here.
UPDATE subscription_payments
SET status = 'refunding'
WHERE id = $1 AND status IN ('succeeded', 'pending');

-- name: RevertSubscriptionPaymentRefund :execresult
-- Roll back an in-flight refund reservation to the previous status ($2).
-- updated_at is maintained by the trg_subscription_payments_updated_at trigger.
UPDATE subscription_payments
SET status = $2
WHERE id = $1 AND status = 'refunding';

-- name: UpdateSubscriptionPaymentProviderPaymentID :one
UPDATE subscription_payments
SET provider_payment_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateSubscriptionPaymentPaymentURL :one
UPDATE subscription_payments
SET payment_url = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateSubscriptionPaymentMethodAndProviderID :one
UPDATE subscription_payments
SET payment_method_id = $2, provider_payment_id = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateSubscriptionPaymentMethodID :one
UPDATE subscription_payments
SET payment_method_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListSubscriptionsUpForRenewal :many
SELECT * FROM user_subscriptions
WHERE status = 'active'
  AND auto_renew_enabled = true
  AND valid_until IS NOT NULL
  AND valid_until <= $1
ORDER BY valid_until ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: ListSubscriptionsInExpiredGrace :many
SELECT * FROM user_subscriptions
WHERE status = 'grace'
  AND valid_until IS NOT NULL
  AND valid_until <= $1
ORDER BY valid_until ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: ListExpiredNonRenewingSubscriptions :many
SELECT * FROM user_subscriptions
WHERE status = 'active'
  AND auto_renew_enabled = false
  AND valid_until IS NOT NULL
  AND valid_until <= $1
ORDER BY valid_until ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: ListExpiredCancelledSubscriptions :many
SELECT * FROM user_subscriptions
WHERE status = 'cancelled'
  AND valid_until IS NOT NULL
  AND valid_until <= $1
ORDER BY valid_until ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: ListSubscriptionsWithPendingChange :many
SELECT * FROM user_subscriptions
WHERE status = 'active'
  AND pending_tariff_id IS NOT NULL
  AND pending_change_at IS NOT NULL
  AND pending_change_at <= $1
ORDER BY pending_change_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: GetLastSucceededSubscriptionPaymentBySubscriptionID :one
SELECT * FROM subscription_payments
WHERE subscription_id = $1 AND status = 'succeeded'
ORDER BY created_at DESC
LIMIT 1;

-- name: ListPendingUpgradePayments :many
SELECT sp.*
FROM subscription_payments sp
JOIN user_subscriptions us ON us.id = sp.subscription_id
WHERE sp.status = 'pending'
  AND sp.provider_payment_id IS NOT NULL
  AND sp.tariff_id != us.tariff_id
  AND sp.created_at < $1
ORDER BY sp.created_at ASC
LIMIT $2;

-- name: ListPendingPayments :many
SELECT *
FROM subscription_payments
WHERE status = 'pending'
  AND provider_payment_id IS NOT NULL
  AND provider_payment_id <> ''
  AND created_at < $1
ORDER BY created_at ASC
LIMIT $2;

-- name: ListSubscriptionPaymentsAdmin :many
SELECT sp.*, u.phone AS user_phone
FROM subscription_payments sp
JOIN users u ON sp.user_id = u.id
WHERE (sqlc.arg('status')::text = '' OR sp.status = sqlc.arg('status')::text)
  AND (sqlc.arg('user_id')::uuid IS NULL OR sp.user_id = sqlc.arg('user_id')::uuid)
ORDER BY sp.created_at DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountSubscriptionPaymentsAdmin :one
SELECT COUNT(*)
FROM subscription_payments sp
JOIN users u ON sp.user_id = u.id
WHERE (sqlc.arg('status')::text = '' OR sp.status = sqlc.arg('status')::text)
  AND (sqlc.arg('user_id')::uuid IS NULL OR sp.user_id = sqlc.arg('user_id')::uuid);
