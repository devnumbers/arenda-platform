# ADR 0022: Notification Preferences (Per-Type Reminder Opt-Out)

## Status

Superseded by [ADR 0056](./0056-notification-feed-and-per-category-settings.md)
(per-category settings; the per-event-type model and its tables are removed —
migration `000131`).

Was accepted

## Context

The backend sends reminders of five event types — `operation_due`,
`operation_overdue`, `lease_expiring` (30 days before lease end),
`lease_requires_action` and `free_reminder` (user-created free reminders) —
and the owner has no way to unsubscribe from
individual types: every user receives every type. We need per-user,
per-type control over reminder delivery.

Key questions:

- Opt-in or opt-out default?
- Are permissions bound to the event type or to the delivery channel?
- What happens to reminders already scheduled when a permission is revoked?

## Decision

### 1. Opt-out model with per-type permissions

Each user can grant or revoke permission for each of the five event types
independently. All types are allowed by default: the absence of a preference
row means allowed, so no backfill is needed and existing behaviour is
preserved for all users.

Permissions are bound to the event type, not to the delivery channel: one
setting governs email today and future channels tomorrow. (Superseded by
ADR 0030: preferences are per event type × channel.)

### 2. Storage

New table `user_notification_preferences`:

- composite primary key (`user_id`, `event_type`);
- `allowed` flag plus `created_at`/`updated_at`;
- FK `user_id → users(id) ON DELETE CASCADE`.

A row is written only when the user changes a setting (upsert); a missing
row is read as `allowed = true`.

### 3. Soft revocation, enforced in the ReminderWorker

Revoking a permission never deletes already created reminders. The reminder
worker checks the permission at dispatch time — after picking up a due
reminder, before calling the delivery channel. If the type is revoked, the
reminder transitions to a new terminal status `skipped` ("skipped: disabled
in notification preferences") with no retries, alongside the existing
`pending`, `sending`, `sent`, `failed` and `cancelled` statuses.

Re-granting the permission resumes delivery of future reminders; skipped
reminders are not caught up. Reminder creation is not filtered by
preferences: the single enforcement point is dispatch time.

### 4. Bulk API

- `GET /notification-preferences` returns all five types with their
  effective `allowed` values (`true` for types without a row).
- `PUT /notification-preferences` replaces all five values in one request,
  upserted in a single transaction.

### 5. Audit

Permission changes are written to the audit log (see ADR 0020): only the
types that actually changed, with old and new values. Unchanged types are
not logged.

### 6. Delivery to property members (issue #159)

A reminder bound to a property is delivered to the owner **and** to every
active member of the property (roles `full_access` and `viewer` alike).
Suspended memberships receive nothing — the recipient lister filters them
out at the source.

Preferences are enforced per recipient: each user is checked against their
own `user_notification_preferences` row at dispatch time, so one member's
opt-out never silences the others. The reminder reaches its terminal status
once per dispatch, aggregated over all recipients:

- `sent` — at least one recipient was delivered (or already had an audit
  row) and no recipient failed;
- `skipped` — **every** recipient revoked permission for the event type;
- `cancelled` — no recipient has a resolvable contact (a deliberate change
  of the boundary semantics: previously the owner's missing contact alone
  cancelled the reminder);
- `failed` — at least one recipient failed; the retry re-sends only to
  recipients without an audit row.

The `sent_email_reminders` audit stores one row per recipient: uniqueness
moved from `UNIQUE (reminder_id)` to `UNIQUE (reminder_id, owner_id)`, where
`owner_id` is semantically the recipient user_id (the column keeps its
name). This gives per-recipient dedup on retries after a partial fan-out
failure.

## Consequences

- (+) Owners can silence individual reminder types without losing the rest.
- (+) Opt-out with no backfill keeps the migration trivial and preserves
  current behaviour for all existing users.
- (+) Soft revocation keeps reminder history intact; re-granting works
  immediately for future dispatches.
- (+) Property reminders fan out to the owner and all active members with
  per-recipient preference checks and per-recipient audit dedup, so a
  partial delivery failure retries only the missing recipients (issue
  #159).
- (+) Type-level (not channel-level) permissions keep the model simple and
  automatically cover future channels.
- (~) A reminder can still be created for a revoked type and occupies a row
  until the worker marks it `skipped`; creation-time filtering was rejected
  to keep enforcement in a single place.
- The `skipped` status is terminal: re-granting a permission does not
  resurrect skipped reminders.
- `/notification-preferences` is exempt from the readonly middleware:
  users with a blocked or expired subscription must always be able to
  manage communication permissions — opt-out is never paywalled. The
  worker keeps delivering reminders to such users unless they revoke
  permission.
- (~) TOCTOU window accepted: the permission is checked at the start of
  dispatch; a revocation landing mid-dispatch (contact resolve, render,
  audit write — a sub-second window) may result in one in-flight send.
  External delivery is non-transactional, the window cannot be fully
  closed, and the affected party is the revoking user themselves.

## See also

- [`docs/adr/0020-audit-log.md`](./0020-audit-log.md) — business audit used
  for preference changes.
