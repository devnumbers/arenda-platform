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

Downgrading to a cheaper tariff:

- Does not require immediate payment.
- Is scheduled to take effect at the end of the already paid period.
- Enables auto-renew automatically so the scheduled change is applied by the
  worker.
- When the downgrade is applied, if the number of active properties exceeds the
  new tariff limit, the system archives the excess properties, skipping
  properties with open leases.

Paid downgrades (to a cheaper non-basic tariff) follow the same deferred model:
the tariff change is scheduled for the end of the already paid period and the
charge is performed by the billing worker at the moment the scheduled change is
applied, not at the time `ChangeTariff` is called.

### 4. Renewal, grace and forced downgrade

A background billing worker runs on a configurable interval:

- Charges subscriptions whose `valid_until` has passed and have
  `auto_renew_enabled = true`.
- On success: extends `valid_until` by one period and clears any scheduled
  downgrade.
- On failure: moves the subscription to `grace` for 7 days.
- When the grace period expires: forces a downgrade to basic and archives excess
  properties.

Turning off auto-renew keeps the current tariff until `valid_until`, after which
the same forced-downgrade logic applies.

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
- (-) Downgrade archiving is not atomic with the tariff change; concurrent
  property edits could temporarily exceed the limit.
- (-) The billing worker has no single-flight protection; overlapping runs could
  process the same subscription twice in rare cases.

## Future work

- Add idempotency keys or distributed locking for the billing worker.
- Implement a "cancel now" endpoint to move a subscription to `cancelled`
  immediately.
- Add retry logic and owner notifications for failed renewal charges.
- Add admin endpoints to extend grace periods or assign service subscriptions.
