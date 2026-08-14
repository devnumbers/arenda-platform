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

-- Worker batch listings (issue #252, ADR 0008 lifecycle phases). Each listing
-- is a plain selection; the processing transaction re-reads and locks the row
-- by user id, so a concurrent mutation between listing and processing is
-- re-checked under the lock and never applied twice. The partial indexes of
-- migration 000104 back every filter below.

-- name: ListSubscriptionsUpForRenewal :many
SELECT * FROM user_subscriptions
WHERE status = 'active'
  AND auto_renew_enabled = true
  AND valid_until IS NOT NULL
  AND valid_until <= $1
ORDER BY valid_until ASC, id ASC
LIMIT $2;

-- name: ListSubscriptionsInExpiredGrace :many
SELECT * FROM user_subscriptions
WHERE status = 'grace'
  AND valid_until IS NOT NULL
  AND valid_until <= $1
ORDER BY valid_until ASC, id ASC
LIMIT $2;

-- name: ListExpiredNonRenewingSubscriptions :many
SELECT * FROM user_subscriptions
WHERE status = 'active'
  AND auto_renew_enabled = false
  AND valid_until IS NOT NULL
  AND valid_until <= $1
ORDER BY valid_until ASC, id ASC
LIMIT $2;

-- name: ListExpiredCancelledSubscriptions :many
SELECT * FROM user_subscriptions
WHERE status = 'cancelled'
  AND valid_until IS NOT NULL
  AND valid_until <= $1
ORDER BY valid_until ASC, id ASC
LIMIT $2;

-- name: ListSubscriptionsWithPendingChange :many
SELECT * FROM user_subscriptions
WHERE status = 'active'
  AND pending_tariff_id IS NOT NULL
  AND pending_change_at IS NOT NULL
  AND pending_change_at <= $1
ORDER BY pending_change_at ASC, id ASC
LIMIT $2;

-- Reconciliation listings (issue #252): pending payments stale enough that a
-- webhook is presumed lost. The provider reference is mandatory — without it
-- there is nothing to query at the provider.

-- name: ListStalePendingSubscriptionPayments :many
SELECT * FROM subscription_payments
WHERE status = 'pending'
  AND provider_payment_id IS NOT NULL
  AND provider_payment_id <> ''
  AND created_at < $1
ORDER BY created_at ASC, id ASC
LIMIT $2;

-- name: ListStalePendingUpgradeSubscriptionPayments :many
-- Pending payments whose target tariff differs from the subscription's current
-- one are the tariff-change payments of the ChangeTariff flow (upgrades and
-- recovery upgrades): a lost webhook here leaves the paid change unapplied.
SELECT sp.*
FROM subscription_payments sp
JOIN user_subscriptions us ON us.id = sp.subscription_id
WHERE sp.status = 'pending'
  AND sp.provider_payment_id IS NOT NULL
  AND sp.provider_payment_id <> ''
  AND sp.tariff_id != us.tariff_id
  AND sp.created_at < $1
ORDER BY sp.created_at ASC, sp.id ASC
LIMIT $2;

-- Subscription payments (issue #250). The partial unique index
-- idx_subscription_payments_one_pending_upgrade (user_id, tariff_id, period)
-- WHERE status = 'pending' is the durable idempotency backstop against double
-- payment initiation; Create surfaces its violation as a unique-constraint
-- error the application maps to ErrAlreadyExists.

-- name: CreateSubscriptionPayment :one
INSERT INTO subscription_payments (
    id,
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
    refunded_amount_kopecks,
    charge_attempts,
    error_code,
    succeeded_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING *;

-- name: GetSubscriptionPaymentByID :one
SELECT * FROM subscription_payments WHERE id = $1;

-- name: GetSubscriptionPaymentByIDForUpdate :one
SELECT * FROM subscription_payments WHERE id = $1 FOR UPDATE;

-- name: ListSubscriptionPaymentsByUserID :many
SELECT * FROM subscription_payments
WHERE user_id = $1
ORDER BY created_at DESC, id DESC;

-- name: ListPendingSubscriptionPaymentsByUserID :many
SELECT * FROM subscription_payments
WHERE user_id = $1 AND status = 'pending'
ORDER BY created_at DESC, id DESC;

-- name: UpdateSubscriptionPayment :one
UPDATE subscription_payments
SET
    payment_method_id = $2,
    provider_payment_id = $3,
    payment_url = $4,
    status = $5,
    refunded_amount_kopecks = $6,
    charge_attempts = $7,
    error_code = $8,
    succeeded_at = $9
WHERE id = $1
RETURNING *;

-- Payment methods (issue #251). Token uniqueness is enforced per user by the
-- UNIQUE (user_id, token_hash) constraint: the upsert converges on the
-- existing row instead of creating a duplicate card, and exactly one active
-- method per user is enforced by the partial unique index
-- idx_payment_methods_one_active_per_user. Sensitive columns (provider_token,
-- provider_card_id, exp_date) hold ciphertext; the application encrypts
-- before writing and decrypts after reading.

-- name: UpsertPaymentMethodByTokenHash :one
-- Inserts a method or converges on the row with the same (user_id,
-- token_hash): a re-bound card (webhook redelivery, sync polling, duplicate
-- binding) updates the token and display fields instead of duplicating the
-- row. Empty incoming display fields do not wipe stored ones, so completion
-- paths that do not know the card data cannot erase what an earlier delivery
-- stored. is_active is deliberately not in the update set: activation is a
-- separate explicit step.
INSERT INTO payment_methods (
    id,
    user_id,
    provider,
    provider_token,
    token_hash,
    display_mask,
    provider_card_id,
    exp_date,
    is_active
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (user_id, token_hash)
DO UPDATE SET
    provider_token = EXCLUDED.provider_token,
    provider_card_id = COALESCE(NULLIF(EXCLUDED.provider_card_id, ''), payment_methods.provider_card_id),
    display_mask = COALESCE(NULLIF(EXCLUDED.display_mask, ''), payment_methods.display_mask),
    exp_date = COALESCE(NULLIF(EXCLUDED.exp_date, ''), payment_methods.exp_date),
    updated_at = now()
RETURNING *;

-- name: GetPaymentMethodByID :one
SELECT * FROM payment_methods WHERE id = $1;

-- name: GetPaymentMethodByIDForUpdate :one
SELECT * FROM payment_methods WHERE id = $1 FOR UPDATE;

-- name: ListPaymentMethodsByUserID :many
SELECT * FROM payment_methods WHERE user_id = $1 ORDER BY created_at DESC, id DESC;

-- name: LockPaymentMethodsByUserID :many
-- Serializes activation switches per user: the deactivate-all / activate-one
-- pair must not interleave with a concurrent switch, or the one-active
-- partial unique index rejects the second committer.
SELECT id FROM payment_methods WHERE user_id = $1 ORDER BY id FOR UPDATE;

-- name: DeactivateAllPaymentMethodsForUser :exec
UPDATE payment_methods
SET is_active = false, updated_at = now()
WHERE user_id = $1;

-- name: UpdatePaymentMethodActiveByID :one
UPDATE payment_methods
SET is_active = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeletePaymentMethodByID :exec
-- Owner-scoped: the row must belong to the user issuing the deletion.
DELETE FROM payment_methods WHERE id = $1 AND user_id = $2;

-- Card binding sessions (issue #251). One row per initiated provider binding;
-- the request key is unique per provider, and open sessions are resolved by
-- status polling or the add-card webhook before the TTL expires.

-- name: CreateCardBindingSession :one
INSERT INTO card_binding_sessions (
    id,
    user_id,
    provider,
    request_key,
    status,
    expires_at
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetCardBindingSessionByRequestKeyForUpdate :one
SELECT * FROM card_binding_sessions WHERE provider = $1 AND request_key = $2 FOR UPDATE;

-- name: ListOpenCardBindingSessionsByUserID :many
SELECT * FROM card_binding_sessions
WHERE user_id = $1 AND status = 'new'
ORDER BY created_at DESC, id DESC;

-- name: UpdateCardBindingSessionStatus :one
UPDATE card_binding_sessions
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

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
