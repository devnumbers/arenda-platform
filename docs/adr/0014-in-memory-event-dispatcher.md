# ADR 0014: In-Memory Event Dispatcher for Domain Events

## Status

Accepted

## Context

The `identity` module needs to notify `billing` when a new user registers so
that billing can create an onboarding subscription. Previously this onboarding
was called directly inside the identity transaction, coupling the two bounded
contexts and making it hard to evolve either side independently.

We now publish a `UserRegistered` domain event after the identity transaction
commits. The flow is implemented by the following backend files:

- `apps/backend/internal/platform/events/dispatcher.go` — the
  `InProcessDispatcher` adapter.
- `apps/backend/internal/identity/application/events.go` — the
  `UserRegistered` event and the `EventPublisher` port.
- `apps/backend/internal/identity/application/service.go` — publishes
  `UserRegistered` after `tx.Commit()` succeeds.
- `apps/backend/internal/billing/application/service.go` — the
  `OnUserRegistered` handler that creates the onboarding subscription.
- `apps/backend/cmd/api/main.go` — wires the handler into the dispatcher.

## Decision

Use the `InProcessDispatcher` from `internal/platform/events` for domain events.

- The dispatcher is **synchronous**, **in-process**, and requires **no external
  queue or broker**.
- `internal/identity/application.EventPublisher` is the application-level port;
  `internal/platform/events.InProcessDispatcher` is the infrastructure adapter.
- `identity/application/service.go` publishes `UserRegistered` only after
  `tx.Commit()` returns successfully, so the event is not emitted if the user
  registration fails.
- `billing/application/service.go` exposes `OnUserRegistered`, which is
  subscribed in `cmd/api/main.go`.

### Accepted limitations

The current dispatcher is a deliberate simplification with the following
accepted limitations:

- **No durability.** Events live only in process memory; if the process restarts
  or crashes, any in-flight event is lost.
- **Synchronous execution in the request goroutine.** `Publish` blocks until all
  subscribers return, adding latency to the registration request.
- **Process-local only.** Events are not shared across replicas; horizontal
  scaling will cause each instance to have its own subscription state.
- **No retries / at-least-once delivery.** A handler error is only logged; the
  failed event is not redelivered.
- **No ordering guarantees once more event types are added.** Adding other
  event publishers or subscribers may change dispatch order and must be reviewed
  carefully.
- **Tight coupling to the concrete event type.** The dispatcher currently
  dispatches on the `identityapp.UserRegistered` type directly, which limits
  reuse across modules.

## Consequences

- (+) Removes cross-context transaction coupling: billing no longer runs inside
  the identity transaction.
- (+) Simple solution that requires no new infrastructure, message broker, or
  operations overhead.
- (+) Easy to trace in a single-process monolith because dispatch is
  synchronous.
- (-) Acceptable only for the current single-process monolith deployment shape.
- (-) Risk of losing the onboarding event if the process crashes between
  `tx.Commit()` and successful handler execution.
- (-) Billing handler latency and failures directly affect the registration
  HTTP response time and error rate.
- (-) Will not work correctly if the backend is scaled horizontally.

## Future work

When the project outgrows the current single-process shape, replace the
in-memory dispatcher with a durable outbox pattern:

1. Write events to an `outbox` table in the same database transaction as the
   business operation (e.g. user registration).
2. Run a background relay process that polls the outbox and forwards events to
   a message broker or directly to subscribers.
3. Add retries with exponential backoff and a dead-letter queue for
   persistently failing handlers.
4. Promote `EventPublisher` from the `identity` module to a shared application
   port so other bounded contexts can publish domain events consistently.

## See also

- [`docs/adr/0013-in-memory-rate-limiters.md`](./0013-in-memory-rate-limiters.md)
  — another accepted in-process simplification.
- Identity/auth review: [`docs/reviews/2026-07-06-identity-auth-review.md`](../reviews/2026-07-06-identity-auth-review.md).
