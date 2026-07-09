# ADR 0010: T-Kassa Webhook as Source of Truth

## Status

Accepted

## Context

T-Kassa redirects the user to `SuccessURL` or `FailURL` after a payment attempt, but these redirects cannot be trusted for business decisions: the user can close the browser before the redirect, the request can be spoofed, or the URL can be tampered with. The final payment state arrives asynchronously via webhook from T-Kassa.

Receipts (fiscalization) are intentionally out of scope for the MVP. Card saving for recurring payments is performed through the `RebillId` returned in webhook notifications after the first successful payment.

## Decision

Webhook notifications (`AUTHORIZED`, `CONFIRMED`, `REJECTED`, `AUTH_FAIL`) are the source of truth for subscription payment status.

- Redirect URLs (`SuccessURL`/`FailURL`) are UX-only and do not update business state.
- Webhook token signatures are verified before parsing.
- The first payment uses one-stage `PayType=O` with `Recurrent=Y`, `CustomerKey`, and `OperationInitiatorType=2` (CIT COF) to obtain a `RebillId`.
- Renewals use `Init` followed by `Charge` with `OperationInitiatorType=R` (MIT recurring) and the saved `RebillId`.
- Standalone card binding uses `AddCustomer` + `AddCard` with `CheckType=3DSHOLD`. The redirect/return URL after T-Kassa card binding is configured at the T-Kassa terminal, not passed per request.
- `Receipt` is not sent in this iteration; fiscalization is out of scope.

## Consequences

- (+) Business state is updated only on verifiable provider events.
- (+) The same flow works for first payments and renewals.
- (-) The UI must tolerate a short delay between redirect and webhook.
- (-) Duplicate or out-of-order webhooks must be handled idempotently.
- (-) Without receipts, the solution is not legally complete for all merchants; this must be revisited before production fiscal requirements.

## Future work

- Add receipt support for fiscalization when production requirements are clear.
- Add webhook retry and polling fallback via `GetState` if webhooks are delayed.
- Add provider-specific metrics for webhook processing.

## Notes

- T-Kassa notification documentation: https://developer.tbank.ru/eacq/intro/developer/notification
