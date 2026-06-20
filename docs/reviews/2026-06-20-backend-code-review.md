# Backend Code Review Report

**Date:** 2026-06-20  
**Scope:** Full backend review (`apps/backend`) with focus on recent perf-testing and server changes.  
**Review HEAD:** `c83e0273496888fc5759a1d2b9ea00e756db059d`  
**Fixes HEAD:** `db2bfb9`  
**Base:** `1ff94170b97d3a95241908fc7a1508c48a635d10`  
**Method:** Domain-oriented swarm review (5 independent agents: Architecture/DDD, Go idioms, PostgreSQL/persistence, Performance/observability, Security), followed by sequential subagent-driven implementation with spec/code review after each task.  
**Verification run (review):** `go test ./...` ✅, `go vet ./...` ✅, `make backend-lint` ✅.  
**Verification run (after fixes):** `go test ./...` ✅, `go vet ./...` ✅, `go build ./...` ✅, `make backend-lint` ✅.

---

## Executive Summary

The backend generally respects the intended DDD/layered architecture: domain packages stay free of HTTP, SQL, and generated DTOs; application services own transactions and ports; and HTTP handlers keep OpenAPI types at the edge. The recent perf harness is thorough and useful for capacity testing.

However, several **Critical** issues affect data consistency and operational stability:

1. **External HTTP calls are performed inside PostgreSQL transactions** in the billing renewal flow, holding locks and connections for an unbounded time.
2. **Read endpoints mutate durable state** (`ListLeases`, `GetLease`, `ListRecurringOperationsByProperty`, `GetRecurringOperation`), violating command/query separation and creating lock contention.
3. **Database does not enforce key invariants** (one open lease per property, subscription property limits), so concurrent requests can violate business rules.
4. **Property archiving during subscription changes runs in a separate transaction**, creating a split-brain risk between billing and property states.
5. **Soft-deleted operation rows are visible to date-deduplication queries**, which can silently prevent regeneration of operations.
6. **A transaction can be leaked** if a payment-provider initialization error occurs after `Begin`.

| Severity | Count |
|----------|-------|
| Critical | 10 |
| Important | 22 |
| Minor | 23 |

---

## Resolution

All Critical and Important issues have been addressed in a sequential subagent-driven implementation pass. The fixes are recorded in commits from `db3c3ce` through `8e2f258` and summarized in `docs/plans/2026-06-20-backend-fixes-implementation-plan.md`.

- **Critical issues:** 10 / 10 fixed.
- **Important issues:** 22 / 22 fixed or explicitly accepted as residual risk (see notes below).
- **Minor issues:** addressed during the cleanup pass; remaining items are cosmetic.

Deferred / residual items:
- **Phone number encryption at rest** — deferred to a separate ADR (`docs/adr/0009-phone-number-encryption-deferred.md`) because it is a breaking data change.
- **Trusted proxy validation for `X-Forwarded-For`** — documented as deployment requirement; implementation deferred until ingress topology is fixed.
- **SMS audit table plaintext** — accepted risk until phone encryption ADR is resolved.

Final verification after all fixes:
- `go test ./...` ✅ (44 packages, no failures)
- `go vet ./...` ✅
- `make backend-lint` ✅ (0 issues)
- `go build ./...` ✅ (including `cmd/perfvegeta`)

---

## Critical Issues

### C1. External payment call inside a transaction

- **Files:** `internal/billing/application/service.go:844` (and surrounding transaction at :777–:893)
- **Problem:** `s.provider.Charge` is invoked while a PostgreSQL transaction is open. The transaction holds row locks and a connection for an unbounded duration (network latency + provider processing). Under load this can exhaust the connection pool, block the billing worker, and cause deadlocks.
- **Recommendation:** Persist a `pending` payment inside the transaction, commit, call `provider.Charge` **outside** the transaction, then open a new transaction to record the result.
- **Reference:** PostgreSQL [transaction isolation](https://www.postgresql.org/docs/current/transaction-iso.html), [explicit locking](https://www.postgresql.org/docs/current/explicit-locking.html).

### C2. Read endpoints mutate lease status

- **Files:** `internal/leases/application/service.go:201`, `:218`, `:229–247`
- **Problem:** `ListLeases` and `GetLease` call `recalculateStatus`, which calls `s.leases.Update` and persists `status`/`updated_at` on every GET. This violates command/query separation, turns listing into a write operation, and can create lock contention. It also bypasses the readonly middleware, which only inspects HTTP methods.
- **Recommendation:** Compute the effective status in memory or in SQL and return it. Move durable status transitions to an explicit command, reconciliation worker, or scheduler.
- **Reference:** Project `AGENTS.md` architecture rules; PostgreSQL [transaction isolation](https://www.postgresql.org/docs/current/transaction-iso.html).

### C3. Read endpoints extend recurring-operation horizon

- **Files:** `internal/leases/application/recurring_operation_service.go:170`, `:196`, `:642–644`
- **Problem:** `ListRecurringOperationsByProperty` and `GetRecurringOperation` call `extendHorizon`, which begins a transaction and bulk-inserts operations on read. Same consequences as C2. Additionally, the horizon logic returns early when `EndDate` is within 12 months even if existing operations do not reach it.
- **Recommendation:** Do not extend the horizon inside read use cases. Generate up to `min(12 months from now, EndDate)` when the recurring operation is created/updated or via a background worker.
- **Reference:** Project `AGENTS.md` architecture rules.

### C4. No DB constraint for one open lease per property

- **Files:** `internal/leases/application/service.go:87`; `db/migrations/000003_leases.up.sql:19`
- **Problem:** `CreateLease` checks `HasOpenLease` **outside** the transaction. Two concurrent requests can both pass the check and insert open leases for the same property.
- **Recommendation:** Add a partial unique index:
  ```sql
  CREATE UNIQUE INDEX idx_leases_one_open_per_property
  ON leases(property_id)
  WHERE status IN ('awaiting_start','active','requires_action');
  ```
  Handle `unique_violation` (`pgerrcode.UniqueViolation`) in the service.
- **Reference:** PostgreSQL [partial indexes](https://www.postgresql.org/docs/current/indexes-partial.html), [explicit locking](https://www.postgresql.org/docs/current/explicit-locking.html).

### C5. Subscription property-limit check is racy

- **Files:** `internal/properties/application/service.go:91`, `:341`
- **Problem:** `CreateProperty` and `UnarchiveProperty` read `ActivePropertyLimit` outside the transaction, then count and insert inside the transaction. Concurrent creations can exceed the subscription limit.
- **Recommendation:** Lock the subscription row (`SELECT FOR UPDATE`) inside the same transaction before counting active properties.
- **Reference:** PostgreSQL [explicit locking](https://www.postgresql.org/docs/current/explicit-locking.html).

### C6. Property archiving is not atomic with subscription changes

- **Files:** `internal/billing/application/ports.go:73`; `internal/billing/application/service.go:950`; `internal/properties/application/service.go:291–317`
- **Problem:** `PropertyArchiver` is an application port that does **not** accept a transaction. Billing services call it inside their own transaction, but the archiver starts a new transaction. A rollback of the billing transaction after archiving can leave properties archived while the subscription reverts; conversely, a failed archive after a committed subscription can exceed the tariff limit.
- **Recommendation:** Change the port to accept `transaction.Tx`: `ArchiveExcessProperties(ctx, tx, ownerID, limit) error` and implement it with transaction-bound repositories (`propertyRepo.WithTx(tx)`).
- **Reference:** PostgreSQL [tutorial on transactions](https://www.postgresql.org/docs/current/tutorial-transactions.html).

### C7. Transaction leak on provider initialization failure

- **Files:** `internal/billing/application/service.go:277–293`
- **Problem:** In `changeTariffUpgrade`, after committing the pending-payment transaction, if `provider.Init` fails the code starts `markTx` but never defers a rollback. Any error (or panic) before `markTx.Commit` leaves the transaction open, leaking a backend connection.
- **Recommendation:** Add `defer func() { _ = markTx.Rollback(ctx) }()` immediately after `Begin`.
- **Reference:** Go [defer statements](https://go.dev/ref/spec#Defer_statements); PostgreSQL [transactions](https://www.postgresql.org/docs/current/tutorial-transactions.html).

### C8. Soft-deleted operations block date re-use

- **Files:** `db/queries/operations.sql:19`, `:23`; `internal/leases/application/rent_service.go`, `property_billing_lifecycle.go`, `recurring_operation_service.go`
- **Problem:** `ListOperationDatesByLease` and `ListOperationDatesByRecurringOperation` do not filter `deleted_at IS NULL`. Regeneration logic treats those dates as occupied, so a suspended/archived property or updated lease will never regenerate operations on dates that were previously soft-deleted.
- **Recommendation:** Add `AND deleted_at IS NULL` to both queries, regenerate sqlc, and add a regression test.
- **Reference:** PostgreSQL [SELECT / WHERE](https://www.postgresql.org/docs/current/sql-select.html#SQL-WHERE).

### C9. Cancelled subscriptions are never downgraded to basic

- **Files:** `internal/billing/application/service.go:1029`; `ProcessRenewals`
- **Problem:** `expireNonRenewingSubscription` returns early when `sub.Status != Active`. The caller also iterates `ListExpiredCancelled`, so cancelled subscriptions whose `valid_until` has passed keep their paid tariff indefinitely.
- **Recommendation:** Remove the active-status guard for the cancelled batch, or route expired cancelled subscriptions through the same downgrade path.
- **Reference:** Project `CONTEXT.md` subscription lifecycle.

### C10. Fake SMS sender logs raw codes outside local/dev

- **Files:** `cmd/api/main.go:139–143`; `internal/identity/adapters/sms/fake.go:21`
- **Problem:** `SMS_SENDER=fake` is allowed in any environment except `production`, and the fake sender logs the full phone number and raw SMS code. In staging this leaks authentication credentials into logs.
- **Recommendation:** Reject `SMS_SENDER=fake` unless `APP_ENV` is `local` or `dev`. Never log raw codes outside local development.
- **Reference:** Project `AGENTS.md` observability rules.

---

## Important Issues

### Architecture / DDD

1. **Domain imports shared utility packages**  
   `internal/leases/domain/lease.go:9`, `rent_schedule.go:6`, `tenant_contact.go:10`, `internal/identity/domain/phone.go:3` import `internal/shared/timeutil` / `internal/shared/phone`. These are infrastructure-oriented helpers that blur layer boundaries. Inline or duplicate the small logic, or define domain-specific value types.

2. **AGENTS.md out of sync**  
   `apps/backend/AGENTS.md:28` lists bounded contexts as `identity`, `properties`, `billing`, `platform`, but the codebase contains `leases` and `notifications` as separate contexts. Update the documentation.

3. **Upgrade flow split across transactions**  
   `internal/billing/application/service.go:261–355` commits a pending payment, calls the provider, then opens a new transaction. A crash between transactions can leave a pending payment without a provider reference or a saved token that was never persisted. Keep these in a single transaction or add idempotent recovery.

4. **Generated SQLC code location**  
   `internal/generated/postgres` lives outside `internal/platform` or adapter packages. Consider moving it to `internal/platform/generated/postgres`.

### Go idioms / clean code

5. **N+1 query in `ListLeases`**  
   `internal/platform/httpapi/lease_handlers.go:337–362` calls `tenantContactSvc.GetTenantContact` for every lease. Batch-load contacts in the service or add a repository join.

6. **`errors` package shadowed**  
   `cmd/perfvegeta/main.go:1217`, `:1384` declare `var errors []string`, shadowing the imported `errors` package. Rename to `errMessages`.

7. **`cmd/perfvegeta/main.go` is 1,720 lines**  
   It mixes CLI orchestration, Postgres diagnostics, host diagnostics, Vegeta invocation, reporting, and CSV/JSON output. Split into focused packages/files.

8. **`NewBeginner` variadic logger**  
   `internal/platform/database/postgres/transaction.go:18–24` takes `logger ...*slog.Logger`. Change to a single `*slog.Logger`.

9. **`ptrString` readability**  
   `internal/platform/httpapi/property_handlers.go:208` uses `new(string(*v))`. Rewrite as a local variable and `&s`.

10. **`BulkCreate` is not truly bulk**  
    `internal/leases/adapters/postgres/repository.go:587–606` inserts operations one at a time. Use `pgx.CopyFrom` or a multi-row `INSERT`.

11. **`WithTx` dead-code branch**  
    `internal/billing/adapters/postgres/subscription_repository.go:33–93` defends against an impossible transaction-type mismatch. A plain type assertion is consistent with other repositories.

### PostgreSQL / persistence

12. **`payment_day` lacks DB-level check**  
    `db/migrations/000003_leases.up.sql:53` — add `CHECK (payment_day BETWEEN 1 AND 31)`.

13. **Soft-deleted operations lack partial indexes**  
    `db/migrations/000006_operation_soft_delete.up.sql:1` — add partial indexes such as `idx_operations_lease_not_deleted ON operations(lease_id, operation_date) WHERE deleted_at IS NULL`.

14. **Hard-delete queries ignore soft-deletion**  
    `db/queries/operations.sql:44`, `:57`, `:66` (`DeleteFutureOperationsByLease`, etc.) do not include `AND deleted_at IS NULL`. Either add the filter or rename the queries to signal destructive behavior.

15. **`instrumentedRow` connection leak**  
    `internal/platform/database/instrumentation.go:117–137` only releases the acquired connection when `Scan` is called. Document the contract or add a fallback release path.

16. **Lost-update risks in lease/operation updates**  
    `internal/leases/application/service.go:218–247`, `:249–392`; `internal/leases/application/operation_service.go:162–169`, `:212–226` read entities outside the transaction and update inside. Load with `FOR UPDATE` inside the transaction.

17. **`sqlc` does not use prepared queries**  
    `sqlc.yaml:13` has `emit_prepared_queries: false`. Under load this adds parse/plan overhead. Enable `true` and ensure pool sizing supports it.

18. **No PostgreSQL statement/idle-transaction timeouts**  
    `internal/platform/database/database.go:38–63` does not set `statement_timeout` or `idle_in_transaction_session_timeout`. Add conservative defaults in `ConnConfig.RuntimeParams`.

19. **Pool tuning is hard-coded**  
    Same file — expose `DB_MAX_CONNS`, `DB_MIN_CONNS`, etc. via environment variables.

20. **`idx_leases_open_past_end` is not partial**  
    `db/migrations/000010_leases_past_end_index.up.sql:1` includes completed/archived leases. Make it partial: `WHERE status IN ('awaiting_start','active','requires_action') AND end_date IS NOT NULL`.

### Performance / observability

21. **`logSuccessfulRequests` hardcoded to `false`**  
    `internal/platform/httpapi/server.go:42–43` means most successful HTTP requests are not logged, violating the observability rule. Make it env-configurable with default `true`.

22. **Subscription fetched on every mutating request**  
    `internal/platform/httpapi/readonly_middleware.go:69–78`/`canMutateData` executes up to three queries per write. Cache the mutation-rights decision per request (e.g., attach it to context after `GetMe`).

23. **Rate limiter serializes all requests**  
    `internal/platform/httpapi/ratelimit.go:42–53` locks a global mutex for every check. Shard buckets by key hash or use `sync.Map`.

24. **Rate limiter is in-memory only**  
    In a multi-instance deployment per-IP/phone limits can be bypassed. Document the single-instance assumption or add a shared backend.

25. **Workers not awaited on shutdown**  
    `cmd/api/main.go:226–230` starts background workers but only gracefully shuts down the HTTP server. Use `sync.WaitGroup` or errgroup.

26. **Cleaner does unbounded deletes**  
    `internal/platform/cleaner/cleaner.go:48–61` issues `DELETE` without `LIMIT` or batching. Batch with a loop.

27. **Session lookup on every request**  
    `internal/platform/httpapi/session.go:95–127` loads the session from the database even for public paths and never extends expiry. Consider a small cache or path-aware skip list.

28. **Billing worker holds advisory lock for entire tick**  
    `internal/platform/scheduler/billing_worker.go:70–89` keeps the lock connection during all sub-processes. Acquire, poll, release, then do per-subscription work.

### Security

29. **HSTS header missing**  
    `internal/platform/httpapi/server.go:46–54` sets CSP/XFO/XCTO but not `Strict-Transport-Security`. Add it when `CookieSecure` is true.

30. **Phone numbers stored in plaintext**  
    `db/migrations/000001_init_schema.up.sql:5`; `internal/identity/adapters/postgres/repository.go:48`. Encrypt at rest or formally accept the risk.

31. **SMS audit table stores full message**  
    `internal/notifications/adapters/postgres/repository.go:440–449` stores phone + body in plaintext. Minimize retention or encrypt.

32. **No-op encryptor with empty key**  
    `internal/platform/encryption/aes.go:22–25`; `cmd/api/main.go:91–93`. Refuse to use the no-op encryptor when a real payment provider is configured, even in `local`.

33. **`APP_ENV` defaults to `local`**  
    `internal/platform/config/config.go:56–58`. A missing env var causes production-like deployment to run with `CookieSecure=false` and no encryption key. Default to the most restrictive mode or fail fast.

34. **`X-Request-ID` not validated**  
    `internal/platform/httpapi/request_id.go:33–41` echoes arbitrary client input. Validate format/length and fall back to a generated ID.

35. **`RealIP` trusts `X-Forwarded-For` blindly**  
    `internal/platform/httpapi/server.go:61`. Clients can spoof IP addresses unless a trusted proxy sanitizes the header. Validate against trusted proxies or document the requirement.

36. **4xx errors expose raw `err.Error()`**  
    Several handlers put `err.Error()` into RFC 7807 `detail`. Map domain errors to fixed user-facing messages.

37. **`internalError` logs raw upstream errors**  
    `internal/platform/httpapi/problem.go:24–28` may leak PII/secrets from database/provider errors. Wrap/redact before logging.

---

## Minor Issues

1. `internal/platform/httpapi/session.go:120` — stale session cookie is not cleared when expired/not found.
2. `internal/platform/httpapi/session.go:27–28` — context keys are untyped integers; use distinct unexported sentinel types.
3. `internal/platform/httpapi/session.go:52` — `MaxAge` truncates toward zero; use `Expires` or ensure `MaxAge >= 1`.
4. `internal/platform/httpapi/session.go:85–88` — session token hashes use plain SHA-256; consider keyed HMAC for consistency.
5. `internal/platform/httpapi/session.go:90–92` — `fallbackClock` uses `time.Now()` directly.
6. `cmd/api/main.go:82` — local `logger` shadows imported `platform/logger` package.
7. `cmd/perfseed/seed.go:116–137` — `TRUNCATE` without `RESTART IDENTITY` leaves sequences non-reproducible.
8. `cmd/perfseed/seed.go:110` — `writeFixtures` uses relative path `perf/fixtures.json`; accept a flag.
9. `cmd/perfseed/seed.go:139–182` — entire seed in one transaction; may cause long WAL flush for large datasets.
10. `internal/platform/cleaner/cleaner.go:49` — uses `time.Now()` instead of injected `clock.Clock`.
11. `internal/platform/cleaner/cleaner.go:51–58` — logs use `Error` instead of `ErrorContext`.
12. `internal/platform/httpapi/auth_handlers.go:57–66` — client validation failures logged at `Error` level; should be `Warn`/`Debug`.
13. `internal/platform/httpapi/problem.go:40` — `json.NewEncoder` error silently discarded.
14. `internal/platform/httpapi/logging.go:110–113` — unmatched routes log raw `r.URL.Path` (high cardinality); use fixed label.
15. `internal/platform/httpapi/server.go:60–61` — comment explains deprecated `middleware.RealIP`; plan migration.
16. `internal/billing/application/service.go:76` — `ListTariffs` sorts in memory; push to SQL.
17. `internal/billing/adapters/postgres/payment_method_repository.go:90–91` — magic string `"23505"` instead of `pgerrcode.UniqueViolation`.
18. `internal/billing/adapters/postgres/subscription_repository.go:292–311` — manual UUID conversion instead of `pgconv.UUIDFromPgtype`.
19. `internal/billing/adapters/postgres/tariff_repository.go:18–25` — tariff cache has no TTL; document immutability or add expiry.
20. `internal/identity/application/service.go:73` — explicit `_ = tx.Rollback(ctx)` before early return is redundant due to deferred rollback.
21. `internal/leases/adapters/postgres/repository.go:512–518` — `UpdateStatusByPropertyID` uses hand-written SQL; move to sqlc queries.
22. `internal/leases/application/ports.go:42–43` — `GetByLease` and `GetByLeaseID` appear redundant.
23. `internal/notifications/adapters/postgres/repository.go:462–467` — `isDuplicateSMSReminderError` matches any unique violation, not the specific constraint.

---

## Confirmed Good Practices

- Domain packages do not import HTTP, OpenAPI generated types, `pgx`, `sqlc`, `database/sql`, config, or adapters. Layer direction is respected.
- `transaction.Tx` / `transaction.Beginner` is a clean, persistence-agnostic port.
- HTTP handlers map generated OpenAPI DTOs explicitly to application/domain models.
- Money stored as integer kopecks for operations/leases/tariffs/payments; system timestamps use `timestamptz`; domain dates use `date`.
- `X-Request-ID` is generated when missing, echoed in responses, and included in problem details and logs.
- Session tokens are 256-bit random values, hashed before DB lookup; SMS codes are hashed with a phone-bound salt.
- Payment provider tokens are encrypted with AES-GCM and authenticated with HMAC-SHA256 for duplicate detection.
- Worker batch queries use `FOR UPDATE SKIP LOCKED` correctly.
- `sent_sms_reminders` is written before the provider call, giving an at-most-once SMS delivery guarantee.
- Webhooks always return `200 OK` to avoid leaking payload validity.
- The perf harness captures `pg_stat_statements`, pgx pool snapshots, and host resource limits — a solid practice for capacity testing.

---

## Suggested Fix Order

1. **Stop holding transactions during external calls** (C1, C7).
2. **Remove write-on-read side effects** (C2, C3).
3. **Add DB constraints and fix race conditions** (C4, C5, C6, C8, C9).
4. **Fix security issues** (C10, I29–I37).
5. **Address observability and performance regressions** (I21–I28).
6. **Refactor large files and tighten Go idioms** (I7, I8, I9, I11, minors).
7. **Update documentation** (`AGENTS.md` contexts, inline comments).

---

## Appendix: File-list mismatch

The user-provided list of recent changes referenced `apps/backend/cmd/perfmaxrps/main.go` and `apps/backend/perf/scripts/endpoint_benchmark.js/endpoints.js`, but the repository currently contains `apps/backend/cmd/perfvegeta/main.go` and no `perf/scripts` directory. Update release notes or changeset descriptions to match the committed files.
