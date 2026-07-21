# ADR 0008: Subscription Lifecycle, Renewal and Readonly Mode

## Status

Accepted

## Context

The MVP needs a paid subscription model with three public tariffs (basic, pro,
business). Owners must be able to upgrade, downgrade, enable/disable auto-renew,
and manage payment methods. When a paid period ends the system must either renew
automatically or downgrade to the free basic tariff. If payment fails, the owner
needs a short grace period to fix the payment method before data mutations become
read-only.

Key questions:

- When does an upgrade vs downgrade take effect and how is it priced?
- What happens when auto-renewal fails?
- How do we protect data integrity when a subscription becomes read-only?
- How are property limits enforced when a user downgrades?

## Decision

### 1. Subscription status model

A subscription has one of three statuses:

- `active` — paid (or free basic) and fully usable.
- `grace` — renewal charge failed; data mutations are still allowed so the owner
  can fix the payment method.
- `cancelled` — the owner explicitly turned off auto-renew or the subscription
  was otherwise terminated; data mutations are allowed until the end of the
  already paid period and read-only afterwards.

A `blocked` status (grace period expired without payment) was part of the
original design but is never produced by code; it has been removed from the
model.

Status transitions are managed by the billing application service and the
scheduled billing worker.

### 2. Upgrades

Upgrading to a more expensive tariff:

- Requires immediate payment of the full price of the new tariff.
- Becomes active as soon as the payment succeeds.
- Resets the validity period from the moment of payment.
- Enables auto-renew automatically.

### 3. Downgrades

Downgrading to a cheaper tariff is always deferred and never charged at the
moment it is scheduled:

- `ScheduleDowngrade` sets the `pending_*` fields (`pending_tariff_id`,
  `pending_change_at`, `pending_period`) AND `auto_renew_enabled = true`.
  Enabling auto-renew is what lets the worker apply the change and keeps the
  new tariff renewing on the normal cycle afterwards.
- The billing worker applies the downgrade when `pending_change_at <= now`
  (`pending_change_at` equals the current `valid_until`, i.e. the end of the
  already paid period).
- Application depends on the target period's price:
  - A **free** target period (in practice the basic tariff) applies with **no
    charge**: `tariff_id` switches to the new tariff, `valid_until` is
    extended by the new tariff's chosen period from `now`,
    `auto_renew_enabled = true`, `status = active`, and the `pending_*` fields
    are cleared.
  - A **paid** target period is charged **at apply time** via the normal
    renewal path: the scheduled-change job leaves the row due, and the renewal
    worker charges the pending tariff/period price by the active payment
    method. On success the change is applied (the same field updates as above,
    plus the recorded payment); on failure the subscription enters grace and
    the pending change is cleared on grace entry — the scheduled change is
    considered consumed by the failed charge, and re-scheduling after renewal
    is a fresh user action.
- If the number of active properties exceeds the new tariff's
  `active_property_limit`, the system archives the excess properties;
  properties with open leases have those leases force-completed first (same
  side effects as a user-initiated completion), so the limit is always
  enforced. Manual archiving still rejects occupied properties.
- Scheduling itself is never charged: for a paid target the charge happens
  only at apply time. Subsequent renewals charge the NEW tariff on the normal
  renewal cycle.

### 4. Renewal, grace and forced downgrade

A background billing worker runs on a configurable interval:

- Charges subscriptions whose `valid_until` has passed and have
  `auto_renew_enabled = true`.
- On success: extends `valid_until` by one period and clears any scheduled
  downgrade. For `active` subscriptions the extension stacks on the current
  `valid_until`; a renewal from `grace` extends from the moment of payment —
  grace time is not paid for and is not preserved.
- On failure: moves the subscription to `grace` for 7 days.
- While in grace the owner may renew the current tariff through the normal
  `ChangeTariff` payment flow (a same-tariff change is rejected for `active`
  subscriptions but allowed in grace). Scheduling a downgrade from grace is
  rejected: grace is a transient state, so deferred changes cannot be planned
  into it.
- When the grace period expires: forces a downgrade to basic and archives
  excess properties (force-completing open leases as described above).

Turning off auto-renew keeps the current tariff until `valid_until`, after which
the same forced-downgrade logic applies.

The billing period is stored subscription state (`user_subscriptions.current_period`,
nullable `'month'|'year'`): it is set whenever a tariff transition is applied
(`ApplyTariffChange`, `ApplyRenewal`, `ApplyScheduledDowngrade`) and cleared when
the subscription is downgraded to basic via the grace-expiry/cancellation path
(`DowngradeToBasic`). Renewal resolution and the subscription API read this
column — they no longer derive the period from the last succeeded payment, which
was wrong right after a scheduled downgrade (the payment still referenced the
old tariff's period, so a `pro/year` target renewed as `pro/month`).

### 5. Readonly mode and recovery paths

When a subscription cannot mutate data (grace after the grace period, or `cancelled` with an expired
`validUntil`), HTTP middleware returns `403 SubscriptionBlocked` for mutating
requests. The following
paths are exempt so the owner can recover:

- `/auth/*`
- `/me`
- `/tariffs`
- `/subscription` and `/subscription/*`
- `/subscription/payment-methods/*`
- `/webhooks/*`
- `/internal/*`

### 6. Payment methods

- A user may have multiple saved payment methods.
- Only one method is active at a time.
- The active method is used for renewals.
- The active method cannot be deleted until another method is activated.
- Provider tokens are encrypted at rest; token hashes enforce uniqueness.

### 7. Pricing and fake provider

- All prices are stored and exposed in kopecks (`BIGINT`).
- Local development uses the fake payment provider (`PAYMENT_PROVIDER=fake`).
- Fake payments are confirmed via
  `POST /internal/fake-subscription-payment/{id}/confirm`, which is only
  available when `APP_ENV=local`.

## Consequences

- (+) Clear lifecycle makes it easy to reason about renewals, failures and
  downgrades.
- (+) Readonly mode protects data integrity while leaving recovery paths open.
- (+) Downgrade auto-archive keeps the system consistent with tariff limits
  without manual intervention.
- (+) Fake provider enables full end-to-end testing and local development
  without real payments.
- (~) Downgrade archiving commits in the same transaction as the tariff change; only a concurrent property create or restore that slips in before the subscription row lock can briefly exceed the limit.
- (+) The billing worker runs each tick under `pg_try_advisory_lock(0xB111)`
  (see `apps/backend/internal/platform/scheduler/billing_worker.go`), so all
  four phases — scheduled changes, renewals, pending upgrades and expired
  grace — execute on a single leader and never overlap.

## Future work

- Optionally add idempotency keys for renewal charges (distributed locking is
  already provided by the advisory lock above).
- Implement a "cancel now" endpoint to move a subscription to `cancelled`
  immediately.
- Add retry logic and owner notifications for failed renewal charges.
- Add admin endpoints to extend grace periods or assign service subscriptions.
