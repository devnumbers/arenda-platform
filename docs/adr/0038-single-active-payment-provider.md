# ADR 0038: Single active payment provider and provider-switch semantics

The billing module abstracts the payment processor behind a provider-neutral port (ADR 0007, hermetically sealed by the #248 rewrite), but the platform never routes payments to several processors at once: exactly one provider adapter is active per environment, selected by configuration, and every payment records the provider that created it. This ADR fixes what "one active provider" means and what happens when it changes.

## Status

Accepted

## Context

`PAYMENT_PROVIDER` selects the active adapter at startup (`fake` or `tkassa`; local defaults to `fake`, non-local environments require an explicit value and reject `fake`). Provider-side identities — the provider's payment id, the saved-card charge tokens, the card ids — live in the provider's own namespace: a reference issued by one processor is meaningless at another. The rewritten schema keeps `subscription_payments.provider` (and `payment_methods` provider-scoped references) precisely so historical payments stay interpretable regardless of which adapter is active now.

## Decision

- **Exactly one provider adapter is active per environment.** The composition root constructs it from configuration and wires it behind the port; there is no per-payment provider routing and no fallback chain. Adding a provider means adding an adapter behind the same port — never operating two at once.
- **Every payment is bound to the provider that created it** (`subscription_payments.provider`), and saved payment methods carry provider-scoped references only. The application layer never branches on the provider name: provider identity is data on payments, not control flow.
- **Switch semantics: saved methods of the previous provider are not chargeable after the switch.** Their charge tokens are meaningless to the new provider, so auto-renewal is impossible until the user binds a new method under the active provider. Subscriptions that rely on auto-renew therefore fail their charge and enter (or remain in) the grace period, exactly like any other failed renewal — the grace path is the switch path: no special migration, no cross-provider charge attempts. Grace communication (#253) tells the user to re-bind a payment method; the first successful payment after re-binding resumes normal operation.
- **Refunds of payments created by the previous provider must be settled before the switch** (or handled manually in the provider's dashboard). The module has no multi-provider routing to reach a retired adapter once its configuration is gone; a refund request for an old-provider payment against the new provider fails as payment-not-found.

## Considered Options

- **Multi-provider routing (payments pinned to their provider, both adapters active)** — rejected: no product requirement needs it, and it doubles configuration, webhook surfaces and operational dependencies for a single-product platform.
- **Cross-provider token migration** — rejected: charge tokens are not portable between processors by definition; only a fresh customer-initiated binding under the new provider creates chargeable credentials.

## Consequences

- (+) A provider switch is a configuration change plus ordinary grace handling — no data migration, no dual-write window.
- (+) The port stays hermetic: a future provider (or a replacement terminal) is a new adapter; the application layer is untouched.
- (-) A switch strands paid-but-unexpired subscriptions that relied on auto-renew until each owner re-binds a method (accepted: the grace period is the designed buffer, and #253 communicates it).
- (-) Old-provider refunds after the switch are a manual operator task (accepted: refunds are rare, full-amount, and the window is closable operationally before switching).

## See also

- [`docs/adr/0007-payment-provider-abstraction.md`](./0007-payment-provider-abstraction.md) — the provider port this narrows to a single active adapter.
- [`docs/adr/0008-subscription-lifecycle.md`](./0008-subscription-lifecycle.md) — the grace path a switch rides on.
- [`docs/adr/0037-billing-rewrite-schema-reset.md`](./0037-billing-rewrite-schema-reset.md) — the rewritten schema carrying per-payment provider identity.
- Issue #248 (provider port ticket), issue #244 (rewrite spec).
