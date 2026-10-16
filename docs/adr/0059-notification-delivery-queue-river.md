# ADR 0059: Notification Delivery Queue — River

The delivery pipeline of the notifications map (#734, ticket #740) runs
channel deliveries (email, Web Push) through [River](https://riverqueue.com)
— a Go job queue backed by our own PostgreSQL — instead of synchronous
in-request sends. The publisher writes the feed rows and enqueues the
delivery jobs in one transaction; River works the jobs with retries, backoff
and a global email-provider budget.

## Status

Accepted. Implements the delivery axis of the map #734 charter decision
(«очередь = River»), based on research #735. Complements
[ADR 0058](./0058-notification-feed-and-per-category-settings.md) (the feed
model) and [ADR 0013](./0013-in-memory-rate-limiters.md) (rate-limit canon,
unchanged for per-user budgets).

## Context

Every email today is sent synchronously in-request or in a goroutine worker
(ADR 0014 dispatcher, grace channel of issue #253): single attempt,
best-effort. Three pressures break that shape as the notification catalog
lands (#748–#752):

1. **A burst is a loss.** A fan-out to many recipients shares the request's
   fate; a provider hiccup silently drops the leg.
2. **No retry budget.** Transient SMTP/push failures are logged and gone.
3. **No provider ceiling.** Nothing smooths a burst against the email
   provider's rate limits.

The research (#735) compared River against Redis-backed queues (asynq,
gocraft/work) and a hand-rolled SKIP LOCKED poller: River wins on
transactional enqueue into our own pgxpool (no second store), unique jobs,
snooze-as-throttle, retention/maintenance and OTel observability out of the
box. License is **MPL-2.0**: file-level copyleft on River's own files only —
as an unmodified dependency in a SaaS backend it imposes no obligations on
our code.

## Decision

1. **In-process River client.** The API process runs the client next to the
   five scheduler workers (`cmd/api/wire/workers.go`); one deploy instance
   (`docs/deployment.md`), so no separate worker process. `Start` runs in the
   workers phase and returns immediately; the phase's `Wait` also blocks on
   `client.Stopped()` — cancelling the lifecycle context begins the soft stop
   (`NOTIFICATIONS_RIVER_SOFT_STOP_TIMEOUT`, 10s), and in-flight jobs finish
   before the process exits. No `NOTIFICATIONS_QUEUE_ENABLED` switch: with no
   publishers in #740 the queue is inert (nothing enqueues), so a flag would
   guard nothing — the grace migration (#741) is the rollout gate.

2. **Two queues, independent ceilings.** `notifications_email` (SMTP,
   `NOTIFICATIONS_EMAIL_MAX_WORKERS`=4) and `notifications_push` (fan-out,
   `NOTIFICATIONS_PUSH_MAX_WORKERS`=16) cannot starve each other.

3. **Transactional enqueue; post-commit publication seam.** Following the
   grace-events canon (решение #740): the publishing context commits its own
   business transaction first, then calls `Publisher.Publish`, which — in
   one transaction — inserts the feed rows and enqueues the channel jobs via
   River's `InsertTx`. A job never exists without its row and vice versa.
   The queue adapter unwraps the caller's `transaction.Tx` to the pgx
   transaction (`database.PgxTxOf`); the instrumented tx keeps query
   diagnostics.

4. **Jobs carry the notification id, not the content.** The worker reloads
   the committed feed row — the row stays the single source of the text —
   and resolves the recipient's contact and live push subscriptions at
   delivery time. The per-category × channel settings matrix (ADR 0058)
   is enforced in the same place when it exists (#743); until then nothing
   can silence a channel (the old table is dropped, everything-on default).

5. **Unique jobs = one delivery in flight.** Job args are the notification
   id; `UniqueOpts{ByArgs, ByState minus completed/cancelled/discarded}`
   makes the enqueue idempotent across crash windows without blocking a
   legitimate future publication. Feed-row dedup ((user_id, dedup_key)
   unique, ADR 0058) remains the primary guard — a deduped row enqueues
   nothing.

6. **Retries and throttling.** River's default backoff ladder
   (`attempts^4 ± 10%`) with a budget of 8 attempts per channel
   (`NOTIFICATIONS_{EMAIL,PUSH}_MAX_ATTEMPTS`); exhaustion lands in
   `discarded` (queryable DLQ, 7-day retention). The global provider budget
   (`NOTIFICATIONS_EMAIL_PROVIDER_PER_MINUTE`=60, token bucket) is spent
   inside the email job before the send; an empty bucket snoozes the job —
   `river.JobSnooze` does not consume an attempt. A push-service 429 snoozes
   the fan-out the same way (re-sent payloads collapse per device via the
   notification-id tag). Per-user budgets stay in the HTTP rate-limit layer
   (ADR 0013) — nothing moves into the queue.

7. **Migrations via rivermigrate, own chain.** River's schema is applied by
   the `rivermigrate` Go API right after `database.MigrateUp` (the `migrate`
   deploy step and `AUTO_MIGRATE`). The SQL is never vendored into
   `db/migrations` — it changes per River upgrade and is upstream-tested;
   our squawk/migrations-lint gates keep applying to our chain only.

8. **Observability.** The `otelriver` plugin feeds River's spans and metrics
   to the global OTel providers (Uptrace); the adapters keep the
   `notifications.{email,push}.dispatched{outcome}` counters (email:
   `sent`/`skipped`/`rate_limited`/`failed`/`cancelled`) so delivery health
   is alertable without SQL.

9. **At-least-once, honestly.** A crash between the provider send and the
   job's completion can duplicate a message (River's documented execution
   semantics). A duplicate beats a loss; exactly-once against SMTP/push is
   unattainable in principle. If a future payment-critical notification
   needs stronger guarantees, the seam is a send-ledger with
   `UNIQUE(notification_id)` checked before sending — deliberately not built
   speculatively.

## Consequences

- (+) Bursts degrade to latency (queue + worker ceiling), not to losses;
  transient channel failures retry for hours, not die once.
- (+) Publishers get one call (`Publisher.Publish`) with the fan-out, dedup,
  actor-skip and channel scheduling inside; grace (#741) and the catalog
  publishers (#748–#752) do not touch the queue directly.
- (+) The email provider ceiling is enforced where the send happens and is
  observable; throttling never burns the retry budget (snooze).
- (−) A new dependency pinned at v0.x (River pre-1.0 changes API between
  releases; upgrades are deliberate, CHANGELOG-checked events).
- (−) `river_job` and companions live in the same database; retention
  defaults (completed/cancelled 24h, discarded 7d) keep them small.
- (−) Graceful shutdown can now take up to the soft-stop window (10s) after
  the HTTP server drains — inside the documented 5–15s deploy gap, worth
  watching.
- = Delivery semantics change from best-effort single-attempt to
  at-least-once; recorded here and in the context `GLOSSARY.md` so the
  documented guarantees do not drift.

## See also

[ADR 0058](./0058-notification-feed-and-per-category-settings.md),
[ADR 0013](./0013-in-memory-rate-limiters.md),
[ADR 0033](./0033-unit-of-work-transactional-seam.md),
research #735 (`docs/research/2026-09-17-notifications-river.md` on branch
`research/notifications-river`).
