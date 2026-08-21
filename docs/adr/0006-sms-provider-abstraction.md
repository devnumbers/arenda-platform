# ADR 0006: SMS Provider Abstraction

## Status

Superseded by ADR 0044 (SMS Channel Removal) — the platform never shipped SMS;
authentication codes are delivered by email and reminders by email and Web
Push. The SMS port, adapters, and the `sent_sms_reminders` audit table are
removed.

## Context

The platform needs to send SMS messages for two distinct flows:

- Authentication: one-time codes for phone-number login.
- Notifications: operation reminders.

The SMS provider is expected to change over time. Options include a local
fake sender for development, T-Kassa SMS, REG.RU SMS, smsc.ru, or any other
regional provider. Business logic must not depend on a specific provider's API
or features.

## Decision

Introduce a single outbound SMS port and hide every provider behind it.

- The port lives in the notifications domain as `notifications.Sender` with one
  method:

  ```go
  Send(ctx context.Context, phone, message string) error
  ```

- All SMS traffic (auth codes and operation reminders) goes through this port.
- Provider-specific adapters live under
  `internal/notifications/adapters/sms/<name>/`.
- The active adapter is selected by the `SMS_SENDER` configuration value.
- The fake adapter logs messages via `slog` and is the default for local/dev
  environments.
- Sent messages are tracked in the `sent_sms_reminders` table to prevent
  duplicate sends.

## Consequences

- (+) Adding or switching an SMS provider only requires a new adapter; domain
  and application code stay unchanged.
- (+) Error handling, retries, and logging are applied uniformly across all SMS
  sends.
- (+) Tests and local development do not require real SMS credentials or
  network calls.
- (-) A small amount of extra configuration is needed to choose and configure a
  provider.
- (-) Provider-specific features, such as templates or a custom sender name,
  must be encapsulated inside the adapter and exposed only through the common
  `Send` signature.

## Future work

- Implement a real adapter for the chosen production provider.
- Add retry logic with exponential backoff for transient provider errors.
- Add metrics for sent and failed SMS messages.
