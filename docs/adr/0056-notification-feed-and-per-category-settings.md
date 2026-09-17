# ADR 0056: Notification Feed and Per-Category Settings

The notifications context grows a stored feed (map #734, data model — decision
#737): every event fans out one feed row per recipient, carrying a text
snapshot, payload links and personal read/deleted flags. The old per-event-type
× per-channel preference model (ADR 0030) is replaced by per-category
settings — email per account, push per device (decision #738) — and the old
table is dropped without migrating its rows: the handful of grace opt-outs are
reset, and the service category «Тариф» is honestly always on for everyone.

## Status

Accepted. Supersedes [ADR 0030](./0030-notification-channel-preferences.md)
(and, through it, the whole of [ADR 0022](./0022-notification-preferences.md)):
the "event type" axis of the preference model is gone — permissions bind to
the notification category, the stored feed is written regardless of settings,
and the settings matrix lives on the account (email) and on each push
subscription (push). The legacy table
`user_notification_channel_preferences` was dropped in migration `000131`.

## Context

ADR 0030 bound one `allowed` flag to each (event type, channel) pair. The
notifications map (#734) changed the landscape twice over:

1. **The feed always exists.** In-app delivery is the product: the feed row is
   written unconditionally and settings can only silence the email/push
   channels. A per-event-type permission grid no longer matches the surface —
   the settings screen (mockup 1789-100250) is a category × channel matrix
   (Аренда, Платежи и операции, Задачи, Совместный доступ), while Тариф and
   Системные are service categories that stay always-on and off-screen.
2. **Push is per device.** Web Push permission is a browser-level fact: the
   master «Получать пуш-уведомления» toggle and the four category toggles
   belong to each push subscription (master-off = `enabled=false` on the
   subscription, no re-subscribe needed), while email is one configuration per
   account.

With the taxonomy collapsed to six categories, keeping the dead
per-event-type table alive would preserve a model nothing reads anymore.

## Decision

- **Taxonomy**: notification category (6) → event type (catalog v1, 15 types,
  decision #737). New enum `notification_category`; the feed's event types
  join `notification_event_type`, whose dead rental-era values stay forever
  (#277). Category slugs: `rental`, `payments_operations`, `tasks`,
  `shared_access`, `tariff`, `system`.
- **Stored feed**: `notifications` — one row per recipient with the text
  snapshot (title, body, context label), payload links, `read_at`/`deleted_at`
  and a durable dedup invariant: unique `(user_id, dedup_key)` makes a repeat
  publication a no-op.
- **Settings (contract in #743)**: email per account —
  `GET/PUT /notification-preferences` over the four configurable categories;
  push per device — master `enabled` flag plus category flags on the push
  subscription. The old per-event-type `GET/PUT /notification-preferences`
  contract is removed; front and back move together in the map branch.
- **Reset, not migrate**: the old table and its rows are dropped (migration
  `000131`); stored grace opt-outs are not honoured. The grace sender
  (`DirectNotificationService`) delivers both channels unconditionally — the
  «Тариф» category is always on.

## Consequences

- The audit action `notification_preferences.updated` is no longer written;
  the admin label stays for the historical journal rows.
- Delivery gating (which channel reads which setting at send time) is the
  dispatch pipeline's (#740) and the settings API's (#743) vocabulary; until
  they land, only the always-on grace path delivers, and the interim
  `/profile/notifications` screen shows no settings.
- The feed is written by the publishers (#748–#752); dedup key formats are
  their vocabulary, the schema only enforces uniqueness.
