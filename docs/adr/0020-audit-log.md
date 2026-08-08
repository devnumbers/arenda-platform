# ADR 0020: In-Transaction Audit Log

## Status

Accepted

## Context

The audit log was deliberately kept out of the MVP: `docs/entities/operacija.md` and the operation editing docs listed it among the excluded features. As the product grew — billing, subscriptions, payment methods, and the admin panel — a persistent "who did what, and when" journal became necessary: support needs to trace user, admin, and system actions, and sensitive billing mutations (refunds, manual payment syncs, tariff changes) need attributable history.

The journal must cover more than HTTP-driven user actions: system activity (the renewal worker, webhook background processing, watchdog reconciliation) and security-relevant auth events (registrations, logins, failed logins, logouts) belong in the same record. At the same time it must never turn into a PII or secrets dump.

## Decision

Approach A: write the audit entry to the `audit_log` table **in the same database transaction as the business operation**, through the `auditapp.Recorder` port of the new `internal/audit` module. Recording is **fail-safe**: an insert error is returned to the caller and rolls the business operation back, so a mutation never commits without its audit entry.

The module follows the standard layering:

- `internal/audit/domain` — the `Entry` model, actor roles, entity types, and the registry of audited actions (dotted `<module>.<verb>` identifiers).
- `internal/audit/application` — the `Recorder` port (`Record`, `WithTx`), its `Service` implementation, and a `Noop` recorder.
- `internal/audit/adapters/postgres` — the `EntryWriter` over sqlc queries; `WithTx` binds the writer to the business transaction.

Services that already run inside a transaction call `audit.WithTx(tx).Record(...)` before commit. The schema (migration `000080_audit_log`) stores `actor_id` with `ON DELETE SET NULL` so entries outlive deletion of the acting user, `context` as `jsonb`, `request_id` as text, and `ip` as `inet`, with indexes on `created_at`, actor, entity, and action. Entry ids are application-generated UUIDv7 per ADR 0019.

### Documented exceptions to fail-safe

- **Logout / logout-all (fail-open).** The session is deleted first; the entry is written post-commit in `internal/platform/httpapi/auth_handlers.go`, and a recording error is only logged. Failing the response after the session is already gone would be wrong, and there is nothing to roll back.
- **Services without an explicit transaction (fail-loud).** Tenant contacts and operation categories mutate without a surrounding transaction, so the entry is written post-commit and the recording error is deliberately returned to the caller. The mutation is already committed; surfacing the error makes audit gaps visible instead of silent. A retry may duplicate the entity — accepted for these entities.

### Actors and attribution

- Actor roles: `owner` (authenticated user actions), `admin` (admin-panel actions), `system` (workers, webhook background processing, watchdog reconciliation — `actor_id` is NULL), and `anonymous` (failed logins, where the actor is unknown). Property Sharing (PRD #153, issue #166 follow-up) adds `full_access` and `viewer`: property-scoped actions write the actor's real role resolved through the policy port (`internal/shared/policy`) instead of masking every action as the owner's own. The strings are identical to the policy `Role` values; each calling module (leases, properties, access) owns its mapper because audit does not depend on shared/policy, and any role that never reaches a write path falls back to `owner`. `viewer` entries appear only through `property_member.left` (self-exit is the single write a viewer may perform); `suspended` and `none` actors are stopped by the write gates before any `Record` call, so they never appear in the journal.
- `actor_id` comes from service parameters (`ownerID`) for user actions and from the session for admin actions: `PaymentProcessor.RefundPayment` and `SyncPendingPayment` take an explicit `actorID` parameter, while the background sync passes `uuid.Nil`, which yields a system entry.
- `request_id` and client IP are filled automatically by the recorder from `internal/platform/requestctx`: the request-ID middleware and `realIPMiddleware` place both into the request context, so call sites never pass them by hand.

### PII policy

The `context` jsonb carries a strict whitelist of structured details, such as ids, amounts in kopecks, dates, names of changed fields, entity display names (e.g. property or category names), enum/bool state values, and system triggers. It must never contain phone numbers, emails, session or payment tokens, PAN, tenant names, comment texts, or raw request bodies.

### What is recorded

- All mutating use cases across identity, properties, leases, and billing, plus auth events (register, login, login failed, logout, phone change).
- Reads (GET) are never recorded.
- No-op branches where nothing actually changed write no entry.
- System entries carry `context.trigger` (e.g. `"scheduler"`, `"billing_limit"`) naming what initiated the action.

### Known v1 gaps (accepted)

- Renewal charges created by the billing worker are not recorded.
- `ConfirmFakePayment` (the dev-only fake payment flow) is not recorded.
- `SyncPaymentMethods` and other payment-method upsert paths are not recorded: `ON CONFLICT` does not distinguish insert from update, so recording would emit duplicate entries.
- Scheduler-driven lease transitions (`ReconcileRequiresAction`) are not recorded.
- A redelivered AddCard webhook writes a duplicate `payment_method.added` entry: webhook processing is idempotent for state but not for the journal.

### Reading and retention

Entries are read-only through the admin API (`GET /admin/audit-logs`, `GET /admin/audit-logs/{id}`, `GET /admin/users/{id}/audit-logs`) and the react-admin «Журнал действий» resource (a standalone list plus a tab on the user page). There is no update or delete path. Retention is unbounded for now: rotation or partitioning is deferred to a separate ADR when volume demands it.

## Rejected alternatives

- **(B) In-process event dispatcher.** Publishing "something happened" events through the `InProcessDispatcher` (ADR 0014) and recording them in a subscriber would decouple business code from the journal, but the dispatcher is synchronous, in-memory, and has no durability: a subscriber writing outside the business transaction loses entries on a crash between commit and handling — exactly the limitation ADR 0014 documents. Audit requires the opposite guarantee.
- **(C) HTTP middleware.** A middleware logging every mutating request has no domain semantics (it cannot name the action or the entity), cannot see system actions that never pass through HTTP, and pollutes the journal with failed mutations that were rejected or rolled back.

## Consequences

- (+) Every committed mutation is guaranteed to carry its audit entry; there is no window between commit and recording.
- (+) Business modules state what happened in domain language; the journal stays free of transport noise and failed requests.
- (-) One extra `INSERT` per mutation inside the business transaction.
- `PaymentProcessor.RefundPayment` and `SyncPendingPayment` signatures gained an `actorID` parameter; all call sites (HTTP handlers, background sync, tests) pass it explicitly.
- `.golangci.yml` gained a targeted depguard allow for `internal/platform/requestctx` in the application layer: the recorder fills request diagnostics from context, and requestctx carries only request-scoped diagnostics (request ID, trace ID, client IP), not platform infrastructure.
- The two fail-safe exceptions above (logout fail-open, no-transaction services fail-loud) are deliberate; changing them requires amending this ADR.

## See also

- [`docs/adr/0014-in-memory-event-dispatcher.md`](./0014-in-memory-event-dispatcher.md) — deferred the durable outbox; approach A is the in-transaction write that ADR anticipated, applied to audit rather than events.
- [`docs/adr/0019-uuid-v7-app-generated-ids.md`](./0019-uuid-v7-app-generated-ids.md) — audit entry ids are application-generated UUIDv7.
