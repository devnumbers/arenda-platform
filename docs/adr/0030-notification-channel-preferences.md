# ADR 0030: Notification Channel Preferences (Per-Event-Type × Per-Channel)

The notification preference model moves from one `allowed` flag per event type
(ADR 0022) to a separate flag per delivery channel: each event type now carries
independent `emailAllowed` and `pushAllowed` settings. This is an expand-phase
change — the new per-channel table lives alongside the legacy
`user_notification_preferences`, nothing breaks, and the old table is removed in
a later contract phase once all callers migrate.

## Status

Accepted. Supersedes point 1 of [ADR 0022](./0022-notification-preferences.md)
("Permissions are bound to the event type, not to the delivery channel"). All
other points of ADR 0022 — opt-out default, soft revocation in the
ReminderWorker, bulk API, per-recipient enforcement, audit — remain in force,
generalised from event type to (event type, channel).

## Context

ADR 0022 bound permissions to the event type alone: one setting governed email
and any future channel. Web Push (spec #178) makes that model insufficient for
two reasons:

1. **Channels have different semantics.** Email delivery is always available to
   a user with a contact; push delivery additionally requires OS/browser
   permission and an active subscription. A single flag cannot express "I want
   email but not push" or vice versa. When push permission is revoked at the OS
   level, the user still needs a way to keep email on without the system
   treating the channel as silently failed.

2. **Users want independent control.** The product decision (#172, point 4) is
   to expose two sections in the UI — "Email" and "Пуши" — so the owner can
   silence one channel without losing the other. The per-event-type-only model
   makes that impossible.

Key questions:

- New table or ALTER the existing one?
- What are the default push settings for existing users?
- Is the old API shape preserved during the transition?

## Decision

### 1. Per-event-type × per-channel model

Each of the five event types carries two independent flags: `emailAllowed` and
`pushAllowed`. Both default to `true` (opt-out). A missing row for a
(user, event_type, channel) triple means allowed.

### 2. New table (expand → contract)

A new `user_notification_channel_preferences` table holds the per-channel rows,
keyed by `(user_id, event_type, channel)`. The legacy
`user_notification_preferences` table is frozen — new code reads and writes only
the new table. The old table is dropped in a subsequent contract-phase
migration once all reads (the legacy `ListPreferences`/`IsEventAllowed` methods)
are removed.

This avoids an in-place `ALTER TABLE ... ADD COLUMN` + primary-key change on a
table with existing rows, which is riskier than a parallel table plus a one-shot
data migration. Both tables coexist; the expand-phase API serves the new shape.

### 3. Data migration: push mirrors email

For every existing row in `user_notification_preferences`, the migration creates
two rows in the new table: `channel='email', allowed=<old value>` and
`channel='push', allowed=<old value>`. Existing users start with push mirroring
their email setting (#172, point 7). New users get both channels allowed by
default.

### 4. API backward compatibility

The `GET/PUT /notification-preferences` endpoints serve a new shape: each
preference carries `emailAllowed` and `pushAllowed`. The legacy `allowed` field
is preserved in the response (equal to `emailAllowed`) so existing clients that
read it continue to work. On writes the `allowed` field is ignored — clients
send `email_allowed`/`push_allowed` directly, and the expand-phase frontend was
updated in the same change to do so, so there is no legacy writer to support.
This dual shape lives until the frontend fully migrates to the per-channel UI,
after which `allowed` is removed (contract phase).

### 5. Enforcement in the ReminderWorker

The single dispatch-time check generalises from `IsEventAllowed(eventType)` to
`IsChannelAllowed(eventType, channel)`. The worker checks the channel it is
about to deliver on. In the current email-only dispatch path, that is
`ChannelEmail`; the upcoming push sender will check `ChannelPush`.

## Considered Options

- **ALTER the existing table** (`ADD COLUMN channel`, widen PK, duplicate rows).
  Rejected: primary-key change plus row duplication in one migration is riskier
  than a parallel table, and the expand-phase goal is to keep both models
  coexisting without touching the proven legacy path.

## Consequences

- (+) Owners can independently enable or disable email and push per event type,
  matching the product's two-section UI and the asymmetric semantics of push
  (requires permission/subscription) vs email (does not).
- (+) The expand-phase keeps the old table and API shape alive, so existing
  clients and the current email dispatch path continue to work unchanged during
  the transition.
- (+) Defaults preserve user intent: existing users keep their current email
  behaviour and gain push mirrored to the same setting, so no one is surprised
  by a new channel they did not ask for.
- (~) Two preference tables coexist until the contract phase removes the legacy
  one; the codebase carries both `NotificationPreference` and
  `NotificationChannelPreference` domain types and both sets of repository/
  service methods in the interim.
- (~) The `allowed` response field is a transient alias for `emailAllowed`; it
  will be removed once the frontend adopts the per-channel shape.

## See also

- [ADR 0022](./0022-notification-preferences.md) — the original per-event-type
  model; points 2–6 still apply, generalised to (event type, channel).
- [Spec #178](https://github.com/devnumbers/arenda-platform/issues/178) — PWA +
  Web Push для кабинета.
- [Map #172](https://github.com/devnumbers/arenda-platform/issues/172), point 7
  — push defaults mirror email for existing users.
