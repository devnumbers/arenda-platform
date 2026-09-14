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

-- name: CreateTariff :one
-- Admin tariff creation (issue #256). The name is UNIQUE; a duplicate surfaces
-- as a unique violation the adapter narrows to ErrAlreadyExists.
INSERT INTO tariffs (
    id,
    name,
    active_property_limit,
    monthly_price_kopecks,
    yearly_price_kopecks,
    is_active
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateTariff :one
-- Admin tariff edit (issue #256): prices, property limit and the activity
-- flag. The name is immutable — user-facing tariff selection is by name, so a
-- rename would silently change what existing references point at.
UPDATE tariffs
SET active_property_limit = $2,
    monthly_price_kopecks = $3,
    yearly_price_kopecks = $4,
    is_active = $5
WHERE id = $1
RETURNING *;

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
    current_period,
    grace_archived_property_ids
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
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
    current_period = $12,
    grace_reminded_at = $13,
    grace_archived_property_ids = $14,
    keep_property_id = $15
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

-- The stand-only time-travel rig of issue #665 is the one deliberate writer
-- that moves history: the dunning retry schedule anchors at the latest
-- reason='grace_entered' transition (the ListSubscriptionsBySelection bound),
-- so shifting the schedule data-side means moving those rows in time. UPDATE
-- is rejected by the immutability trigger (ADR 0037), so the move is a
-- delete-and-reinsert of the same rows with shifted created_at; DELETE is
-- deliberately unguarded by that trigger. The queries below are reachable
-- only from the admin time-shift operation, which the config railguard keeps
-- out of production (BILLING_TIME_TRAVEL, local/dev/stage stands only).

-- name: DeleteSubscriptionGraceEntryTransitions :exec
DELETE FROM subscription_transitions
WHERE subscription_id = $1 AND reason = 'grace_entered';

-- name: AppendSubscriptionTransitionWithCreatedAt :exec
-- The time-travel twin of AppendSubscriptionTransition: identical columns plus
-- the shifted created_at (the plain append always stamps now()).
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
    payment_id,
    created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ShiftSubscriptionPaymentsCreatedAt :execrows
-- The other half of the coherent subscription time shift (issue #665): the
-- dunning retry predicate counts payments created at or after the grace entry
-- anchor, so the payments must travel by the same delta to keep the relative
-- order — a +24 h retry boundary stays consumed/unconsumed exactly as it was
-- before the shift.
UPDATE subscription_payments
SET created_at = created_at + make_interval(secs => sqlc.arg('delta_seconds')::double precision)
WHERE subscription_id = sqlc.arg('subscription_id');

-- Worker batch selections (issue #252, ADR 0008 lifecycle phases; one
-- parameterized query per aggregate since issue #286). The phases set the
-- values through application.SubscriptionSelection / PaymentSelection; a
-- bound left NULL drops its condition. Each listing is a plain selection; the
-- processing transaction re-reads and locks the row by user id, so a
-- concurrent mutation between listing and processing is re-checked under the
-- lock and never applied twice. The composite indexes of migration 000107
-- back every selection: the required status prefix narrows to the phase's
-- status, the second column serves the phase-clock range and the batch order.

-- name: ListSubscriptionsBySelection :many
-- The order follows the phase clock: deferred-change batches
-- (pending_change_due set) by pending_change_at, everything else by
-- valid_until, each with id as the tie-breaker — the oldest-first fairness of
-- the batches. A non-NULL user_id narrows the selection to one subscription:
-- the under-lock re-check of a phase, run in the transaction that locked the
-- row.
--
-- The grace_retry_due bound (ticket #431, spec #419) selects grace
-- subscriptions with a due dunning retry: the retry schedule is anchored at
-- the latest grace entry (the newest reason='grace_entered' transition) and
-- fires twice — +24 h and +72 h — each boundary consumed by any payment
-- created at or after the entry (a prior retry or a manual payment; a
-- successful manual payment also removes the row through the status bound).
-- The window must still be open and an active payment method linked: without
-- one there is nothing to charge. The 24/72-hour offsets are product
-- constants mirrored by graceRetryFirstAfter/graceRetrySecondAfter in
-- billing/application/phases.go — no schema or config knob.
SELECT * FROM user_subscriptions
WHERE (sqlc.narg('user_id')::uuid IS NULL OR user_id = sqlc.narg('user_id'))
  AND user_subscriptions.status = sqlc.arg('status')
  AND (sqlc.narg('auto_renew')::bool IS NULL OR auto_renew_enabled = sqlc.narg('auto_renew'))
  AND (sqlc.narg('valid_until_before')::timestamptz IS NULL OR valid_until <= sqlc.narg('valid_until_before'))
  AND (sqlc.narg('valid_until_after')::timestamptz IS NULL OR valid_until > sqlc.narg('valid_until_after'))
  AND (sqlc.arg('unreminded')::bool = false OR grace_reminded_at IS NULL)
  AND (sqlc.narg('pending_change_due')::timestamptz IS NULL
       OR (pending_tariff_id IS NOT NULL AND pending_change_at IS NOT NULL AND pending_change_at <= sqlc.narg('pending_change_due')))
  AND (sqlc.narg('grace_retry_due')::timestamptz IS NULL OR (
       valid_until IS NOT NULL
       AND valid_until > sqlc.narg('grace_retry_due')
       AND active_payment_method_id IS NOT NULL
       AND EXISTS (
           SELECT 1 FROM subscription_transitions gt
           WHERE gt.subscription_id = user_subscriptions.id
             AND gt.to_status = 'grace'
             AND gt.reason = 'grace_entered'
             AND gt.created_at = (
                 SELECT max(g2.created_at) FROM subscription_transitions g2
                 WHERE g2.subscription_id = user_subscriptions.id
                   AND g2.to_status = 'grace'
                   AND g2.reason = 'grace_entered')
             AND (
                 (gt.created_at + interval '24 hours' <= sqlc.narg('grace_retry_due')
                  AND (SELECT count(*) FROM subscription_payments p
                       WHERE p.subscription_id = user_subscriptions.id
                         AND p.created_at >= gt.created_at) = 0)
                 OR
                 (gt.created_at + interval '72 hours' <= sqlc.narg('grace_retry_due')
                  AND (SELECT count(*) FROM subscription_payments p
                       WHERE p.subscription_id = user_subscriptions.id
                         AND p.created_at >= gt.created_at) <= 1)
             )
       )))
ORDER BY
  CASE WHEN sqlc.narg('pending_change_due')::timestamptz IS NOT NULL THEN pending_change_at END ASC,
  CASE WHEN sqlc.narg('pending_change_due')::timestamptz IS NOT NULL THEN id END ASC,
  valid_until ASC,
  id ASC
LIMIT sqlc.arg('batch_limit');

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
    succeeded_at,
    expires_at,
    card_mask
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING *;

-- name: GetSubscriptionPaymentByID :one
SELECT * FROM subscription_payments WHERE id = $1;

-- name: GetSubscriptionPaymentByIDForUpdate :one
SELECT * FROM subscription_payments WHERE id = $1 FOR UPDATE;

-- name: ListSubscriptionPaymentsWithCardByUserID :many
-- The user's payment history (issues #250, #619): newest first, with the card
-- the payment was charged with resolved for display — the payment's own
-- snapshot first, the bound method's mask as the fallback for payments
-- created before the snapshot existed. A method deleted after the snapshot
-- was taken changes nothing; a deleted method behind a snapshot-less payment
-- resolves to NULL.
SELECT sp.*, COALESCE(sp.card_mask, pm.display_mask) AS resolved_card_mask
FROM subscription_payments sp
LEFT JOIN payment_methods pm ON pm.id = sp.payment_method_id
WHERE sp.user_id = $1
ORDER BY sp.created_at DESC, sp.id DESC;

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
    succeeded_at = $9,
    expires_at = $10,
    card_mask = $11
WHERE id = $1
RETURNING *;

-- name: ListExpiredPendingSubscriptionPayments :many
-- The TTL-expiry batch of the pending-payment worker (issue #616):
-- still-pending payments whose payer form deadline ran out — the server-side
-- expiry is the truth that marks them failed and unlocks the tariff choice.
-- Unlike the reconciliation selections this batch does not require a provider
-- reference: a crashed initiation must expire too. The worker re-checks and
-- locks every row in its own transaction.
SELECT * FROM subscription_payments
WHERE status = 'pending'
  AND expires_at IS NOT NULL
  AND expires_at < $1
ORDER BY expires_at ASC, id ASC
LIMIT $2;

-- name: ListSubscriptionPaymentsBySelection :many
-- The reconciliation batch of the payment phases (issues #252, #254): pending
-- payments stale enough that a webhook is presumed lost, and payments stuck in
-- the refunding reservation. The provider reference is mandatory in every
-- selection — without it there is nothing to query at the provider. The order
-- follows the staleness clock the phase set: updated-stale batches
-- (updated_before set) by updated_at, the rest by created_at, each with id as
-- the tie-breaker.
SELECT sp.* FROM subscription_payments sp
LEFT JOIN user_subscriptions us ON us.id = sp.subscription_id
WHERE sp.status = sqlc.arg('status')
  AND sp.provider_payment_id IS NOT NULL
  AND sp.provider_payment_id <> ''
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR sp.created_at < sqlc.narg('created_before'))
  AND (sqlc.narg('updated_before')::timestamptz IS NULL OR sp.updated_at < sqlc.narg('updated_before'))
  AND (sqlc.arg('tariff_change_only')::bool = false OR us.tariff_id IS DISTINCT FROM sp.tariff_id)
ORDER BY
  CASE WHEN sqlc.narg('updated_before')::timestamptz IS NOT NULL THEN sp.updated_at END ASC,
  CASE WHEN sqlc.narg('updated_before')::timestamptz IS NOT NULL THEN sp.id END ASC,
  sp.created_at ASC,
  sp.id ASC
LIMIT sqlc.arg('batch_limit');

-- Admin payment views (issue #254). The phone filter matches the stored
-- ciphertext (deterministic encryption) or the plaintext of a not-yet-
-- encrypted row, mirroring ListUsersAdmin; user input is never interpolated
-- into SQL — sort/order map to columns through fixed CASE arms.

-- name: ListSubscriptionPaymentsAdmin :many
SELECT sp.id, sp.user_id, sp.subscription_id, sp.tariff_id, sp.payment_method_id,
       sp.period, sp.amount_kopecks, sp.provider, sp.provider_payment_id, sp.payment_url,
       sp.status, sp.refunded_amount_kopecks, sp.charge_attempts, sp.error_code,
       sp.created_at, sp.updated_at, sp.succeeded_at, sp.expires_at, sp.card_mask,
       u.phone AS user_phone, u.phone_encrypted AS user_phone_encrypted
FROM subscription_payments sp
JOIN users u ON u.id = sp.user_id
LEFT JOIN user_subscriptions us ON us.user_id = sp.user_id
WHERE (sqlc.arg('user_id')::uuid IS NULL OR sp.user_id = sqlc.arg('user_id'))
  AND (sqlc.arg('status')::text = '' OR sp.status = sqlc.arg('status'))
  AND (sqlc.arg('user_phone')::text = '' OR u.phone = sqlc.arg('user_phone_enc')::text OR (u.phone = sqlc.arg('user_phone')::text AND u.phone_encrypted = false))
  AND (sqlc.arg('subscription_status')::text = '' OR us.status = sqlc.arg('subscription_status'))
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'createdAt' AND sqlc.arg('order')::text = 'asc' THEN sp.created_at END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'createdAt' AND sqlc.arg('order')::text = 'desc' THEN sp.created_at END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'amountKopecks' AND sqlc.arg('order')::text = 'asc' THEN sp.amount_kopecks END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'amountKopecks' AND sqlc.arg('order')::text = 'desc' THEN sp.amount_kopecks END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'status' AND sqlc.arg('order')::text = 'asc' THEN sp.status END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'status' AND sqlc.arg('order')::text = 'desc' THEN sp.status END DESC,
  CASE WHEN sqlc.arg('sort')::text = '' THEN sp.created_at END DESC,
  sp.id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountSubscriptionPaymentsAdmin :one
SELECT COUNT(*)
FROM subscription_payments sp
JOIN users u ON u.id = sp.user_id
LEFT JOIN user_subscriptions us ON us.user_id = sp.user_id
WHERE (sqlc.arg('user_id')::uuid IS NULL OR sp.user_id = sqlc.arg('user_id'))
  AND (sqlc.arg('status')::text = '' OR sp.status = sqlc.arg('status'))
  AND (sqlc.arg('user_phone')::text = '' OR u.phone = sqlc.arg('user_phone_enc')::text OR (u.phone = sqlc.arg('user_phone')::text AND u.phone_encrypted = false))
  AND (sqlc.arg('subscription_status')::text = '' OR us.status = sqlc.arg('subscription_status'));

-- name: GetSubscriptionPaymentAdmin :one
SELECT sp.id, sp.user_id, sp.subscription_id, sp.tariff_id, sp.payment_method_id,
       sp.period, sp.amount_kopecks, sp.provider, sp.provider_payment_id, sp.payment_url,
       sp.status, sp.refunded_amount_kopecks, sp.charge_attempts, sp.error_code,
       sp.created_at, sp.updated_at, sp.succeeded_at, sp.expires_at, sp.card_mask,
       u.phone AS user_phone, u.phone_encrypted AS user_phone_encrypted
FROM subscription_payments sp
JOIN users u ON u.id = sp.user_id
WHERE sp.id = $1;

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
-- created_at comes from the caller's clock (the domain session), not the
-- database default: the per-user binding limit's sliding window (ticket #427)
-- is measured on this timestamp against the service clock.
INSERT INTO card_binding_sessions (
    id,
    user_id,
    provider,
    request_key,
    status,
    expires_at,
    created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetCardBindingSessionByRequestKeyForUpdate :one
SELECT * FROM card_binding_sessions WHERE provider = $1 AND request_key = $2 FOR UPDATE;

-- name: ListOpenCardBindingSessionsByUserID :many
SELECT * FROM card_binding_sessions
WHERE user_id = $1 AND status = 'new'
ORDER BY created_at DESC, id DESC;

-- name: CountCardBindingSessionsByUserSince :one
-- Every started session counts, whatever its later outcome (ticket #427).
SELECT COUNT(*) FROM card_binding_sessions WHERE user_id = $1 AND created_at >= $2;

-- name: UpdateCardBindingSessionStatus :one
UPDATE card_binding_sessions
SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteExpiredCardBindingSessions :execrows
-- The hygiene batch (ticket #433): sessions past their lifetime, whatever
-- their status — an expired session never produces a payment method, and the
-- only later read of an old row is the binding limit's sliding window, which
-- is measured on created_at and long past for an expired session (TTL 24 h vs
-- window 1 h). The subselect keeps the delete batched; the loop re-runs it
-- until fewer than the batch limit rows remain, so repeated runs are safe.
DELETE FROM card_binding_sessions
WHERE id IN (
    SELECT c.id FROM card_binding_sessions c
    WHERE c.expires_at < $1
    ORDER BY c.expires_at
    LIMIT $2
);

-- name: CountSubscriptionPaymentsBySelection :one
-- The count twin of ListSubscriptionPaymentsBySelection for the stuck-payment
-- gauges (ticket #433): the WHERE clause mirrors the list query's one — keep
-- the two in sync when a selection field changes. The limit of the selection
-- value is ignored: a gauge counts the whole batch.
SELECT COUNT(*) FROM subscription_payments sp
LEFT JOIN user_subscriptions us ON us.id = sp.subscription_id
WHERE sp.status = sqlc.arg('status')
  AND sp.provider_payment_id IS NOT NULL
  AND sp.provider_payment_id <> ''
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR sp.created_at < sqlc.narg('created_before'))
  AND (sqlc.narg('updated_before')::timestamptz IS NULL OR sp.updated_at < sqlc.narg('updated_before'))
  AND (sqlc.arg('tariff_change_only')::bool = false OR us.tariff_id IS DISTINCT FROM sp.tariff_id);

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
