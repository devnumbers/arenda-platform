# ADR 0007: Payment Provider Abstraction

## Status

Accepted

## Context

The MVP requires payments through T-Kassa with automatic renewal. The payment
provider is expected to change depending on the environment: a fake provider for
local development and T-Kassa for production. Payments are asynchronous by
nature: the application initiates a payment, the user is redirected to a payment
form, and the final state arrives through a webhook.

Business logic, such as subscription lifecycle management, must not depend on a
specific provider's API or webhook format.

## Decision

Introduce a single subscription payment port and hide every provider behind it.

- The port and provider-neutral DTOs live in `internal/billing/application` as
  `Provider`, `InitRequest`, `InitResult`, `ChargeRequest`, `ChargeResult`, and
  `WebhookPayload`. The provider port has three methods:

  ```go
  Init(ctx context.Context, req InitRequest) (InitResult, error)
  Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
  ParseWebhook(ctx context.Context, payload []byte) (WebhookPayload, error)
  ```

  `Init` is used for the first payment/redirect flow and returns a provider
  payment identifier and a URL the user follows to complete the payment.
  `Charge` is used for recurrent charges with a saved token after a previous
  payment established the token. `ParseWebhook` normalizes an incoming provider
  webhook payload into the provider-neutral `WebhookPayload` consumed by the
  application service.

- Webhook handling is decoupled from the adapter: incoming provider payloads are
  normalized into `application.WebhookPayload` and passed to
  `PaymentService.HandleWebhook`.
- All subscription states (active, grace, overdue) and transitions (upgrade,
  renewal, downgrade, auto-archive) are managed by the application service, not
  by the payment provider adapter.
- Provider-specific adapters implement the application port and live under
  `internal/billing/adapters/payment/<name>/`.
- The fake adapter for local/dev returns a `PaymentURL` that can be used to
  confirm the subscription payment via
  `POST /internal/fake-subscription-payment/{id}/confirm`.
- The real T-Kassa adapter will implement the same port, sign outgoing requests
  with an SHA-256 token, and parse incoming T-Kassa webhooks.
- The active adapter is selected by the `PAYMENT_PROVIDER` configuration value.

## Consequences

- (+) Switching between the fake and real provider only requires configuration;
  domain and application code stay unchanged.
- (+) Business logic does not depend on the provider's API or webhook format.
- (+) The subscription lifecycle can be tested end-to-end without real payments.
- (-) Each provider requires a mapping from its specific webhook payload to the
  generic `WebhookPayload` structure.
- (-) Provider-specific errors and retries must be encapsulated inside the
  adapter.

## Future work

- Implement the T-Kassa adapter in `internal/billing/adapters/payment/tkassa/`.
- Add SHA-256 token signing for outgoing requests and webhook signature
  validation.
- Add a retry policy for transient provider errors.
- Add polling via `GetState` as a fallback when webhooks are not received.
- Add metrics for initiated, succeeded, and failed payments.

## Notes

- `payment_methods.provider_token` is encrypted at rest using the platform
  `encryption.Encryptor` (AES-256-GCM). An empty `ENCRYPTION_KEY` falls back to
  a pass-through encryptor for local development only; production must provide a
  32-byte key.
