-- Billing context queries (rewritten module, issue #245).
--
-- Only the queries consumed by the rewritten core module and by the admin
-- dashboard live here. The payment, payment-method and webhook queries return
-- with their tickets (#250, #251, #254).

-- name: GetTariffByName :one
SELECT * FROM tariffs WHERE name = $1;

-- name: GetTariffByID :one
SELECT * FROM tariffs WHERE id = $1;

-- name: ListTariffs :many
-- User-facing tariff listing: hidden tariffs stay referable by FK but are not
-- offered (issue #245).
SELECT * FROM tariffs WHERE is_active ORDER BY monthly_price_kopecks, id;

-- name: ListAllTariffs :many
-- Admin tariff listing: every tariff including hidden ones (issue #247).
SELECT * FROM tariffs ORDER BY monthly_price_kopecks, id;

-- name: CreateSubscription :one
INSERT INTO user_subscriptions (
    id,
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
    last_applied_payment_id,
    current_period
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
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
    last_applied_payment_id = $11,
    current_period = $12
WHERE id = $1
RETURNING *;

-- name: AppendSubscriptionTransition :exec
-- The transition log is append-only (enforced by trigger, ADR 0037); the first
-- transition of a subscription has no prior status or tariff.
INSERT INTO subscription_transitions (
    id,
    subscription_id,
    from_status,
    to_status,
    from_tariff_id,
    to_tariff_id,
    reason,
    initiator_type,
    initiator_id,
    payment_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListSubscriptionTransitionsBySubscription :many
SELECT * FROM subscription_transitions
WHERE subscription_id = $1
ORDER BY created_at DESC, id DESC;

-- Admin dashboard stats. These queries are consumed by the admin context's
-- repository, not by the billing module itself.

-- name: CountActiveSubscriptionsAdmin :one
SELECT COUNT(*) FROM user_subscriptions WHERE status = 'active';

-- name: GetSubscriptionPaymentsStatsLast30dAdmin :one
-- Aggregates over payments created in the last 30 days. Refunds are full-amount
-- only in the rewritten schema (ADR 0037): the legacy partial_refunded status
-- is gone.
SELECT
  COALESCE(SUM(amount_kopecks) FILTER (WHERE status = 'succeeded'), 0)::bigint AS succeeded_total_kopecks,
  COUNT(*) FILTER (WHERE status = 'failed') AS failed_count,
  COUNT(*) FILTER (WHERE status = 'refunded') AS refunded_count
FROM subscription_payments
WHERE created_at >= now() - interval '30 days';

-- name: ListRecentSubscriptionPaymentsAdmin :many
SELECT sp.id, sp.user_id, sp.amount_kopecks, sp.status, sp.created_at,
       u.phone AS user_phone, u.phone_encrypted AS user_phone_encrypted
FROM subscription_payments sp
JOIN users u ON u.id = sp.user_id
ORDER BY sp.created_at DESC, sp.id DESC
LIMIT 5;
