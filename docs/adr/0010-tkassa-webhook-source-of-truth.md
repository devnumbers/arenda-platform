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
- Standalone card binding uses `AddCustomer` + `AddCard` with `CheckType=3DSHOLD`. `RedirectUrl`/`FailRedirectUrl` are passed per request (built from `APP_BASE_URL`), so the flow does not depend on the terminal's return URL settings: a production incident showed that an unconfigured terminal return URL makes T-Kassa fail the post-form redirect with error 9 ("Переадресовываемый url пуст"). These redirect fields are outside the official `AddCard` schema (TerminalKey, CustomerKey, Token, CheckType, IP, ResidentState) and the T-Kassa server excludes them from token verification, so the request token is computed without them — a production incident (error 204, "Неверный токен") showed that signing the whole body breaks `AddCard`. The documented guaranteed alternative is configuring Success/Fail Add Card URL in the terminal settings. The terminal-level notification URL remains mandatory: `AddCard` does not support a per-request `NotificationURL`, so card-binding webhooks arrive there.
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
