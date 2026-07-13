# ADR 0016: T-Kassa OpenAPI Mapping as Source of Truth

## Status

Accepted

## Context

T-Kassa API has a published OpenAPI specification. Keeping a copy of the spec inside the repository gives the backend a machine-readable source of truth for request and response shapes. From that spec we generate strongly-typed Go structs with `oapi-codegen`, so new fields, renamed enums, or changed optionality surface as compile-time breaks instead of runtime surprises.

This ADR records how each T-Kassa method maps to the adapter code, which generated types correspond to each request/response, the fields the adapter actually sends, and intentional deviations from the spec that were introduced because of production incidents or product requirements.

## Decision

### Spec location and generation

- The official T-Kassa OpenAPI spec (version **1.21**) lives at  
  `apps/backend/internal/billing/adapters/payment/tkassa/spec/openapi.yaml`.
- Generated Go types are in  
  `apps/backend/internal/billing/adapters/payment/tkassa/spec/spec.gen.go` (package `spec`).
- Generation is driven by `generate.go`:

```go
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.1 -generate types -package spec -o spec.gen.go openapi.yaml
```

Run `go generate` in the `spec` directory to regenerate types after a spec update.

### Mapping table

The adapter implementation is in `apps/backend/internal/billing/adapters/payment/tkassa/tkassa.go`. For several responses the spec either defines an inline object or a generic `Response` schema, so the adapter uses small local response structs that mirror the JSON shape while using domain-friendly types (e.g. `int64` for kopecks).

| T-Kassa method | Path | Adapter function | Generated request type | Generated / local response type | Required fields we send | Intentional deviations |
|---|---|---|---|---|---|---|
| Init | `POST /v2/Init` | `Provider.Init` | `spec.Init` | `spec.Response` (spec); local `initResponse` (adapter) | `TerminalKey`, `Amount` (kopecks), `OrderId` (payment UUID), `CustomerKey`, `PayType="O"`, `NotificationURL`, `SuccessURL`, `FailURL`, `DATA.OperationInitiatorType`, `Recurrent="Y"` when saving card | `OperationInitiatorType` is sent inside `DATA` rather than as a top-level enum. First payments use `"2"` (CIT COF) to obtain a `RebillId`; renewal `Init` uses `"R"` (MIT) and omits `Recurrent`. `Description` is truncated to 140 characters. |
| Charge | `POST /v2/Charge` | `Provider.Charge` | `spec.Charge` | Inline object in spec; local `chargeResponse` | `TerminalKey`, `PaymentId`, `RebillId` | `Amount` is intentionally omitted: T-Kassa takes the charge amount from the original `Init` call. |
| GetState | `POST /v2/GetState` | `Provider.Status` | `spec.GetState` | Inline object in spec; local `getStateResponse` | `TerminalKey`, `PaymentId` | — |
| Cancel | `POST /v2/Cancel` | `Provider.Cancel` | `spec.Cancel` | `spec.Cancel2` (spec); local `cancelResponse` (adapter) | `TerminalKey`, `PaymentId`, `Amount` (full refund) | `Amount` is always sent even though the spec makes it optional. `OriginalAmount`/`NewAmount` from the response are used to compute the refunded amount when present. |
| AddCustomer | `POST /v2/AddCustomer` | Called from `Provider.InitAddCard` | `spec.AddCustomer` | `spec.AddCustomerResponse`; local `addCustomerResponse` | `TerminalKey`, `CustomerKey` | Duplicate customer (`ErrorCode=7`) is treated as success so the card-binding flow can continue. |
| AddCard | `POST /v2/AddCard` | Called from `Provider.InitAddCard` | `spec.AddCard` | `spec.AddCardResponse`; local `addCardResponse` | `TerminalKey`, `CustomerKey`, `CheckType`, `Token`; plus `RedirectUrl` and `FailRedirectUrl` | `RedirectUrl`/`FailRedirectUrl` are **not** part of the official `AddCard` schema. The token is computed over the schema fields only; the redirect URLs are appended afterwards, because the T-Kassa server excludes them from token verification. `CheckType` defaults to `"3DSHOLD"`. `NotificationURL` is not supported by `AddCard`. |
| RemoveCard | `POST /v2/RemoveCard` | `Provider.RemoveCard` | `spec.RemoveCard` | `spec.RemoveCardResponse`; local `removeCardResponse` | `TerminalKey`, `CustomerKey`, `CardId` | `ErrorCode=107` is mapped to `application.ErrProviderCardNotFound`. |
| GetCardList | `POST /v2/GetCardList` | `Provider.GetCardList` | `spec.GetCardList` | Bare JSON array in spec; local `cardListItem` | `TerminalKey`, `CustomerKey` | Response is a bare array on success, not the usual envelope. The local item accepts `Status` values `"A"` (active), `"D"` (deleted), and also `"I"` (inactive) even though the generated enum only lists `"A"`/`"D"`. `RebillId` may be absent for cards not saved for recurrent charges. `ErrorCode=503`/`501` are mapped to domain "not found" errors. |

### Webhooks

`openapi.yaml` only describes the synchronous REST API; it does **not** define webhook notification schemas. Webhook handling (`Provider.ParseWebhook`) is therefore documented by reference to the official T-Kassa notification documentation and to ADR 0010:

- T-Kassa notification documentation: https://developer.tbank.ru/eacq/intro/developer/notification
- ADR 0010 records the webhook source-of-truth policy: business state is updated only on verified `AUTHORIZED`, `CONFIRMED`, `REJECTED`, or `AUTH_FAIL` notifications; `SuccessURL`/`FailURL` are UX-only.

### Contract tests

`tkassa_test.go` contains contract tests (`TestProviderInitContract`, `TestProviderChargeContract`, `TestProviderGetStateContract`, `TestProviderCancelContract`, `TestProviderAddCustomerContract`, `TestProviderAddCardContract`, `TestProviderRemoveCardContract`, `TestProviderGetCardListContract`). Each test captures the outgoing HTTP request, unmarshals the body into the generated `spec.*` request type, and asserts the values that the adapter sends:

- `Init` validates `spec.Init` fields, `PayType=O`, `Recurrent=Y`, URLs, truncated `Description`, and `DATA.OperationInitiatorType`.
- `Charge` validates `spec.Charge` fields and the absence of `Amount`.
- `GetState`/`Cancel`/`AddCustomer`/`AddCard`/`RemoveCard`/`GetCardList` validate the corresponding generated request types.
- `AddCard` additionally asserts that `RedirectUrl`/`FailRedirectUrl` are present but excluded from the token, matching the production incident fix.

These tests are the guard against drift between the adapter's `map[string]any` bodies and the generated spec types.

## Consequences

- (+) Request/response shapes are version-controlled and verifiable against the official spec.
- (+) Compile-time types catch schema changes (renamed fields, new enums) after `go generate`.
- (+) Contract tests make deviations explicit and prevent accidental regressions.
- (-) The adapter still builds request bodies as `map[string]any` at runtime, so the generated types are only enforced in tests, not in production code.
- (-) Several T-Kassa responses are inline objects or bare arrays, so the generated types are incomplete for those methods and local structs remain necessary.

## Future work

- Migrate the adapter internals from `map[string]any` request bodies to the generated `spec.*` request types in production code, keeping the token-signing logic intact.
- Regenerate types and rerun contract tests whenever T-Kassa publishes a new spec version.

## Notes

- T-Kassa notification documentation: https://developer.tbank.ru/eacq/intro/developer/notification
