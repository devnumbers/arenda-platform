# ADR 0033: Unit-of-Work Transactional Seam

## Status

Accepted

## Context

Every mutating use case across the backend opens a database transaction by hand:

```go
tx, err := s.db.Begin(ctx)
if err != nil { return fmt.Errorf("begin tx: %w", err) }
defer func() { _ = tx.Rollback(ctx) }()

txUsers, err := s.users.WithTx(tx)
if err != nil { return fmt.Errorf("bind user repo: %w", err) }
txCodes, err := s.codes.WithTx(tx)
if err != nil { return fmt.Errorf("bind code repo: %w", err) }
// ... business logic ...
if err := tx.Commit(ctx); err != nil {
    return fmt.Errorf("commit tx: %w", err)
}
```

This `Begin` / `defer Rollback` / `WithTx` × N / `Commit` ceremony is duplicated in **75 call sites** across the application layer (identity, billing, leases, properties, access, notifications). It is fragile in three concrete ways:

1. **Forgotten `WithTx`.** A repository called without `WithTx(tx)` writes outside the transaction. Nothing catches this — the code compiles, the tests pass, and the partial write only surfaces under concurrency.
2. **Audit-out-of-tx drift.** ADR 0020 requires the audit entry in the same transaction as the mutation. The current shape (`s.audit.WithTx(tx).Record(...)` called by hand, at the right point) relies on programmer discipline. It has already produced subtle code: `identity.AuthenticationService.VerifyCode` does an *early* `Commit` on a verification error so it can record the failed-login audit — a transactional shape that is hard to reason about.
3. **Boilerplate cost.** ~15 lines of ceremony per use case, reproduced 75 times, each with its own minor variation in error wrapping and rollback handling.

The existing `internal/transaction` package provides only the thinnest primitives (`Beginner`, `Tx`). There is no seam that bundles "open tx, hand out transactional stores, commit/rollback" into one operation.

## Decision

Introduce a **Unit-of-Work** port as the canonical transactional seam for the backend, in `internal/transaction`:

```go
// UoW runs work inside a single database transaction. The work function receives
// a Tx bound to the transaction; it must not retain the Tx past the call. UoW
// commits on nil error and rolls back otherwise. A panic in work triggers a
// rollback followed by a re-panic, so a panicking use case never leaves a tx open.
type UoW interface {
    Do(ctx context.Context, work func(tx Tx) error) error
}
```

Implementation rules:

- **Panic recovery.** `Do` wraps `work` in `defer func(){ if r := recover(); r != nil { _ = tx.Rollback(ctx); panic(r) } }()`. Safety over purity: a panicking use case rolls back and re-panics; the transaction never leaks to GC.
- **Rollback-after-commit.** After a successful `Commit`, the deferred `Rollback` is a pgx no-op. We trust pgx v5's documented behavior rather than tracking a committed flag.
- **Context cancellation** during `Commit` is propagated as-is (no `context.Canceled` wrapping). Matches current behavior.
- **γ-factory per context.** Each context owns a private helper that builds its transactional stores from a `Tx` in one place. For identity:

  ```go
  // application/stores.go
  type txStores struct {
      users    UserRepository
      codes    LoginCodeRepository
      attempts AttemptRepository
      sessions SessionRepository
      audit    auditapp.Recorder
  }

  // runInTx opens a UoW, builds the identity transactional stores, and runs work.
  func runInTx(ctx context.Context, uow transaction.UoW, work func(*txStores) error) error {
      return uow.Do(ctx, func(tx transaction.Tx) error {
          // ... build txStores from tx once ...
          return work(stores)
      })
  }
  ```

  Use cases describe only the business logic; they cannot forget `WithTx` or record audit out of tx, because `txStores` is the only handle they get.

### Migration: identity first, then by context — completed

The 75 existing call sites were not migrated in one shot. Identity migrated first (this effort, tickets #210–#212); the remaining contexts followed the same pattern, smallest first: access (#346), properties (#347), notifications (#348, unblocked by the FreeReminder removal #380), leases (#349 — the largest block, 18 manual `Begin`s across three services); billing arrived already on UoW with its module rewrite (#245). The remediation grid #325 tracked the chain, and its closing ticket #350 removed the transitional `path-except` from `.golangci.yml`: the forbidigo rule now makes a manual `Begin` a lint failure in production code module-wide. `transaction.Beginner`/`Tx` remain the primitives the platform layer — and UoW itself — is built on; no application use case calls them anymore.

Migration order within identity (bottom-up, each step compiles and tests green):

1. Introduce `transaction.UoW` (port + pgx adapter) and `identity/application.runInTx` + `txStores`.
2. `LogoutService` (no transaction today — becomes the smoke test for the apparatus).
3. `ProfileService` (one repository, simple tx).
4. `AuthenticationService` → split into deep modules: `LoginCodeService` (issue + verify code, attempt window, TTL) and `SessionService.Issue` (find-or-create user + session); `AuthenticationService` becomes a thin orchestrator. The early-Commit-on-error in `VerifyCode` is removed: the failed-login audit is recorded through `txStores.Audit()` inside the normal tx lifecycle, or in a dedicated short tx if it must outlive a rolled-back verification.
5. `PhoneChangeService` — reuses `LoginCodeService` with `purpose = phone_change` instead of duplicating the login-code flow.

### Lint enforcement

`forbidigo` in `.golangci.yml` forbids direct `Begin(ctx)` calls **module-wide** (final state, ticket #350): a manual `Begin` in production code outside `internal/platform` is a lint failure. Two explicit path exclusions carry the whole policy:

- `internal/platform` — the transaction layer itself (`postgres/transaction`, `postgres/uow`, `database/instrumentation`, `scheduler`) opens transactions directly; that is the layer UoW wraps.
- `*_test.go` — test fixtures, legitimized in the next section.

The transitional `path-except` (suppress everywhere except the contexts migrated so far, narrowed by each UoW ticket) was removed with the migration's completion. New contexts inherit the rule automatically — no config edit per context.

### Test fixtures (supplement, ticket #350)

Test fixtures are legitimate controllers of transaction boundaries: the transaction is what they isolate or verify, so routing them through `runInTx` would test the harness instead of the behavior. Two fixture families call `Begin` directly:

- **`fakeUoW` in application unit tests.** Each migrated context's `stores_test.go` implements a `fakeUoW` reproducing `UoW.Do` semantics (Begin → work → Commit on nil, Rollback on error, rollback + re-panic on panic) so commit/rollback/panic behavior is asserted without a database. The fixture must call `Begin` — that code path is the behavior under test.
- **Adapter integration tests.** They open a rolled-back transaction over a real Postgres (`TEST_DATABASE_URL`-gated) as an isolation fixture — every test's data vanishes on rollback — and they assert repository `WithTx` binding inside a caller-controlled transaction (e.g. the access slot-coordinator visibility regression, issue #158: a pool-backed port must see the caller's uncommitted writes through `WithTx`, not through a separate connection).

Production code keeps the absolute rule: a new production `Begin` outside platform fails `make backend-lint` regardless of context.

## Consequences

- (+) A use case can no longer forget `WithTx` or record audit outside the transaction: `txStores` is the only handle, and audit lives next to the repositories it shares the tx with. ADR 0020's in-tx guarantee becomes structural rather than convention-based.
- (+) The early-Commit-on-error in `VerifyCode` — the most fragile piece of identity transaction code — is eliminated.
- (+) One canonical transactional shape, readable by agents and humans: "open tx, get stores, run business, UoW handles the rest."
- (-) Two patterns coexisted until every context migrated — time-boxed by design; the transition closed with ticket #350 (the forbidigo rule went module-wide, the manual path is retired for production code), so the coexistence cost no longer accrues.
- (-) One extra layer of indirection (`runInTx` → `uow.Do` → `tx`) over the manual path. Justified by the correctness gains above.
- `internal/transaction` gains a concrete `UoW` type alongside the existing `Beginner`/`Tx`.

## Known gap (not addressed here)

`platform/httpsupport` imports `identity/domain` for `User`/`Role` — a separate layering issue. It is resolved by ADR 0034 (actor identity shared kernel), not by this ADR.

## See also

- [`docs/adr/0020-audit-log.md`](./0020-audit-log.md) — in-transaction audit; this ADR makes its guarantee structural.
- [`docs/adr/0034-actor-identity-shared-kernel.md`](./0034-actor-identity-shared-kernel.md) — removes the reverse `platform → identity` dependency.
