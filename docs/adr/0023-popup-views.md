# ADR 0023: Popup Views (Server-Side Seen Markers for One-Time Popups)

## Status

Accepted

## Context

Each user must see a one-time onboarding modal about reminders: a text, the
four reminder-type checkboxes and a hint that further configuration lives in
"Мои данные". New users see it on first login to the cabinet; existing users
on their next visit. The modal can be closed only with the "Сохранить"
button — no close icon, no backdrop click. We need a generic mechanism that
guarantees a popup is shown exactly once per user, across all devices.

Key questions:

- Where does the "seen" marker live — inside notification preferences or in
  a dedicated entity?
- Who owns the list of active popups and their content?
- How does the frontend learn which popups are pending and report them seen?

## Decision

### 1. Dedicated table instead of a flag in notification preferences

New table `user_popup_views`:

- composite primary key (`user_id`, `popup_key`);
- `seen_at` — when the popup was seen;
- FK `user_id → users(id) ON DELETE CASCADE`.

A row is written once; its presence means "seen". Separation of concerns:
`/notification-preferences` (ADR 0022) governs reminder *delivery*; popup
views govern *display*. A `configured` flag in preferences was rejected
precisely because a view is a separate entity.

### 2. Popup registry is a constant in backend code

Active popup keys are an ordered constant list in backend code — currently a
single key, `reminders_onboarding`. Popup content is static and lives on the
frontend: no CMS, a deliberate decision.

### 3. API

- `GET /popups/pending` → `{popups: string[]}`: active registry keys minus
  the keys the user has already seen.
- `POST /popups/{popupKey}/seen` → 204. Idempotent: a repeat call is not an
  error. An unknown key → 400.

Both endpoints are exempt from the readonly middleware: closing a popup must
work even for users with a blocked subscription.

### 4. Exactly-once display via the server-side marker

The "seen" marker lives on the server per user per popup, so the popup is
shown exactly once on all devices. If the registry grows to several popups,
they are shown one at a time, in registry order.

### 5. No audit

A popup view is telemetry, not a domain action; nothing is written to the
audit log (ADR 0020).

## Consequences

- (+) Exactly-once display per user on all devices via a server-side marker.
- (+) Clean separation of concerns: delivery stays in notification
  preferences, display in popup views; no `configured` flag conflating the
  two entities.
- (+) Adding a popup is a small, reviewable code change: a registry key on
  the backend plus static content on the frontend.
- (+) The readonly exemption keeps popups dismissible for users with a
  blocked subscription, consistent with the notification-preferences
  exemption in ADR 0022.
- (~) Popup content changes require a frontend release — accepted
  deliberately, no CMS is wanted.
- (~) `user_popup_views` grows by one row per user per popup; the volume is
  trivial and rows are removed with the user via `ON DELETE CASCADE`.

## See also

- [`docs/adr/0022-notification-preferences.md`](./0022-notification-preferences.md)
  — per-type reminder delivery permissions; the delivery side of the same
  reminders feature.
