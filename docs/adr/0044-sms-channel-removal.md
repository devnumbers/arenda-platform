# ADR 0044: SMS Channel Removal

## Status

Accepted — supersedes ADR 0006 (SMS Provider Abstraction).

## Context

ADR 0006 planned SMS as a delivery channel for two flows: authentication codes
and operation reminders. Neither shipped. Authentication delivers one-time
codes by email (the login code is bound to the phone+email triple, and the
phone is the account identifier, not a delivery address); reminders are
delivered over email and Web Push (ADR 0030 per-channel preferences). What
remained of SMS was dead weight: the `ChannelSMS` constant, the `SMSSender`
port with no implementation, an unwired `adapters/sms` notifier, the
`sent_sms_reminders` audit table with its queries, and SMS wording in product
docs and UI copy («Код из SMS», «SMS-настройка операции»).

The product decision: the platform will not send SMS — there is no provider
and no roadmap for one. Issue #379 (re-add SMS dispatch to the reminder
worker) is closed as wontfix.

## Decision

Remove the SMS channel end to end instead of keeping it dormant.

- Reminders are delivered over email and Web Push only; login and phone-change
  codes over email.
- Migration 000112 drops `sent_sms_reminders`; live-channel audit stays in
  `sent_email_reminders` / `sent_push_reminders`.
- The `SMSSender` port, `ChannelSMS`, and the `adapters/sms` package are
  deleted; the `Notifier` port shrinks to returning `error` (the
  provider-response return existed only to persist an SMS provider's reply).
- Product docs and UI copy stop mentioning SMS.

## Consequences

- (+) The channel vocabulary shrinks to the channels that exist; no dead
  branches in reminder dispatch and finalize.
- (+) One less outbound provider abstraction to configure, secure, and
  document; nothing logs or stores phone numbers for delivery.
- (-) Reintroducing SMS later means re-adding a channel end to end (port, audit
  table, preferences, UI copy) rather than flipping a configuration value.
  That cost is accepted: a dormant abstraction for a channel with no plans
  behind it misleads more than it helps.
