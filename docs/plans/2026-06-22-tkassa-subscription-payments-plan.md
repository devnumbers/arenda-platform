# T-Kassa Provider For Subscription Payments

## Summary

- Implement the real `tkassa` payment provider for subscriptions instead of the current placeholder, keeping the existing user flow: the first payment uses the bank payment form, later renewals use a saved payment method.
- Selected flow: one-stage payment, no `Receipt` in this iteration, card saving through T-Bank `RebillId`, standalone card binding through the bank form with `AddCard CheckType=3DSHOLD`.

## Key Changes

- `tkassa` adapter:
  - Implement JSON HTTP client calls for `/v2/Init`, `/v2/Charge`, `/v2/GetState`, `/v2/AddCustomer`, `/v2/AddCard`, and `/v2/RemoveCard`.
  - Generate request `Token` from root-level parameters plus `Password`, excluding nested `DATA` and `Receipt`, sorting keys lexicographically, and hashing with SHA-256.
  - Verify webhook `Token`; do not trust redirect URLs for subscription activation.
  - For the first subscription payment, send `Init` with `PayType=O`, `Recurrent=Y`, `CustomerKey=<user_id>`, `DATA.OperationInitiatorType=2`, `NotificationURL`, `SuccessURL`, and `FailURL`.
  - For renewals, call `Init` with `DATA.OperationInitiatorType=R`, then call `Charge` with the saved `RebillId`.

- Billing application/API:
  - Extend the provider port and webhook model with `RebillId`, `CardId`, `Pan`, `ExpDate`, `CustomerKey`, and `NotificationType`.
  - Treat `AUTHORIZED` as "credentials received / still pending", not as a successful subscription payment. Treat `CONFIRMED` as succeeded and `AUTH_FAIL`/`REJECTED` as failed.
  - Replace raw-token `POST /subscription/payment-methods` behavior with bank-form initiation through `AddCustomer`/`AddCard`, returning `confirmUrl`.
  - Return exactly `200 OK` with body `OK` for T-Kassa webhooks.
  - Delete payment methods through `/v2/RemoveCard` when a provider `CardId` is available; do not delete locally if the provider delete fails.

- Persistence/config/docs:
  - Add `provider_card_id` to payment methods; keep `provider_token` as the encrypted `RebillId`.
  - Add idempotent payment method lookup/create by user and token hash so duplicate webhooks are safe.
  - Add T-Kassa base URL, timeout, notification, success, fail, add-card success, and add-card fail URL configuration.
  - Update OpenAPI, generated code, `.env.example`, subscription docs, and `CHANGELOG.md`.

## Test Plan

- Unit tests for token signing, webhook verification, nested object exclusion, and status mapping.
- Adapter tests through `httptest.Server` for `Init`, `Charge`, `GetState`, `AddCustomer`, `AddCard`, `RemoveCard`, provider errors, and timeouts.
- Application tests for `AUTHORIZED -> CONFIRMED`, `CONFIRMED` before/without duplicate `AUTHORIZED`, failed payment, renewal charge success/failure/pending, duplicate webhook idempotency, and standalone card binding webhooks.
- HTTP tests for exact T-Kassa webhook body `OK`, payment-method initiation response, and the removed raw-token request contract.
- Verification commands:
  - `cd apps/backend && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`
  - `cd apps/backend && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml`
  - `cd apps/backend && go test ./...`
  - `make backend-lint`

## Assumptions

- The T-Kassa terminal has bank payment form, recurrent/saved-credential payments, card payments, and webhooks enabled.
- `CustomerKey` and `OrderId` use existing UUID strings and stay within T-Bank limits.
- `Receipt` is intentionally omitted in this implementation; fiscalization/legal receipt handling is out of scope.
- Redirect URLs are UX only; webhook remains the source of truth.
- Refunds and cancellation provider APIs are out of scope for this iteration.

## References

- https://developer.tbank.ru/eacq/scenarios/payments/nonPCI
- https://developer.tbank.ru/eacq/scenarios/payments/nonPCI/card/
- https://developer.tbank.ru/eacq/scenarios/payments/nonPCI/autopay/
- https://developer.tbank.ru/eacq/api/init
- https://developer.tbank.ru/eacq/api/charge
- https://developer.tbank.ru/eacq/api/add-card
- https://developer.tbank.ru/eacq/intro/developer/notification
- https://developer.tbank.ru/eacq/intro/developer/token
