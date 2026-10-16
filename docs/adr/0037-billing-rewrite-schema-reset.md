# ADR 0037: Billing module rewrite with a destructive schema reset

## Status

Accepted

## Context

The billing context (subscriptions, tariffs, T-Kassa payments) predates the standards the repository settled on after the identity refactor: T-Kassa specifics leak into the application layer, flows are duplicated across services, operational parameters are scattered constants, ports are declared producer-side, transactions are opened by hand, and the schema accumulated 25+ incremental migrations carrying dead weight (the legacy `partial_refunded` status, card binding encoded as an `addcard:` prefix inside a payment-method token). The module rewrite (#244) rebuilds domain, application and adapters to the identity standards (ADR 0033 Unit-of-Work, ADR 0035 consumer-side interfaces).

The rewritten core (#245) needs a clean schema: the transition log and card-binding sessions are new tables, the tariff active flag is a new column, and the old placeholder conventions must not survive into the new domain. Running old and new code against shared tables is impossible: the tables are recreated, so the old module code is deleted entirely and the remaining flows (payments #250, card binding #251, workers #252, refunds #254, admin operations #255) return ticket by ticket on the new foundation.

Billing data on stage/prod is expendable — an explicit owner decision recorded in #244: subscriptions can be re-onboarded, tariff plans are re-seeded, and historical payment records have no operational consumer that cannot live without them during the transition window.

## Decision

The billing rewrite starts with a single destructive migration (`000104_billing_core_rewrite`) that drops and recreates the four core billing tables (`tariffs`, `user_subscriptions`, `payment_methods`, `subscription_payments`) with a clean schema, adds the new `subscription_transitions` (append-only, guarded by a trigger) and `card_binding_sessions` tables, adds `tariffs.is_active`, re-seeds the three canonical tariff plans (`uuidv7()` ids, ADR 0019), and re-onboards every existing owner to the basic plan with a `schema_reset` transition entry — losing paid billing state is accepted, but the platform must keep working for existing owners (without a subscription row the property limiter allows nothing). The migration applies both to an empty database and on top of old billing data; the old billing data is discarded.

Schema invariants are carried over from the evolved pre-rewrite schema where they encode real behaviour: partial unique indexes for payment idempotency, the deferred-change CHECK (`pending_change_at >= valid_until`), exactly-one-active-payment-method, worker batch partial indexes (ADR 0008). The legacy `partial_refunded` status is dropped — refunds are full-amount only.

## Considered Options

- **Additive migration** (new tables alongside old, backfill, switch, drop) — rejected: the rewrite changes the domain model, not just storage; a backfill of state the new domain interprets differently buys nothing for data that is accepted as lost.
- **Squash-all migrations** (fold every context into one baseline) — rejected: out of scope of #244; the reset is deliberately limited to the billing context.
- **Keep the old module until every flow is rewritten** (coexistence) — rejected: impossible on recreated tables, and it would force the new module to inherit the old schema's placeholder conventions.

## Consequences

- (+) The new module starts from a schema that matches its domain language 1:1; no legacy branches (`partial_refunded`, `addcard:` tokens) reach the new code.
- (+) The transition log is immutable against rewriting at the database level (UPDATE rejected by trigger). DELETE is deliberately not guarded: cascading user erasure fires row-level triggers and must be able to remove the log together with its subscription; TRUNCATE-based test isolation keeps working.
- (-) Stage/prod billing history is lost (accepted by the owner); the down migration cannot restore it — but it does restore the pre-rewrite *schema shape* (the four tables exactly as they stood at migration 000103, empty, without seed rows), because the historical down chain below 104 still issues ALTER/UPDATE against those tables and `migrate down -all` must stay executable (issues #313, #316; verified by the up/down/up cycle test mirroring the CI job).
- (-) Between #245 and the flow tickets (#249–#255) the corresponding HTTP endpoints answer 501 and the worker shells tick as no-ops — a deliberate, visible transitional state.
- The vendored T-Kassa OpenAPI spec (`internal/billing/adapters/payment/tkassa/spec/`) stays in place: it is provider contract data, not module code, and the provider-port ticket (#248) rebuilds the adapter on top of it.

## See also

- [`docs/adr/0008-subscription-lifecycle.md`](./0008-subscription-lifecycle.md) — the lifecycle behaviour the rewrite preserves.
- [`docs/adr/0033-unit-of-work-transactional-seam.md`](./0033-unit-of-work-transactional-seam.md) — the transactional seam the new application layer uses.
- [`docs/adr/0035-consumer-side-interfaces.md`](./0035-consumer-side-interfaces.md) — where the new transport-facing interfaces live.
- Issue #244 (rewrite spec), issue #245 (core ticket).
